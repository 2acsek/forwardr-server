package main

import (
	"errors"
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

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	const PrivateFolder = "/app/private"
	const TorrentFolder = "/app/torrent"

	e.GET("/download", func(c echo.Context) error {
		urlToDownload := c.QueryParam("url")
		fileNameOverride := c.QueryParam("filename")
		folderToDownload := c.QueryParam("folder")
		if folderToDownload == "" {
			folderToDownload = TorrentFolder
		} else if folderToDownload == "private" {
			folderToDownload = PrivateFolder
		} else {
			return c.String(http.StatusBadRequest, "Invalid folder")
		}
		decodedURL, err := url.QueryUnescape(urlToDownload)
		if err != nil {
			return c.String(http.StatusBadRequest, "Invalid URL")
		}
		filename, err := downloadFile(folderToDownload, decodedURL, fileNameOverride)
		if err != nil && filename != "" {
			return c.String(http.StatusInternalServerError, "Failed to download file: "+err.Error())
		}
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to download file: "+err.Error())
		}
		return c.String(http.StatusOK, "File downloaded successfully: "+filename)
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

func downloadFile(folder string, url string, overrideFilename string) (string, error) {
	// This function should handle the actual downloading of the file.
	resp, err := http.Get(url)
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

	filepath := filepath.Join(folder, filename)
	out, err := os.Create(filepath)
	if err != nil {
		return "", err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}
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
