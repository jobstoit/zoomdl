package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"time"
)

const (
	FileTypeMP4        FileType = "MP4"
	FileTypeMPA        FileType = "MPA"
	FileTypeTimeline   FileType = "TIMELINE"
	FileTypeTranscript FileType = "TRANSCRIPT"
	FileTypeChat       FileType = "CHAT"
	FileTypeCC         FileType = "CC"
	FileTypeCSV        FileType = "CSV"

	RecordingTypeScharedScreenWithSpeakerCC RecordingType = "shared_screen_with_speaker_view(CC)"
	RecordingTypeScharedScreenWithSpeaker   RecordingType = "shared_screen_with_speaker_view"
	RecordingTypeScharedScreenWithGallery   RecordingType = "shared_screen_with_gallery_view"
	RecordingTypeSpeaker                    RecordingType = "speaker_view"
	RecordingTypeGallery                    RecordingType = "gallery_view"
	RecordingTypeSharedScreen               RecordingType = "shared_screen"
	RecordingTypeAudioOnly                  RecordingType = "audio_only"
	RecordingTypeAudioTranscript            RecordingType = "audio_transcript"
	RecordingTypeChat                       RecordingType = "chat_file"
	RecordingTypeActiveSpeaker              RecordingType = "active_speaker"
	RecordingTypePoll                       RecordingType = "poll"
	RecordingTypeTimeline                   RecordingType = "timeline"
	RecordingTypeClosedCaption              RecordingType = "closed_caption"
)

// FileType describes the cloud recording filetypes
type FileType string

// RecordingType describes the cloud recording types
type RecordingType string

// ListAllRecordsResponse
type ListAllRecordsResponse struct {
	SessionID      string          `json:"session_id"`
	SessionName    string          `json:"session_name"`
	StartTime      time.Time       `json:"start_time"`
	RecordingCount int             `json:"recording_count"`
	RecordingFiles []RecordingFile `json:"recording_files"`
}

// RecordingFile describes the
type RecordingFile struct {
	ID            string        `json:"id"`
	FileType      FileType      `json:"file_type"`
	RecordingType RecordingType `json:"recording_type"`
	DownloadURL   string        `json:"download_url"`
}

// ZoomClient handles transactions with the zoom Video SDK API v2.0.0
// https://marketplace.zoom.us/docs/api-reference/video-sdk
type ZoomClient struct {
	BaseURL *url.URL
	cli     *http.Client
}

type ZoomClientOption func(*ZoomClient) error

func NewZoomCLient(opts ...ZoomClientOption) (*ZoomClient, error) {
	z := &ZoomClient{}

	for _, opt := range opts {
		if err := opt(z); err != nil {
			return nil, err
		}
	}

	if z.BaseURL == nil {
		u, err := url.Parse("https://api.zoom.us/v2")
		if err != nil {
			return nil, err
		}
		z.BaseURL = u
	}

	return z, nil
}

func WithBaseURL(baseURL string) ZoomClientOption {
	return func(z *ZoomClient) error {
		u, err := url.Parse(baseURL)
		if err != nil {
			return err
		}

		z.BaseURL = u
		return nil
	}
}

// ListAllRecordings returns all recordings
func (z ZoomClient) ListAllRecordings() ([]ListAllRecordsResponse, error) {
	recordings := []ListAllRecordsResponse{}

	url := path.Join(z.BaseURL.String(), "/videosdk/recordings")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return recordings, err
	}

	res, err := z.cli.Do(req)
	if err != nil {
		return recordings, err
	}

	if err := json.NewDecoder(res.Body).Decode(&recordings); err != nil {
		return recordings, err
	}

	return recordings, nil
}
