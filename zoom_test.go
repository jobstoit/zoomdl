package main

import (
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

func TestListAllRecordings(t *testing.T) {
	dir := "tmp_list_all_recordings"
	c := SetupTest(t, dir)

	c.config.StartingFromYear = 2017

	meetings, err := c.ListAllRecordings()
	assert(t, err == nil, "unexpected error listing recordings")
	assert(t, len(meetings) > 1, "missing expected recordings")

	for _, meeting := range meetings {
		switch meeting.ID {
		case 1001:
			assert(t, meeting.Topic == "static", "meeting topic must me static")
			assert(t, len(meeting.RecordingFiles) == 4, "meeting must have 4 recording files")
			if len(meeting.RecordingFiles) != 4 {
				continue
			}

			rf := meeting.RecordingFiles[0]
			assert(t, rf.RecordingType == RecordingTypeAudioOnly, "meeting recording type must be of type audio_only")
		}
	}

	assert(t, len(meetings) == 15, "expect 15 recordings")
}

func TestDownload(t *testing.T) {
	dir := "tmp_test_download"
	c := SetupTest(t, dir)

	fpath, err := c.DownloadVideo("static", RecordingFile{
		RecordingType:  RecordingTypeActiveSpeaker,
		RecordingStart: time.Date(2018, time.January, 1, 0, 0, 0, 0, time.UTC),
		FileExtension:  "MP4",
		DownloadURL:    c.config.APIEndpoint.JoinPath("files/123").String(),
	})

	assert(t, err == nil, "error must be nil")
	assert(t,
		path.Join("static/2018-01-01_00-00-00_active_speaker.mp4") == fpath,
		"path must be equal",
	)

	stat, err := os.Stat(path.Join(dir, fpath))
	assert(t, err == nil, "getting file stat error must be nil")
	if stat != nil {
		assert(t, stat.Size() > 0, "downloaded filesize must be bigger than zero")
	}
}

func TestSweep(t *testing.T) {
	dir := "tmp_test_sweep"
	c := SetupTest(t, dir)

	c.config.RecordingTypes = []string{
		string(RecordingTypeActiveSpeaker),
		string(RecordingTypeGallery),
		string(RecordingTypeGallery),
		string(RecordingTypeSpeaker),
	}
	c.config.IgnoreTitles = []string{"ignore"}
	c.config.StartingFromYear = 2022

	if err := c.Sweep(); err != nil {
		t.Fatalf("unexpected error during sweep: %v", err)
	}

	assertFileExists(t, path.Join(dir, "static/2022-10-01_00-00-00_gallery_view.mp4"))
	assertFileExists(t, path.Join(dir, "static/2022-10-01_00-00-00_active_speaker.mp4"))
	assertFileExists(t, path.Join(dir, "static/2022-11-01_00-00-00_active_speaker.mp4"))
	assertFileExists(t, path.Join(dir, "static/2022-11-01_00-00-00_gallery_view.mp4"))
	assertFileExists(t, path.Join(dir, "static2/2023-01-01_00-00-00_active_speaker.mp4"))
	assertFileNotExists(t, path.Join(dir, "ignore/2023-01-02_00-00-00_.mp4"))
}

func TestDeleteRecording(t *testing.T) {
	c := SetupTest(t, "tmp_test_delete")

	assert(t, c.DeleteRecording(1001) == nil, "deletion doesnt return an error")
	assert(t, c.DeleteRecording(1001) != nil, "deletion returns an error")
}

func assertFileExists(t *testing.T, fpath string) {
	_, err := os.Stat(fpath)
	if err != nil {
		t.Errorf("missing expected file %s", fpath)
	}
}

func assertFileNotExists(t *testing.T, fpath string) {
	_, err := os.Stat(fpath)
	if !strings.HasSuffix(err.Error(), "no such file or directory") {
		t.Errorf("unexpected file %s, err: %v", fpath, err)
	}
}
