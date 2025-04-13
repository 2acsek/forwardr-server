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
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (api *API) Downloads(w http.ResponseWriter, r *http.Request) {
	all := api.Store.GetAll()
	writeJSON(w, all)
}

func (api *API) DownloadTorrent(w http.ResponseWriter, r *http.Request) {
	encodedUrl := r.URL.Query().Get("url")
	fileName := r.URL.Query().Get("fileName")
	path := "/app/torrent"

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

func (api *API) DownloadPrivate(w http.ResponseWriter, r *http.Request) {
	encodedUrl := r.URL.Query().Get("url")
	fileName := r.URL.Query().Get("fileName")
	path := "/app/private"

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
