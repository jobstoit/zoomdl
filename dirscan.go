package main

import (
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var (
	// Regex for
	// [0] Some - Title dir/GMT20220707-161102-speaker_view.mp4
	// [1] Some - Title dir
	// [2] GMT20220707-161102
	// [3] GMT
	// [4] speaker_view
	formatReg = regexp.MustCompile(`([\w\s\-]+)/((GMT)[0-9]{8}[-][0-9]{6})[-]([\s\w]+)\.mp4$`)
)

// Recording contains the details about a recording on the system
type Recording struct {
	ID    string
	Class string
	Path  string
	Date  time.Time
}

func NewRecording(path string) *Recording {
	r := &Recording{}
	matches := formatReg.FindStringSubmatch(path)

	r.Class = matches[1]
	t, err := time.Parse("MST20060102-150405", matches[2])
	if err != nil {
		panic(err)
	}

	r.Date = t

	return r
}

// ScanDir scans the directory for the recordings
func ScanDir(root string) []string {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		//TODO

		return nil
	})

	if err != nil {
		panic(err)
	}

	return []string{}
}
