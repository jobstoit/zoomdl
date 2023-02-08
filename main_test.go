package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// SetupTest sets up the tests
func SetupTest(dir string) *ZoomClient {
	mock := NewZoomMockAPI()
	server := httptest.NewServer(mock)
	endpointURL, _ := url.Parse(server.URL) //nolint: errcheck

	mock.baseURL = endpointURL
	mock.Seed()

	createSaveFile(dir)

	return NewZoomClient(&Config{
		APIEndpoint:  endpointURL,
		AuthEndpoint: endpointURL,
		Directory:    dir,
		Concurrency:  2,
		ChunckSizeMB: 4,
	})
}

// ZoomMockAPI mocks the zoom api for testing
type ZoomMockAPI struct {
	baseURL  *url.URL
	meetings []Meeting
}

// NewZoomMockAPI returns a new mock api
func NewZoomMockAPI() *ZoomMockAPI {
	z := &ZoomMockAPI{}
	z.meetings = []Meeting{}

	return z
}

// Seed will populate the mock api with random data
func (z *ZoomMockAPI) Seed() {
	from := time.Date(2017, time.January, 1, 0, 0, 0, 0, time.UTC)
	topics := []string{
		"Zooming About",
		"Buzzword",
		"Above and Beyond",
		"Dancing in the moonlight",
	}

	z.meetings = []Meeting{
		createMeeting(z.baseURL, `static`, 1001, time.Date(2022, time.October, 1, 0, 0, 0, 0, time.UTC), RecordingTypeAudioOnly, RecordingTypeActiveSpeaker, RecordingTypeGallery, RecordingTypeScharedScreenWithSpeakerCC),
		createMeeting(z.baseURL, `static`, 1002, time.Date(2022, time.November, 1, 0, 0, 0, 0, time.UTC), RecordingTypeGallery, RecordingTypeActiveSpeaker),
		createMeeting(z.baseURL, `static`, 1003, time.Date(2022, time.December, 1, 0, 0, 0, 0, time.UTC), RecordingTypeGallery, RecordingTypeActiveSpeaker),
		createMeeting(z.baseURL, `static2`, 1004, time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC), RecordingTypeGallery, RecordingTypeActiveSpeaker),
		createMeeting(z.baseURL, `ignore`, 1005, time.Date(2023, time.January, 2, 0, 0, 0, 0, time.UTC), RecordingTypeGallery, RecordingTypeActiveSpeaker),
		createMeeting(z.baseURL, `ignore`, 1005, time.Date(2023, time.January, 2, 0, 0, 0, 0, time.UTC), RecordingTypeGallery, RecordingTypeActiveSpeaker),
	}

	for i := 0; i < 10; i++ {
		topic := topics[rand.Intn(len(topics))]
		z.meetings = append(z.meetings, createRandomMeeting(z.baseURL, topic, i, from))
	}

	log.Printf("added %d entries", len(z.meetings))
}

// ServeHTTP is an implementation of http.Handler
func (z *ZoomMockAPI) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/me/recordings", z.listAllRecordings)
	mux.HandleFunc("/oauth/token", z.authorize)
	mux.HandleFunc("/meetings/1001/recordings", z.deleteMeeting)

	if strings.HasPrefix(r.URL.Path, "/files") {
		z.download(wr, r)
		return
	}

	mux.ServeHTTP(wr, r)
}

func (z *ZoomMockAPI) listAllRecordings(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		wr.WriteHeader(http.StatusNotFound)
		return
	}

	queries := r.URL.Query()
	now := time.Now()
	from := getDate(queries.Get("from"), now).Unix()
	to := getDate(queries.Get("to"), now).Unix()

	res := ListAllRecordsResponse{}
	for _, meet := range z.meetings {
		start := meet.StartTime.Unix()
		if start > from && to < start {
			res.Meetings = append(res.Meetings, meet)

		}
	}

	if err := json.NewEncoder(wr).Encode(res); err != nil {
		wr.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func getDate(s string, defaultDate time.Time) time.Time {
	if s == "" {
		return defaultDate
	}

	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return defaultDate
	}

	return t
}

func (z *ZoomMockAPI) authorize(wr http.ResponseWriter, r *http.Request) {
	res := AccessToken{
		AccessToken: "granted",
		ExpiresIn:   3333,
		Scope:       "",
		TokenType:   "access_token",
	}

	if err := json.NewEncoder(wr).Encode(res); err != nil {
		wr.WriteHeader(http.StatusInternalServerError)
	}
}

func (z *ZoomMockAPI) download(wr http.ResponseWriter, r *http.Request) {
	wr.Header().Add("Content-Length", "16")

	if r.Method == http.MethodHead {
		wr.WriteHeader(http.StatusOK)
		return
	}

	fmt.Fprint(wr, "some random file")
}

func (z *ZoomMockAPI) deleteMeeting(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		wr.WriteHeader(http.StatusNotFound)
		return
	}

	found := false
	for i := len(z.meetings) - 1; i > -1; i-- {
		if z.meetings[i].UUID == "1001" {
			z.meetings = append(z.meetings[:i], z.meetings[i+1:]...)
			found = true
		}
	}

	if !found {
		wr.WriteHeader(http.StatusNotFound)
		return
	}

	wr.WriteHeader(http.StatusOK)
}

func createRandomMeeting(baseURL *url.URL, topic string, id int, from time.Time) Meeting {
	recordedAt := randdate(from, time.Now())
	recordingTypes := []RecordingType{}
	for i := 0; i < rand.Intn(10); i++ {
		recordingTypes = append(recordingTypes, randomRecordingType())
	}

	return createMeeting(baseURL, topic, id, recordedAt, recordingTypes...)
}

func createMeeting(baseURL *url.URL, topic string, id int, startTime time.Time, recordingTypes ...RecordingType) Meeting {
	m := Meeting{
		ID:        id,
		Topic:     topic,
		UUID:      fmt.Sprintf("%d", id),
		StartTime: startTime,
	}

	for _, typ := range recordingTypes {
		id := randomString(15)
		m.RecordingFiles = append(m.RecordingFiles, RecordingFile{
			ID:             id,
			RecordingStart: startTime,
			DownloadURL:    baseURL.JoinPath("files", id).String(),
			RecordingType:  typ,
			FileExtension:  getFileExtention(typ),
		})
	}

	return m
}

func getFileExtention(rt RecordingType) string {
	switch rt {
	case RecordingTypeAudioTranscript, RecordingTypeTimeline, RecordingTypePoll:
		return "CSV"
	case RecordingTypeChat:
		return "TXT"
	case RecordingTypeAudioOnly:
		return "MP4A"
	default:
		return "MP4"
	}
}

// randdate creates a random time.Time
func randdate(from, to time.Time) time.Time {
	min := from.Unix()
	max := to.Unix()
	delta := max - min

	return time.Unix(rand.Int63n(delta)+min, 0)
}

func randomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func randomRecordingType() RecordingType {
	switch rand.Intn(4) {
	case 1:
		return RecordingTypeActiveSpeaker
	case 2:
		return RecordingTypeScharedScreenWithGallery
	case 3:
		return RecordingTypeScharedScreenWithSpeaker
	case 4:
		return RecordingTypeGallery
	default:
		return RecordingTypeSpeaker
	}
}

func assert(t *testing.T, condition bool, explaination ...string) {
	if !condition {
		explaination = append([]string{"assertion failed:"}, explaination...)
		t.Error(strings.Join(explaination, " "))
	}
}
