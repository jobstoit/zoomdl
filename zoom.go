package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jobstoit/httpio"
)

const API_CALL_CONCURRENCY_LIMIT = 2

const (
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
	RecordingType  RecordingType `json:"recording_type"`
	RecordingStart time.Time     `json:"recording_start"`
	FileExtension  string        `json:"file_extension"`
	DownloadURL    string        `json:"download_url"`
}

// ZoomClient handles transactions with the zoom Video SDK API v2.0.0
// https://marketplace.zoom.us/docs
type ZoomClient struct {
	BaseURL *url.URL
	config  *Config
	cli     *http.Client
	token   *AccessToken
	mut     chan bool
	context context.Context
	fs      FileSystem
}

func (z *ZoomClient) lock() {
	z.mut <- true
}

func (z *ZoomClient) unlock() {
	<-z.mut
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
func NewZoomClient(cfg *Config, fs FileSystem) *ZoomClient {
	z := &ZoomClient{}
	z.config = cfg
	z.cli = &http.Client{}
	z.BaseURL = z.config.APIEndpoint
	z.mut = make(chan bool, cfg.Concurrency)
	z.fs = fs

	return z
}

// Authorize
func (z *ZoomClient) Authorize() (*AccessToken, error) {
	z.lock()
	defer z.unlock()

	a := &AccessToken{}

	formData := url.Values{}
	formData.Add(`grant_type`, `account_credentials`)
	formData.Add(`account_id`, z.config.UserID)

	endpointURL := z.config.AuthEndpoint.JoinPath("oauth/token").String()
	req, err := http.NewRequest(http.MethodPost, endpointURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	bearer := base64.StdEncoding.EncodeToString(fmt.Appendf([]byte{}, "%s:%s", z.config.ClientID, z.config.ClientSecret))
	req.Header.Add(`Authorization`, fmt.Sprintf("Basic %s", bearer))
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
func (z *ZoomClient) ListAllRecordings(from time.Time) ([]Meeting, error) {
	if z.token == nil || time.Now().After(z.token.ExpiresAt) {
		at, err := z.Authorize()
		if err != nil {
			return nil, err
		}
		z.token = at
	}

	concurrency := min(z.config.Concurrency, API_CALL_CONCURRENCY_LIMIT)

	ch := make(chan meetingsChan, concurrency)
	count := 0

	endpointURL := z.BaseURL.JoinPath("users/me/recordings")
	if from.IsZero() {
		from = time.Date(z.config.StartingFromYear, 1, 1, 0, 0, 0, 0, time.Local)
	}

	now := time.Now()

	for d := now; !d.Before(from); d = d.AddDate(0, -1, 0) {
		go z.getMeetings(ch, endpointURL, d)
		count++
	}

	meetings := []Meeting{}

	for range count {
		res := <-ch
		if res.err != nil {
			return meetings, res.err
		}

		meetings = append(meetings, res.meetings...)
	}

	close(ch)

	meetings = clearDuplicateMeetings(meetings)

	return meetings, nil
}

func dateFormat(t time.Time) string {
	y, m, d := t.Date()
	return fmt.Sprintf("%04d-%02d-%02d", y, int(m), d)
}

type meetingsChan struct {
	meetings []Meeting
	err      error
}

func (z *ZoomClient) getMeetings(ch chan meetingsChan, endpoint *url.URL, from time.Time) {
	z.lock()
	defer z.unlock()

	res := meetingsChan{}
	query := endpoint.Query()
	query.Set("page_size", "300")
	query.Set("from", dateFormat(from.AddDate(0, -1, 0)))
	query.Set("to", dateFormat(from))

	for {
		m, err := z.getMeeting(endpoint, query)
		if err != nil {
			res.err = err
			break
		}

		res.meetings = m.Meetings
		if m.NextPageToken == "" {
			break
		}

		query.Set("next_page_token", m.NextPageToken)
	}

	query.Del("next_page_token")
	endpoint.RawQuery = query.Encode()

	ch <- res
}

func (z *ZoomClient) getMeeting(endpoint *url.URL, queries url.Values) (ListAllRecordsResponse, error) {
	r := ListAllRecordsResponse{}
	endpoint.RawQuery = queries.Encode()

	res, err := z.do(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return r, err
	}
	defer res.Body.Close()

	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return r, err
	}

	return r, nil
}

func clearDuplicateMeetings(ms []Meeting) []Meeting {
	keys := map[string]bool{}

	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		if _, exists := keys[m.UUID]; exists {
			ms = append(ms[:i], ms[i+1:]...)
			continue
		}

		keys[m.UUID] = true
	}

	return ms
}

// DeleteRecording deletes the recording on zoom
func (z *ZoomClient) DeleteRecording(id int) error {
	url := z.BaseURL.JoinPath("meetings", strconv.Itoa(id), "recordings").String()

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
	fileExtention := strings.ToLower(rec.FileExtension)

	target := path.Join(sessionTitle, fmt.Sprintf("%04d-%02d-%02d_%02d-%02d-%02d_%s.%s",
		rec.RecordingStart.Year(),
		int(recordingTime.Month()),
		recordingTime.Day(),
		recordingTime.Hour(),
		recordingTime.Minute(),
		recordingTime.Second(),
		string(rec.RecordingType),
		fileExtention,
	))

	file, err := z.fs.Writer(z.context, target)
	if err != nil {
		return "", err
	}
	defer file.Close() // nolint: errcheck

	if z.token == nil || time.Now().After(z.token.ExpiresAt) {
		at, err := z.Authorize()
		if err != nil {
			return "", err
		}
		z.token = at
	}

	remoteFile, err := httpio.Get(
		rec.DownloadURL,
		httpio.WithClient(z.cli),
		httpio.WithChunkSize(1024*1024*z.config.ChunckSizeMB),
		httpio.WithConcurrency(z.config.Concurrency),
		httpio.WithHeader("Authorization", fmt.Sprintf("Bearer %s", z.token.AccessToken)),
	)
	if err != nil {
		log.Printf("error fetching data: %v", err)
		return "", err
	}

	if _, err := io.Copy(file, remoteFile); err != nil {
		log.Printf("error writing data: %v", err)
		return "", err
	}

	return target, err
}

// RecordHolder holds stores the saved records
type RecordHolder struct {
	Records []SavedRecord
}

// Sweep will get all the records and download the specified files
func (z *ZoomClient) Sweep() error {
	ctx, cancel := context.WithCancel(context.Background())
	z.context = ctx
	defer cancel()

	saveFile, err := z.fs.Reader(z.context, SavedRecordFileName)
	if err != nil {
		return err
	}

	records := &RecordHolder{}
	if err := json.NewDecoder(saveFile).Decode(records); err != nil && err != io.EOF {
		log.Printf("unable to read record file: %v", err)
	}

	defer z.saveRecords(ctx, records)

	var from time.Time
	if len(records.Records) > 0 {
		from = records.Records[len(records.Records)-1].RecordedAt
	}

	log.Print(`pulling recordings`)
	meetings, err := z.ListAllRecordings(from)
	if err != nil {
		return err
	}

	log.Printf("fetched %d entries", len(meetings))
	recordIDs := getRecordMap(records.Records)
	allowedTypes := strings.Join(z.config.RecordingTypes, " ")
	ignoredTitles := strings.Join(z.config.IgnoreTitles, " ")

	var errs error
	for _, meeting := range meetings {
		if ignoredTitles != "" && strings.Contains(ignoredTitles, meeting.Topic) {
			goto CLEANUP
		}

		for _, rf := range meeting.RecordingFiles {
			if rf.FileExtension == "" ||
				string(rf.RecordingType) == "" ||
				strings.Contains(recordIDs, rf.ID) ||
				!strings.Contains(allowedTypes, string(rf.RecordingType)) {
				continue
			}

			log.Printf("Downloading '%s' from %v of type %s", meeting.Topic, rf.RecordingStart, rf.RecordingType)
			filePath, err := z.DownloadVideo(meeting.Topic, rf)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}

			records.Records = append(records.Records, SavedRecord{
				ID:         rf.ID,
				SessionID:  meeting.UUID,
				Path:       filePath,
				SavedAt:    time.Now(),
				RecordedAt: rf.RecordingStart,
			})
		}

	CLEANUP:
		if z.config.DeleteAfter {
			log.Printf("Deleting '%s' from %v", meeting.Topic, meeting.StartTime)
			if err := z.DeleteRecording(meeting.ID); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}
	log.Print(`finished fetching recordings`)

	return errs
}

func (z *ZoomClient) saveRecords(ctx context.Context, records *RecordHolder) {
	file, err := z.fs.Writer(ctx, SavedRecordFileName)
	if err != nil {
		log.Printf("error opening writer for saving file: %v", err)
	}

	slices.SortFunc(records.Records, func(a, b SavedRecord) int {
		return a.RecordedAt.Compare(b.RecordedAt)
	})

	err = json.NewEncoder(file).Encode(records)
	if err != nil {
		log.Printf("error encoding file: %v", err)
	}

	err = file.Close()
	if err != nil {
		log.Printf("error closing file: %v", err)
	}
}

func (z *ZoomClient) do(method, url string, body io.Reader, opts ...func(*http.Request)) (*http.Response, error) {
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
	req = req.WithContext(z.context)

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", z.token.AccessToken))

	for _, opt := range opts {
		opt(req)
	}

	res, err := z.cli.Do(req)
	if err != nil {
		log.Printf("error getting request: %s %s", req.Method, url)
		return nil, err
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
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
