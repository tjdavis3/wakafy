package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	flags "github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v2"

	"github.com/aquilax/go-wakatime"
	"github.com/eiannone/keyboard"
	clockify "github.com/lucassabreu/clockify-cli/api"
	"github.com/lucassabreu/clockify-cli/api/dto"

	// Autoloads environment from .env
	_ "github.com/joho/godotenv/autoload"
)

const progDesc = `
Pulls all time entries from Wakatime and adds them to Clockify.
`

var Config = struct {
	WakatimeKey string `required:"true" long:"wakatime" env:"WAKATIME_KEY" description:"The API key for accessing Wakatime"`
	ClockifyKey string `required:"true" long:"clockify" env:"CLOCKIFY_KEY" description:"The API key for accessing Clockify"`
	Days        int    `default:"7" short:"d" long:"days" description:"The number of days back to retrieve from Wakatime"`
	// AddProjects is hidden for now since it is not implemented
	AddProjects  bool    `hidden:"true" short:"a" long:"add-projects" description:"If the Wakatime project does not exist in Clockify, add it, otherwise add the time without a project"`
	WriteManPage bool    `long:"manpage" description:"Generates the manpage" hidden:"true"`
	ProjectsFile *string `long:"projects" short:"p" description:"Location of the yaml file to map wakatime projects to Clockify projects"`
	Interactive  bool    `long:"interactive" short:"i" description:"Run in interactive mode" `
}{}

type App struct {
	ClockClient     *clockify.Client
	WakaClient      *wakatime.WakaTime
	ClockProjects   []dto.Project
	ProjectMappings map[string]string
	Workspace       string
}

func main() {
	parser := flags.NewParser(&Config, flags.Default)
	parser.Usage = "{workspace} [OPTIONS]"
	parser.ShortDescription = "Adds Wakatime entries to Clockify"
	parser.LongDescription = progDesc
	args, err := parser.Parse()
	if err != nil {
		if flgErr, ok := err.(*flags.Error); ok {
			if flgErr.Type == flags.ErrHelp {
				os.Exit(0)
			}
		}
		fmt.Println(err.Error())
		os.Exit(2)
	}
	if Config.WriteManPage {
		parser.WriteManPage(os.Stdout)
		os.Exit(0)
	}
	if len(args) != 1 {
		os.Stderr.WriteString("\nError: workspace not specified\n\n")
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	app := &App{}

	// load the project mapping file
	if Config.ProjectsFile != nil {
		pfile, err := os.Open(*Config.ProjectsFile)
		if err != nil {
			fmt.Println("Error opening ", *Config.ProjectsFile, err)
			os.Exit(2)
		}
		defer pfile.Close()
		buf, err := ioutil.ReadAll(pfile)
		if err != nil {
			fmt.Println("Error reading file: ", err)
			os.Exit(2)
		}
		err = yaml.Unmarshal(buf, &app.ProjectMappings)
		if err != nil {
			fmt.Println("Invalid YAML file: ", err)
			os.Exit(2)
		}
	}

	transport := wakatime.NewBasicTransport(Config.WakatimeKey)
	app.ClockClient, err = clockify.NewClient(Config.ClockifyKey)
	if err != nil {
		panic(err)
	}
	app.Workspace, err = app.GetWorkspace(args[0])
	if err != nil {
		log.Fatal(err)
	}

	projParam := clockify.GetProjectsParam{
		Workspace: app.Workspace,
	}
	app.ClockProjects, err = app.ClockClient.GetProjects(projParam)
	// if err != nil {
	// 	panic(err)
	// }

	app.WakaClient = wakatime.New(transport)
	startDate := time.Now().AddDate(0, 0, Config.Days*-1)
	for startDate.Before(time.Now()) {
		fmt.Println(startDate)
		summaries, err := app.WakaClient.Durations("current", startDate, nil, nil)
		if err != nil {
			fmt.Println(err)
			return
		}
		app.AddTime(summaries)
		startDate = startDate.AddDate(0, 0, 1)
	}
}

// GetWorkspace returns the workspace ID for the given workspace name.
// Returns an error if it is not found or there is an error communicating with clockify.
func (app *App) GetWorkspace(workspace string) (workspaceID string, err error) {
	params := clockify.GetWorkspaces{Name: workspace}
	workspaces, err := app.ClockClient.GetWorkspaces(params)
	if err != nil {
		return "", fmt.Errorf("Could not load workspaces: %w", err)
	}
	var ID string
	for _, ws := range workspaces {
		if ws.Name == workspace {
			ID = ws.ID
			break
		}
	}
	if ID == "" {
		return "", fmt.Errorf("Could not find workspace")
	}
	return ID, nil
}

// FindProjectID searches for the ID that corresponds to projectName
// and returns it or nil if not found
func (app *App) FindProjectID(projectName string) *string {
	for _, proj := range app.ClockProjects {
		if projectName == proj.Name {
			return &proj.ID
		}
	}
	return nil
}

// AddTime takes a wakatime durations and adds them to Clockify
func (app *App) AddTime(summaries *wakatime.Durations) {
	// var kbd *bufio.Reader
	// if Config.Interactive {
	// 	kbd = bufio.NewReader(os.Stdin)
	// }
	for _, entry := range summaries.Data {
		// for _, project := range entry.Projects {
		addProject := false
		project, ok := app.ProjectMappings[entry.Project]
		if !ok {
			project = entry.Project
		}
		projectID := app.FindProjectID(project)
		if projectID == nil {
			addProject = true
		}

		if addProject {
			// fmt.Println("Adding Project", entry.Project)
			app.ClockProjects = append(app.ClockProjects, dto.Project{Name: entry.Project})
		}
		// start, _ := time.Parse(time.RFC3339, entry.Range.Start)
		// end, _ := time.Parse(time.RFC3339, entry.Range.End)

		var yn rune

		if Config.Interactive {
			for {
				var err error
				fmt.Printf("Add to project %s? (y/n/q) ", entry.Project)
				yn, _, err = keyboard.GetSingleKey()
				if err != nil {
					panic(err)
				}
				// yn, err := kbd.ReadByte()

				if yn == 'q' {
					fmt.Println()
					os.Exit(0)
				}
				if yn != 'y' && yn != 'n' {
					fmt.Println("Invalid entry")
					continue
				} else {
					fmt.Println()
					break
				}
			}
		} else {
			yn = 'y'
		}
		if yn != 'y' {
			continue
		}
		endUnix := entry.Time.Time().Unix() + int64(entry.Duration)
		end := time.Unix(endUnix, 0)
		fmt.Println(entry.Project, " from: ", entry.Time.Time(), " to: ", end)
		entry := clockify.CreateTimeEntryParam{
			Workspace:   app.Workspace,
			Start:       entry.Time.Time(),
			End:         &end,
			Billable:    true,
			Description: fmt.Sprintf("Coding on %s", entry.Project),
		}
		if projectID != nil {
			entry.ProjectID = *projectID
		}
		_, err := app.ClockClient.CreateTimeEntry(entry)
		if err != nil {
			fmt.Println("Failed to create time entry: ", err.Error())
		}
		// }
		fmt.Println("")
	}
}
