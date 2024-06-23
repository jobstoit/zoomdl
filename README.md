# ZoomDL

ZoomDL is a program that will get all your zoom cloud recordings and downloads them putting them in directories based on the title.
If specified it will delete the downloaded recordings after.

## Usage

On the [zoom marketplace create a Server-to-Server-OAuth app](https://marketplace.zoom.us/develop/create) to get the user id, client id and client secret.
Make sure to read the [Server-to-Server OAuth app docs](https://marketplace.zoom.us/docs/guides/build/server-to-server-oauth-app/)

The configuration is done in environment and captures the following:

```go
func NewConfig() *Config {
 c := &Config{}

 c.RecordingTypes = strings.Split(os.Getenv("ZOOMDL_RECORDING_TYPES"), ";")
 c.IgnoreTitles = strings.Split(os.Getenv("ZOOMDL_IGNORE_TITLES"), ";")

 c.Destinations = strings.Split(os.Getenv("ZOOMDL_DESTINATIONS"), ";")
 if dir := os.Getenv("ZOOMDL_DIR"); dir != "" { // backwards compatibility
  c.Destinations = append(c.Destinations, fmt.Sprintf("file://%s", dir))
 }

 c.UserID = envRequired("ZOOMDL_USER_ID")
 c.ClientID = envRequired("ZOOMDL_CLIENT_ID")
 c.ClientSecret = envRequired("ZOOMDL_CLIENT_SECRET")

 c.APIEndpoint = envURL("ZOOMDL_API_ENDPOINT", "https://api.zoom.us/v2")
 c.AuthEndpoint = envURL("ZOOMDL_AUTH_ENDPOINT", "https://zoom.us")
 c.StartingFromYear = envInt("ZOOMDL_START_YEAR", 2018)
 c.Concurrency = envInt("ZOOMDL_CONCURRENCY", 4)
 c.ChunckSizeMB = envInt("ZOOMDL_CHUNKSIZE_MB", 256)

 c.Duration = envDuration("ZOOMDL_DURATION", "30m")
 c.DeleteAfter = os.Getenv("ZOOMDL_DELETE_AFTER") == "true"

 return c
}
```

Use with docker:

```sh
$ docker run \
 -e "ZOOMDL_USER_ID=<your-user-id>" \
 -e "ZOOMDL_CLIENT_ID=<your-client-id>" \
 -e "ZOOMDL_CLIENT_SECRET=<your-client-secret>" \
 ghcr.io/jobstoit/zoomdl:latest
```
