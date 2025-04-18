package service

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

func RetryDownload(store *model.Store, id string) error {
	dl, exists := store.Get(id)
	if exists {
		dl.Error = ""
		go downloadWorker(dl)
	} else {
		return errors.New("invalid id")
	}

	return nil
}

func downloadWorker(dl *model.Download) {
	dl.Status = model.StatusRunning
	defer func() {
		if dl.Status != model.StatusCompleted {
			dl.Status = model.StatusFailed
		}
	}()

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

	if dl.FileName == "" {
		fileNameFromHeader, err := getFilenameFromHeader(resp)
		if err != nil || fileNameFromHeader == "" {
			dl.Status = model.StatusFailed
			dl.Error = "filename could not be detected"
		}
		unescapedFileName, err := url.QueryUnescape(fileNameFromHeader)
		if err != nil {
			dl.Status = model.StatusFailed
			dl.Error = "filename could not be detected"
		}
		dl.FileName = unescapedFileName
	}

	filePath := filepath.Join(dl.Path, dl.FileName)
	_ = os.MkdirAll(dl.Path, 0755)

	var downloaded int64 = 0
	var out *os.File

	// Check for existing file
	if fi, statErr := os.Stat(filePath); statErr == nil {
		out, err = os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
		downloaded = fi.Size()
		reRequest, _ := http.NewRequest("GET", dl.URL, nil)
		if downloaded > 0 {
			reRequest.Header.Set("Range", fmt.Sprintf("bytes=%d-", downloaded))
		}

		resp, err = client.Do(reRequest)
		if err != nil {
			dl.Error = err.Error()
			return
		}
	} else {
		out, err = os.Create(filePath)
	}
	if err != nil {
		dl.Error = err.Error()
		return
	}
	defer out.Close()

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		dl.Error = fmt.Sprintf("Unexpected status: %d", resp.StatusCode)
		return
	}

	cl := resp.Header.Get("Content-Length")
	if cl != "" {
		if sz, err := strconv.ParseInt(cl, 10, 64); err == nil {
			dl.TotalBytes = downloaded + sz
		}
	}

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

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				dl.Error = err.Error()
				return
			}
			downloaded += int64(n)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			dl.Error = err.Error()
			return
		}
		select {
		case <-progressChan:
			if dl.TotalBytes > 0 {
				dl.Progress = float64(downloaded) / float64(dl.TotalBytes) * 100
			}
		default:
		}
	}

	dl.Progress = 100
	dl.Status = model.StatusCompleted
}
