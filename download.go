package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
)

// DownloadFile downloads the given file
func DownloadFile(cli *http.Client, url, path string, chunkSize int) error {
	size, ranges, err := GetDownloadSize(cli, url)
	if err != nil {
		return err
	}

	if !ranges {
		chunkSize = size
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close() //nolint: errcheck

	defer recoverCreatingFile(path)

	mut := &sync.Mutex{}
	downloadChuncks(cli, mut, 0, chunkSize, size, url, file)

	return nil
}

func recoverCreatingFile(path string) {
	if v := recover(); v != nil {
		fmt.Printf("error getting file %s: %v", path, v)
		if err := os.Remove(path); err != nil {
			fmt.Printf("error removing corrupt file '%s': %v", path, err)
		}
	}
}

func downloadChuncks(cli *http.Client, mut *sync.Mutex, startAt, ammount, fullLenght int, url string, file io.Writer) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}

	rangeEnd := startAt + ammount
	if rangeEnd > fullLenght {
		rangeEnd = fullLenght
	}

	req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", startAt, rangeEnd))

	res, err := cli.Do(req)
	if err != nil {
		panic(err)
	}

	mut.Lock()
	defer mut.Unlock()

	if rangeEnd != fullLenght {
		go downloadChuncks(cli, mut, rangeEnd, ammount, fullLenght, url, file)
	}

	if _, err := io.Copy(file, res.Body); err != nil {
		panic(err)
	}
}

// GetDownloadSize returns the size of the download and whether the server accepts ranges
func GetDownloadSize(cli *http.Client, url string) (int, bool, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0, false, err
	}

	res, err := cli.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer res.Body.Close() //nolint: errcheck

	if res.StatusCode != http.StatusOK {
		return 0, false, fmt.Errorf("error in getting headers for content")
	}

	contentLength := res.Header.Get("Content-Length")
	i, err := strconv.Atoi(contentLength)
	if err != nil {
		i = 0
	}

	return i, res.Header.Get("Accept-Ranges") == "bytes", nil
}
