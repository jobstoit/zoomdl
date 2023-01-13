package main

import (
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	ZoomAPIEndpoint     = "https://api.zoom.us/v2"
	SavedRecordFileName = ".zoomdl_saved_records.json"
)

// Config defines the application configuration
type Config struct {
	RecordingTypes   []string
	IgnoreTitles     []string
	DeleteAfter      bool
	Duration         time.Duration
	Directory        string
	Token            string
	APIEndpoint      string
	UserID           string
	ClientID         string
	ClientSecret     string
	AccountID        string
	StartingFromYear int
}

// SavedRecord is a dataentry stored in the saved records file that
// keeps track of all the records
type SavedRecord struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	SavedAt    time.Time `json:"saved_at"`
	RecordedAt time.Time `json:"recorded_at"`
	Path       string    `json:"path"`
}

func main() {
	config := NewConfig()

	zc := NewZoomClient(config)

	savedPath := path.Join(zc.config.Directory, SavedRecordFileName)
	if _, err := os.Stat(savedPath); os.IsNotExist(err) {
		file, err := os.Create(savedPath)
		if err != nil {
			log.Fatal(`error creating save file`)
		}
		_ = file.Close()
	}

	log.Printf("starting service with allowed recording types %v and ignored titles %v", config.RecordingTypes, config.IgnoreTitles)
	for {
		if err := zc.Sweep(); err != nil {
			log.Printf("error during sweep: %s", err)
		}

		time.Sleep(config.Duration)
	}
}

// NewConfig returns a new initialized config
func NewConfig() *Config {
	c := &Config{}
	envDur := os.Getenv(`ZOOMDL_DURATION`)
	if envDur == `` {
		envDur = `30m`
	}

	dur, err := time.ParseDuration(envDur)
	if err != nil {
		log.Fatalf("error parsing duration format %s: %v", envDur, err)
	}
	c.Duration = dur

	c.RecordingTypes = strings.Split(os.Getenv(`ZOOMDL_RECORDING_TYPES`), ";")
	c.IgnoreTitles = strings.Split(os.Getenv(`ZOOMDL_IGNORE_TITLES`), ";")

	c.UserID = os.Getenv(`ZOOMDL_USER_ID`)
	if c.UserID == `` {
		log.Fatal(`missing required user id`)
	}

	c.ClientID = os.Getenv(`ZOOMDL_CLIENT_ID`)
	if c.ClientID == `` {
		log.Fatal(`missing required client id`)
	}

	c.ClientSecret = os.Getenv(`ZOOMDL_CLIENT_SECRET`)
	if c.ClientSecret == `` {
		log.Fatal(`missing required client secret`)
	}

	c.DeleteAfter = os.Getenv(`ZOOMDL_DELETE_AFTER`) == `true`

	c.Directory = os.Getenv(`ZOOMDL_DIR`)
	if c.Directory == `` {
		c.Directory = `/`
	}

	c.APIEndpoint = os.Getenv(`ZOOMDL_API_ENDPOINT`)
	if c.APIEndpoint == `` {
		c.APIEndpoint = `https://api.zoom.us`
	}

	c.AccountID = os.Getenv(`ZOOMDL_ACCOUNT_ID`)
	if c.AccountID == `` {
		log.Fatal(`missing required account id`)
	}

	year := 2018
	syear := os.Getenv(`ZOOMDL_START_YEAR`)
	if syear != `` {
		y, err := strconv.Atoi(syear)
		if err != nil {
			log.Print(`error parsing year`)
		} else {
			year = y
		}
	}
	c.StartingFromYear = year

	return c

}
