package handler

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/url"

	"github.com/2acsek/forwardr-server/internal/model"
	"github.com/2acsek/forwardr-server/internal/service"
)

type API struct {
	Store *model.Store
}

func NewHandler(store *model.Store) *API {
	return &API{Store: store}
}

func (api *API) Health(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s - %s\n", r.Method, r.URL.String())
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (api *API) Downloads(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s - %s\n", r.Method, r.URL.String())
	all := api.Store.GetAll()
	writeJSON(w, all)
}

func (api *API) ClearDownloads(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s - %s\n", r.Method, r.URL.String())
	api.Store.Clear()
	w.WriteHeader(http.StatusNoContent)
}

func (api *API) DownloadTorrent(w http.ResponseWriter, r *http.Request) {
	path := "/app/torrent"
	downloadFile(api, w, r, path)
}

func (api *API) DownloadPrivate(w http.ResponseWriter, r *http.Request) {
	path := "/app/private"
	downloadFile(api, w, r, path)
}

func (api *API) RetryDownload(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	err := service.RetryDownload(api.Store, id)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	w.Write([]byte("ok"))
}

func downloadFile(api *API, w http.ResponseWriter, r *http.Request, path string) {
	log.Printf("%s - %s\n", r.Method, r.URL.String())
	encodedUrl := r.URL.Query().Get("url")
	fileName := r.URL.Query().Get("fileName")

	if encodedUrl == "" {
		http.Error(w, "Missing url", http.StatusBadRequest)
		return
	}

	urlB64, err := url.QueryUnescape(encodedUrl)
	if err != nil {
		http.Error(w, "Invalid URL encoding", http.StatusBadRequest)
		return
	}
	log.Printf("B64 URL: %s", urlB64)
	urlBytes, err := base64.StdEncoding.DecodeString(urlB64)
	if err != nil {
		http.Error(w, "Invalid base64 URL", http.StatusBadRequest)
		return
	}
	url := string(urlBytes)
	log.Printf("File download initiated for: %s", url)

	id := service.StartDownload(api.Store, url, fileName, path)

	writeJSON(w, map[string]string{"id": id})
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
