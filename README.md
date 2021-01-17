# Wakafy

Loads time entries from [wakatime](https://wakatime.com) and loads the into [Clockify](https://clockify.me).


Usage:
:  wakafy {workspace} [OPTIONS]

Pulls all time entries from Wakatime and adds them to Clockify.


## Application Options:

--wakatime
: The API key for accessing Wakatime [$WAKATIME_KEY]

--clockify
: The API key for accessing Clockify [$CLOCKIFY_KEY]

-d, --days
: The number of days back to retrieve from Wakatime (default: 7)

-p, --projects
: Location of the yaml file to map wakatime projects to Clockify projects

## Help Options:
-h, --help
: Show this help message
