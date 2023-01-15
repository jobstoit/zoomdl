package main

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"testing"
	"text/tabwriter"
	"time"
)

func TestListAllRecordings(t *testing.T) {
	c := SetupTest("tmp_list_all_recordings")
	defer os.RemoveAll(c.config.Directory) //nolint: errcheck

	c.config.StartingFromYear = 2017

	meetings, err := c.ListAllRecordings()
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
	}

	if len(meetings) < 1 {
		t.Errorf("missing expected recordings")
	}

	for _, meeting := range meetings {
		switch meeting.ID {
		case 1001:
			eq(t, "static", meeting.Topic)
			eq(t, 4, len(meeting.RecordingFiles))
			if len(meeting.RecordingFiles) != 4 {
				break
			}

			rf := meeting.RecordingFiles[0]
			eq(t, RecordingTypeAudioOnly, rf.RecordingType)
		}
	}

	eq(t, 15, len(meetings))

	if t.Failed() {
		wr := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)
		fmt.Fprintln(wr, "topic\tdate\trecording count")
		for _, meeting := range meetings {
			fmt.Fprintf(wr, "%s\t%v\t%d\n", meeting.Topic, meeting.StartTime, len(meeting.RecordingFiles))
		}

		wr.Flush()
	}
}

func TestDownload(t *testing.T) {
	c := SetupTest("tmp_test_download")
	defer os.RemoveAll(c.config.Directory) //nolint: errcheck

	fpath, err := c.DownloadVideo("static", RecordingFile{
		FileType:       FileTypeMP4,
		RecordingType:  RecordingTypeActiveSpeaker,
		RecordingStart: time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC),
		FileExtention:  string(FileTypeMP4),
		DownloadURL:    c.config.APIEndpoint.JoinPath("files/123").String(),
	})

	unexpectError(t, err)
	eq(t, path.Join(c.config.Directory, "/static/2018-01-01_00-00-00_active_speaker.mp4"), fpath)

	stat, err := os.Stat(fpath)
	eq(t, false, err == os.ErrNotExist)
	eq(t, true, stat.Size() > 0)
}

func TestSweep(t *testing.T) {
	c := SetupTest("tmp_test_sweep")
	defer os.RemoveAll(c.config.Directory) //nolint: errcheck

	c.config.RecordingTypes = []string{string(RecordingTypeActiveSpeaker), string(RecordingTypeGallery), string(RecordingTypeGallery), string(RecordingTypeSpeaker)}
	c.config.IgnoreTitles = []string{"ignore"}
	c.config.StartingFromYear = 2022

	unexpectError(t, c.Sweep())

	assertFileExists(t, path.Join(c.config.Directory, "static/2022-10-01_00-00-00_gallery_view.mp4"))
	assertFileExists(t, path.Join(c.config.Directory, "static/2022-10-01_00-00-00_active_speaker.mp4"))
	assertFileExists(t, path.Join(c.config.Directory, "static/2022-11-01_00-00-00_active_speaker.mp4"))
	assertFileExists(t, path.Join(c.config.Directory, "static/2022-11-01_00-00-00_gallery_view.mp4"))
	assertFileExists(t, path.Join(c.config.Directory, "static2/2023-01-01_00-00-00_active_speaker.mp4"))
}

func assertFileExists(t *testing.T, fpath string) {
	_, err := os.Stat(fpath)
	if err != nil && err != fs.ErrNotExist {
		t.Errorf("missing expected file %s", fpath)
	}
}
