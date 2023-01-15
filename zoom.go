package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
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
	NextPageToken string    `json:"next_page_token"`
	Meetings      []Meeting `json:"meetings"`
}

// Meeting contains the meeting details
type Meeting struct {
	ID             int             `json:"id"`
	UUID           string          `json:"uuid"`
	Topic          string          `json:"topic"`
	RecordingFiles []RecordingFile `json:"recording_files"`
	StartTime      time.Time       `json:"-"`
}

// RecordingFile describes the
type RecordingFile struct {
	ID             string        `json:"id"`
	FileType       FileType      `json:"file_type"`
	RecordingType  RecordingType `json:"recording_type"`
	RecordingStart time.Time     `json:"recording_start"`
	FileExtention  string
	DownloadURL    string `json:"download_url"`
}

// ZoomClient handles transactions with the zoom Video SDK API v2.0.0
// https://marketplace.zoom.us/docs
type ZoomClient struct {
	BaseURL *url.URL
	config  *Config
	cli     *http.Client
	token   *AccessToken
}

type AccessToken struct {
	AccessToken string    `json:"access_token"`
	ExpiresIn   int       `json:"expires_in"`
	Scope       string    `json:"scope"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"-"`
}

type ZoomClientOption func(*ZoomClient) error

// NewZoomClient creates a new instance of the zoom client
func NewZoomClient(cfg *Config) *ZoomClient {
	z := &ZoomClient{}
	z.config = cfg
	z.cli = &http.Client{}
	z.BaseURL = z.config.APIEndpoint

	return z
}

// Authorize
func (z *ZoomClient) Authorize() (*AccessToken, error) {
	a := &AccessToken{}

	formData := url.Values{}
	formData.Add(`grant_type`, `account_credentials`)
	formData.Add(`account_id`, z.config.UserID)

	endpointURL := z.config.AuthEndpoint.JoinPath("oauth/token").String()
	req, err := http.NewRequest(http.MethodPost, endpointURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	bearer := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", z.config.ClientID, z.config.ClientSecret)))
	req.Header.Add(`Authorization`, fmt.Sprintf("Basic %s", bearer))
	req.Header.Add(`Host`, "zoom.us")
	req.Header.Add(`Content-Type`, "application/x-www-form-urlencoded")

	res, err := z.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close() //nolint: errcheck

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to authorize with account id: %s and client id: %s, status %d, message: %s", z.config.UserID, z.config.ClientID, res.StatusCode, res.Body)
	}

	if err := json.NewDecoder(res.Body).Decode(a); err != nil {
		return nil, err
	}

	dur, err := time.ParseDuration(fmt.Sprintf("%ds", a.ExpiresIn))
	if err != nil {
		return nil, err
	}
	a.ExpiresAt = time.Now().Add(dur)

	return a, nil
}

// ListAllRecordings returns all recordings
func (z *ZoomClient) ListAllRecordings() ([]Meeting, error) {
	meetings := []Meeting{}

	endpointURL := z.BaseURL.JoinPath("users/me/recordings")

	now := time.Now()
	from := time.Date(z.config.StartingFromYear, 1, 1, 0, 0, 0, 0, time.Local)
	for d := from; !d.After(now); d = d.AddDate(0, 1, 0) {
		to := d.AddDate(0, 1, 0)

		resp := ListAllRecordsResponse{}

		query := endpointURL.Query()
		query.Set("page_size", "300")
		query.Set("from", fmt.Sprintf("%04d-%02d-%02d", d.Year(), int(d.Month()), d.Day()))
		query.Set("to", fmt.Sprintf("%04d-%02d-%02d", to.Year(), int(to.Month()), to.Day()))
		endpointURL.RawQuery = query.Encode()

		res, err := z.do(http.MethodGet, endpointURL.String(), nil)
		if err != nil {
			return meetings, err
		}
		defer res.Body.Close() //nolint: errcheck

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return meetings, err
		}

		if err := json.Unmarshal(body, &resp); err != nil {
			log.Printf("error parsing json: %s", body)
			return meetings, err
		}

		meetings = append(meetings, resp.Meetings...)
	}

	meetings = clearDuplicateMeetings(meetings)

	return meetings, nil
}

func clearDuplicateMeetings(ms []Meeting) []Meeting {
	keys := map[string]bool{}
	list := []Meeting{}

	for _, m := range ms {
		if _, exists := keys[m.UUID]; !exists {
			keys[m.UUID] = true
			list = append(list, m)
		}
	}

	return list
}

// DeleteRecording deletes the recording on zoom
func (z *ZoomClient) DeleteRecording(id string) error {
	url := z.BaseURL.JoinPath("meetings", id, "recordings").String()

	res, err := z.do(http.MethodDelete, url, &bytes.Buffer{})
	if err != nil {
		return err
	}
	defer res.Body.Close() //nolint: errcheck

	return nil
}

