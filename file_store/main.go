package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type FileInfo struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
	Size         int64  `json:"size"`
}

var (
	storageDir = "./storage"
	fileMutex  sync.RWMutex
)

func main() {
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		fmt.Printf("Failed to create storage directory: %v\n", err)
		return
	}

	fmt.Println("File Storage Service running on :8081")

	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/files/", downloadHandler)
	http.HandleFunc("/list", listHandler)

	if err := http.ListenAndServe(":8081", nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		http.Error(w, "Hashing error", http.StatusInternalServerError)
		return
	}
	fileID := hex.EncodeToString(hasher.Sum(nil))

	if _, err := file.Seek(0, 0); err != nil {
		http.Error(w, "File read error", http.StatusInternalServerError)
		return
	}

	fileMutex.Lock()
	defer fileMutex.Unlock()

	filePath := filepath.Join(storageDir, fileID)
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "File creation error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath)
		http.Error(w, "File save error", http.StatusInternalServerError)
		return
	}

	info := FileInfo{
		ID:           fileID,
		OriginalName: header.Filename,
		Size:         header.Size,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Path[len("/files/"):]
	if fileID == "" {
		http.Error(w, "File ID required", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(storageDir, fileID)

	fileMutex.RLock()
	defer fileMutex.RUnlock()

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, filePath)
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	fileMutex.RLock()
	defer fileMutex.RUnlock()

	entries, err := os.ReadDir(storageDir)
	if err != nil {
		http.Error(w, "Storage error", http.StatusInternalServerError)
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			ID:           entry.Name(),
			OriginalName: entry.Name(),
			Size:         info.Size(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}
