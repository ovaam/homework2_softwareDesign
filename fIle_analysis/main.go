package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"unicode"
)

type AnalysisResult struct {
	CharCount     int            `json:"char_count"`
	WordCount     int            `json:"word_count"`
	ParaCount     int            `json:"para_count"`
	UniqueWords   int            `json:"unique_words"`
	Fingerprint   string         `json:"fingerprint"`
	WordFrequency map[string]int `json:"word_frequency"`
}

var (
	fileStats  = make(map[string]AnalysisResult)
	statsMutex sync.RWMutex
)

func main() {
	fmt.Println("File Analysis Service запущен на порту :8082")

	http.HandleFunc("/analyze", analyzeHandler)
	http.HandleFunc("/compare/", compareHandler)
	http.HandleFunc("/health", healthHandler)

	log.Fatal(http.ListenAndServe(":8082", nil))
}

func analyzeHandler(w http.ResponseWriter, r *http.Request) {
	content, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading content", http.StatusBadRequest)
		return
	}

	stats := analyzeText(string(content))
	fingerprint := generateFingerprint(content)
	stats.Fingerprint = fingerprint // Добавляем fingerprint в результат

	statsMutex.Lock()
	fileStats[fingerprint] = stats
	statsMutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func analyzeText(content string) AnalysisResult {
	stats := AnalysisResult{
		WordFrequency: make(map[string]int),
	}

	// Подсчет абзацев
	stats.ParaCount = strings.Count(content, "\n\n") + 1
	if len(strings.TrimSpace(content)) == 0 {
		stats.ParaCount = 0
	}

	// Подсчет символов (включая пробелы)
	stats.CharCount = len([]rune(content))

	// Подсчет слов
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Split(bufio.ScanWords)
	wordSet := make(map[string]int)

	for scanner.Scan() {
		word := strings.ToLower(scanner.Text())
		word = strings.TrimFunc(word, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
		if len(word) > 0 {
			stats.WordCount++
			wordSet[word]++
		}
	}

	stats.UniqueWords = len(wordSet)
	stats.WordFrequency = wordSet

	hasher := sha256.New()
	hasher.Write([]byte(content))
	stats.Fingerprint = hex.EncodeToString(hasher.Sum(nil))

	return stats
}

func compareHandler(w http.ResponseWriter, r *http.Request) {
	fileID := strings.TrimPrefix(r.URL.Path, "/compare/")
	if fileID == "" {
		http.Error(w, "File ID required", http.StatusBadRequest)
		return
	}

	statsMutex.RLock()
	defer statsMutex.RUnlock()

	currentStats, exists := fileStats[fileID]
	if !exists {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	results := make(map[string]float64)

	for id, stats := range fileStats {
		if id != fileID {
			similarity := compareStats(currentStats, stats)
			if similarity == 100 {
				results[id] = similarity
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func compareStats(a, b AnalysisResult) float64 {
	commonWords := 0
	totalWords := a.WordCount + b.WordCount

	for word, countA := range a.WordFrequency {
		if countB, exists := b.WordFrequency[word]; exists {
			commonWords += min(countA, countB)
		}
	}

	if totalWords == 0 {
		return 0
	}

	return float64(2*commonWords) / float64(totalWords) * 100
}

func generateFingerprint(content []byte) string {
	hasher := sha256.New()
	hasher.Write(content)
	return hex.EncodeToString(hasher.Sum(nil))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