// DownloadVideo downloads the video to the given file and returns the path
func (z *ZoomClient) DownloadVideo(sessionTitle string, rec RecordingFile) (string, error) {
	recordingTime := rec.RecordingStart

	sessionTitle = serializPathString(sessionTitle)
	fileExtention := fileExtention(rec.FileType)

	filepath := path.Join(z.config.Directory, sessionTitle, fmt.Sprintf("%04d-%02d-%02d_%02d-%02d-%02d_%s.%s",
		rec.RecordingStart.Year(),
		int(recordingTime.Month()),
		recordingTime.Day(),
		recordingTime.Hour(),
		recordingTime.Minute(),
		recordingTime.Second(),
		string(rec.RecordingType),
		fileExtention,
	))

	if err := os.MkdirAll(path.Dir(filepath), os.ModePerm); err != nil && err != os.ErrExist {
		return ``, err
	}

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer file.Close() // nolint: errcheck

	res, err := z.do(http.MethodGet, rec.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	defer res.Body.Close() //nolint: errcheck

	_, err = io.Copy(file, res.Body)
	return filepath, err
}

// RecordHolder holds stores the saved records
type RecordHolder struct {
	Records []SavedRecord
}

// Sweep will get all the records and download the specified files
func (z *ZoomClient) Sweep() error {
	recFileName := path.Join(z.config.Directory, SavedRecordFileName)
	fbody, err := os.ReadFile(recFileName)
	if err != nil {
		return err
	}

	records := &RecordHolder{}
	if len(fbody) > 0 {
		if err := json.Unmarshal(fbody, &records.Records); err != nil && err != io.EOF {
			return err
		}
	}

	defer saveRecords(records, recFileName)

	log.Print(`pulling recordings`)
	meetings, err := z.ListAllRecordings()
	if err != nil {
		return err
	}

	log.Printf("fetched %d entries", len(meetings))
	recordIDs := getRecordMap(records.Records)
	allowedTypes := strings.Join(z.config.RecordingTypes, " ")
	ignoredTitles := strings.Join(z.config.IgnoreTitles, " ")

	for _, meeting := range meetings {
		if ignoredTitles != "" && strings.Contains(ignoredTitles, meeting.Topic) {
			continue
		}

		for _, rf := range meeting.RecordingFiles {
			if strings.Contains(recordIDs, rf.ID) ||
				!strings.Contains(allowedTypes, string(rf.RecordingType)) {
				continue
			}

			log.Printf("Downloading '%s' from %v of type %s", meeting.Topic, rf.RecordingStart, rf.RecordingType)
			filePath, err := z.DownloadVideo(meeting.Topic, rf)
			if err != nil {
				return err
			}

			records.Records = append(records.Records, SavedRecord{
				ID:         rf.ID,
				SessionID:  meeting.UUID,
				Path:       filePath,
				SavedAt:    time.Now(),
				RecordedAt: rf.RecordingStart,
			})
		}

		if z.config.DeleteAfter {
			if err := z.DeleteRecording(meeting.UUID); err != nil {
				return err
			}
		}
	}
	log.Print(`finished fetching recordings`)

	return nil
}

func saveRecords(records *RecordHolder, filename string) {
	fbody, err := json.Marshal(records.Records)
	if err != nil {
		log.Print(`unable to marshal saved records`)
		return
	}

	if err := os.WriteFile(filename, fbody, 0644); err != nil {
		log.Print(`unable to save saved records`)
	}
}

func (z *ZoomClient) do(method, url string, body io.Reader) (*http.Response, error) {
	if z.token == nil || time.Now().After(z.token.ExpiresAt) {
		at, err := z.Authorize()
		if err != nil {
			return nil, err
		}
		z.token = at
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Print(`error creating request`)
		return nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", z.token.AccessToken))
	req.Header.Add("Host", `api.zoom.us`)

	res, err := z.cli.Do(req)
	if err != nil {
		log.Print(`error getting request`)
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		buff := &bytes.Buffer{}
		buff.ReadFrom(res.Body) //nolint: errcheck
		return nil, fmt.Errorf("failed request statuscode %d and body: %s", res.StatusCode, buff.String())
	}

	return res, nil
}

func serializPathString(s string) string {
	s = strings.ReplaceAll(s, `'`, ``)
	s = strings.ReplaceAll(s, ":", " -")
	return s
}

func getRecordMap(records []SavedRecord) string {
	var s string

	for _, rec := range records {
		s += " " + rec.ID
	}

	return s
}

func fileExtention(fileType FileType) string {
	switch fileType {
	case FileTypeCSV, FileTypeChat, FileTypeTimeline, FileTypeTranscript, FileTypeCC:
		return "txt"
	case FileTypeMPA:
		return "mp4a"
	default:
		return "mp4"
	}
}
