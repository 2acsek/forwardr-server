package service

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/2acsek/forwardr-server/internal/model"
	"github.com/google/uuid"
)

func getFilenameFromHeader(resp *http.Response) (string, error) {
	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		return "", nil
	}

	// Typical header: Content-Disposition: attachment; filename="example.txt"
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return "", nil
	}

	filename := params["filename"]
	return strings.Trim(filename, `"`), nil
}

func StartDownload(store *model.Store, url, fileName string, path string) string {
	id := uuid.New().String()
	dl := &model.Download{
		ID:       id,
		URL:      url,
		FileName: fileName,
		Path:     path,
		Status:   model.StatusPending,
	}
	store.Add(dl)

	go downloadWorker(dl)

	return id
}

func downloadWorker(dl *model.Download) {
	dl.Status = model.StatusRunning

	request, err := http.NewRequest("GET", dl.URL, nil)
	if err != nil {
		dl.Status = model.StatusFailed
		dl.Error = "Could not create new download request"
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			q := req.URL.Query()
			req.URL.RawQuery = q.Encode()
			if err != nil {
				return err
			}
			return nil
		},
	}

	resp, err := client.Do(request)
	if err != nil {
		dl.Status = model.StatusFailed
		dl.Error = err.Error()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dl.Status = model.StatusFailed
		dl.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		return
	}
	if dl.FileName == "" {
		fileNameFromHeader, err := getFilenameFromHeader(resp)
		if err != nil || fileNameFromHeader == "" {
			dl.Status = model.StatusFailed
			dl.Error = "filename could not be detected"
		}
		dl.FileName = fileNameFromHeader
	}

	dl.TotalBytes = resp.ContentLength

	filePath := filepath.Join(dl.Path, dl.FileName)
	out, err := os.Create(filePath)
	if err != nil {
		dl.Status = model.StatusFailed
		dl.Error = err.Error()
		return
	}
	defer out.Close()

	buffer := make([]byte, 32*1024)
	var downloaded int64

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	progressChan := make(chan struct{}, 1)
	go func() {
		for range ticker.C {
			select {
			case progressChan <- struct{}{}:
			default:
			}
		}
	}()

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := out.Write(buffer[:n]); writeErr != nil {
				dl.Status = model.StatusFailed
				dl.Error = writeErr.Error()
				return
			}
			downloaded += int64(n)
			dl.DoneBytes = downloaded
		}
		select {
		case <-progressChan:
			if dl.TotalBytes >= 0 {
				dl.Progress = float64(downloaded) / float64(dl.TotalBytes) * 100
			} else {
				dl.Progress = -1
			}
		default:
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			dl.Status = model.StatusFailed
			dl.Error = err.Error()
			return
		}
	}

	dl.Progress = 100
	dl.Status = model.StatusCompleted
}
