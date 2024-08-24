package main

import (
	"context"
	"encoding/json"
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

	meetings, err := c.ListAllRecordings(time.Time{})
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

func TestSaveRecords(t *testing.T) {
	dir := "tmp_test_save"
	c := SetupTest(t, dir)

	ctx := context.Background()

	c.saveRecords(ctx, &RecordHolder{
		Records: []SavedRecord{
			{
				ID:         "random_id2",
				SessionID:  "random_session_id2",
				SavedAt:    time.Now(),
				RecordedAt: time.Date(2022, time.September, 9, 12, 34, 0, 0, time.Local),
				Path:       "random/random2.mp4",
			},
			{
				ID:         "random_id1",
				SessionID:  "random_session_id1",
				SavedAt:    time.Now(),
				RecordedAt: time.Date(2022, time.January, 1, 12, 34, 0, 0, time.Local),
				Path:       "random/random.mp4",
			},
			{
				ID:         "random_id3",
				SessionID:  "random_session_id3",
				SavedAt:    time.Now(),
				RecordedAt: time.Date(2022, time.December, 5, 12, 34, 0, 0, time.Local),
				Path:       "random/random3.mp4",
			},
		},
	})

	rd, err := c.fs.Reader(ctx, SavedRecordFileName)
	if err != nil {
		t.Fatalf("unable to read savefile: %v", err)
	}

	records := &RecordHolder{}
	err = json.NewDecoder(rd).Decode(records)
	if err != nil {
		t.Fatalf("unable to marshal json: %v", err)
	}

	if e, a := 3, len(records.Records); e != a {
		t.Fatalf("expected %d but got %d", e, a)
	}

	if e, a := "random_id1", records.Records[0].ID; e != a {
		t.Errorf("expected %s but got %s", e, a)
	}

	if e, a := "random_id2", records.Records[1].ID; e != a {
		t.Errorf("expected %s but got %s", e, a)
	}

	if e, a := "random_id3", records.Records[2].ID; e != a {
		t.Errorf("expected %s but got %s", e, a)
	}
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
