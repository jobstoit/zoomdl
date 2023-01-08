package main

import (
	"flag"
	"fmt"
	"strings"
	"time"
)

// Config defines the application configuration
type Config struct {
	RecordingTypes []string
	IgnoreTitles   []string
	DeleteAfter    bool
	DownloadBuffer int
	Duration       time.Duration
}

func main() {
	fmt.Println("vim-go")
}

func NewConfig() *Config {
	c := &Config{}
	var rts, igns string
	flag.StringVar(&rts, "ZOOMDL_RECORDING_TYPES", "speaker_view,gallery_view,active_speaker", "The recording types that should be downloaded")
	flag.StringVar(&igns, "ZOOMDL_IGNORE_TITLES", "", "The titles to ignore while downloading")
	flag.BoolVar(&c.DeleteAfter, "ZOOMDL_DELETE_AFTER", false, "specifies whether to delete the downloaded content")
	flag.DurationVar(&c.Duration, "ZOOMDL_DURATION", time.Minute*30, "Specifies how long it takes to do a next cycle")

	flag.Parse()

	c.RecordingTypes = strings.Split(rts, ";")
	c.IgnoreTitles = strings.Split(igns, ";")

	return c
}
