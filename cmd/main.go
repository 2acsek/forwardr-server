package main

import (
	"log"
	"net/http"

	"github.com/2acsek/forwardr-server/internal/handler"
	"github.com/2acsek/forwardr-server/internal/model"
)

func main() {
	store := model.NewStore()
	api := handler.NewHandler(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", api.Health)
	mux.HandleFunc("/downloads", api.Downloads)
	mux.HandleFunc("/downloads/clear", api.ClearDownloads)
	mux.HandleFunc("/download", api.DownloadTorrent)
	mux.HandleFunc("/download/private", api.DownloadPrivate)
	mux.HandleFunc("/download/retry", api.RetryDownload)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("Starting server on :8080")
	log.Fatal(server.ListenAndServe())
}
