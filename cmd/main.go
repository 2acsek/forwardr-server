package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var downloads = make(map[string]string)

func main() {
	e := echo.New()
	e.Debug = true
	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	const PrivateFolder = "/app/private"
	const TorrentFolder = "/app/torrent"

	e.GET("/download", func(c echo.Context) error {
		urlEncoded := c.QueryParam("url")
		urlB64, err := url.QueryUnescape(string(urlEncoded))
		if err != nil {
			return c.String(http.StatusBadRequest, "Can not unescape URL")
		}

		urlDecoded, err := base64.StdEncoding.DecodeString(urlB64)
		if err != nil {
			return c.String(http.StatusBadRequest, "Can not decode URL")
		}
		urlToDownload, err := url.Parse(string(urlDecoded))
		if err != nil {
			return c.String(http.StatusBadRequest, "Can not parse URL")
		}
		fileNameOverride := c.QueryParam("filename")

		go func() {
			downloadFile(TorrentFolder, urlToDownload, fileNameOverride)
		}()

		return c.String(http.StatusOK, "Download started successfully: "+urlToDownload.String())
	})

	e.GET("/download/private", func(c echo.Context) error {
		urlEncoded := c.QueryParam("url")
		urlB64, err := url.QueryUnescape(string(urlEncoded))
		if err != nil {
			return c.String(http.StatusBadRequest, "Can not unescape URL")
		}

		urlDecoded, err := base64.StdEncoding.DecodeString(urlB64)
		if err != nil {
			return c.String(http.StatusBadRequest, "Can not decode URL")
		}
		urlToDownload, err := url.Parse(string(urlDecoded))
		if err != nil {
			return c.String(http.StatusBadRequest, "Can not parse URL")
		}
		fileNameOverride := c.QueryParam("filename")
		go func() {
			downloadFile(PrivateFolder, urlToDownload, fileNameOverride)
		}()

		return c.String(http.StatusOK, "Download started successfully: "+urlToDownload.String())
	})

	e.GET("/status", func(c echo.Context) error {
		html := `<html>
        <head>
            <title>Download Status</title>
            <meta http-equiv="refresh" content="2"> <!-- Auto-refresh every 2 seconds -->
        </head>
        <body>
            <h1>Download Progress</h1>
            <table border="1">
                <tr>
                    <th>Filename</th>
                    <th>Progress</th>
                </tr>`

		// Generate table rows for each download
		for filename, progress := range downloads {
			html += `<tr>
                    <td>` + filename + `</td>
                    <td>` + fmt.Sprintf("%s", progress) + `</td>
                </tr>`
		}

		html += `</table>
        </body>
    </html>`

		return c.HTML(http.StatusOK, html)
	})

	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "Health is OK!!")
	})

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	e.Logger.Fatal(e.Start(":" + httpPort))
}

func downloadFile(folder string, fileURL *url.URL, overrideFilename string) (string, error) {
	request, err := http.NewRequest("GET", fileURL.String(), nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			req.Header.Set("Host", fileURL.Host)
			req.Host = fileURL.Host
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
		return "", err
	}
	defer resp.Body.Close()

	var filename string
	if overrideFilename != "" {
		filename = overrideFilename
	} else {
		filename, err = getFilenameFromHeader(resp)
		if err != nil {
			return "", err
		}
		if filename == "" {
			filename = path.Base(resp.Request.URL.Path)
			if filename == "" || filename == "/" {
				return "unknown_file", errors.New("could not determine filename")
			}
		}
	}

	// Track progress
	contentLength := resp.ContentLength

	filepath := filepath.Join(folder, filename)
	out, err := os.Create(filepath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	downloads[filename] = "0" // Initialize progress to 0%

	buffer := make([]byte, 1024*1024) // 1MB buffer
	var totalWritten int64
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			written, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				downloads[filename] = "Failed"
				return "", writeErr
			}
			totalWritten += int64(written)

			// Update progress
			if contentLength > 0 {
				progress := int(float64(totalWritten) / float64(contentLength) * 100)
				downloads[filename] = fmt.Sprintf("In progress - %d%%", progress)
			} else {
				downloads[filename] = "In progress - unknown size"
			}
		}

		if err == io.EOF {
			// Check if the total bytes written match the content length
			if totalWritten == contentLength || contentLength == -1 {
				break
			} else {
				downloads[filename] = "Failed"
				return "", errors.New("download incomplete")
			}
		}

		if err != nil {
			downloads[filename] = "Failed"
			return "", err
		}
	}

	downloads[filename] = "Success"
	return filename, nil
}

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
