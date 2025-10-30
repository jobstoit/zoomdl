package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	SavedRecordFileName = ".zoomdl_saved_records.json"
)

// Config defines the application configuration
type Config struct {
	RecordingTypes   []string
	IgnoreTitles     []string
	Destinations     []string
	DeleteAfter      bool
	Duration         time.Duration
	Token            string
	APIEndpoint      *url.URL
	AuthEndpoint     *url.URL
	UserID           string
	ClientID         string
	ClientSecret     string
	Concurrency      int
	ChunckSizeMB     int
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
	fs, err := newMultiFS(context.Background(), config)
	if err != nil {
		log.Fatalf("error opening destinations: %v", err)
	}

	zc := NewZoomClient(config, fs)

	log.Printf("starting service with allowed recording types %v and ignored titles %v", config.RecordingTypes, config.IgnoreTitles)
	for {
		wait := config.Duration
		if err := zc.Sweep(); err != nil {
			log.Printf("error during sweep: %s", err)
			wait = time.Second * 5
		}

		time.Sleep(wait)
	}
}

// NewConfig returns a new initialized config
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

func envRequired(env string) string {
	val := os.Getenv(env)
	if val == "" {
		log.Fatalf("missing required environment variable '%s'", env)
	}

	return val
}

func envDefault(env, defaultStr string) string {
	val := os.Getenv(env)
	if val == "" {
		return defaultStr
	}

	return val
}

func envDuration(env, defaultDuration string) time.Duration {
	val := envDefault(env, defaultDuration)

	dur, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("error parsing %s with value '%s': %v", env, val, err)
	}

	return dur
}

func envURL(env, defaultURL string) *url.URL {
	val := envDefault(env, defaultURL)

	u, err := url.Parse(val)
	if err != nil {
		log.Fatalf("error parsing for '%s' url '%s': %v", val, env, err)
	}

	return u
}

func envInt(env string, defaultInt int) int {
	val := os.Getenv(env)
	if val == "" {
		return defaultInt
	}

	i, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("error parsing for '%s' int '%s' (now using default): %v", val, env, err)
		return defaultInt
	}

	return i
}
