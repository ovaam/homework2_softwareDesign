package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func main() {
	fmt.Println("API Gateway запущен на порту :8080")

	fileStoreURL, _ := url.Parse("http://localhost:8081")
	fileStoreProxy := httputil.NewSingleHostReverseProxy(fileStoreURL)

	analysisURL, _ := url.Parse("http://localhost:8082")
	analysisProxy := httputil.NewSingleHostReverseProxy(analysisURL)

	http.Handle("/storage/", http.StripPrefix("/storage", fileStoreProxy))
	http.Handle("/analysis/", http.StripPrefix("/analysis", analysisProxy))

	log.Fatal(http.ListenAndServe(":8080", nil))
}
