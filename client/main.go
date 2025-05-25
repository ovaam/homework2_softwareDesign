package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
)

type FileInfo struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
	Size         int64  `json:"size"`
}

type AnalysisResult struct {
	CharCount     int                `json:"char_count"`
	WordCount     int                `json:"word_count"`
	ParaCount     int                `json:"para_count"`
	UniqueWords   int                `json:"unique_words"`
	Fingerprint   string             `json:"fingerprint"`
	WordFrequency map[string]int     `json:"word_frequency"`
	Plagiarism    map[string]float64 `json:"plagiarism,omitempty"`
}

const (
	TestFilesDir = "test_files"
)

func main() {
	showWelcome()

	for {
		action, exit := selectMainAction()
		if exit {
			color.Green("Работа завершена")
			return
		}

		switch action {
		case "Анализ файла":
			if !handleFileAnalysis() {
				return
			}
		case "Проверка плагиата":
			if !handlePlagiarismCheck() {
				return
			}
		}
	}
}

func selectMainAction() (string, bool) {
	prompt := promptui.Select{
		Label: "Выберите действие (Ctrl+C для выхода)",
		Items: []string{"Анализ файла", "Проверка плагиата", "Выход"},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return "", true
	}

	actions := []string{"Анализ файла", "Проверка плагиата", "Выход"}
	return actions[idx], idx == 2
}

func handleFileAnalysis() bool {
	projectRoot := findProjectRoot()
	testFilesPath := filepath.Join(projectRoot, TestFilesDir)

	if err := os.MkdirAll(testFilesPath, 0755); err != nil {
		color.Red("Ошибка создания папки test_files: %v", err)
		return false
	}

	files, err := getFilesList(testFilesPath)
	if err != nil {
		color.Red("Ошибка при получении списка файлов: %v", err)
		return false
	}

	for {
		showAvailableFiles(files, testFilesPath)

		fileInput, exit := getFileInputFromUser(len(files))
		if exit {
			return true
		}

		selectedFile, err := resolveSelectedFile(fileInput, files, testFilesPath)
		if err != nil {
			color.Red(err.Error())
			continue
		}

		result, err := analyzeSelectedFile(selectedFile)
		if err != nil {
			color.Red(err.Error())
			continue
		}

		printAnalysisResult(result)
		return true
	}
}

func findProjectRoot() string {
	cwd, _ := os.Getwd()
	for i := 0; i < 5; i++ {
		if isProjectRootDir(cwd) {
			return cwd
		}
		cwd = filepath.Dir(cwd)
	}
	return "."
}

func isProjectRootDir(dir string) bool {
	requiredDirs := []string{"api_gateway", "file_store", "analysis", "client"}
	for _, d := range requiredDirs {
		if _, err := os.Stat(filepath.Join(dir, d)); err != nil {
			return false
		}
	}
	return true
}

func getFilesList(dirPath string) ([]os.DirEntry, error) {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("папка %s не существует", dirPath)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения папки: %v", err)
	}

	var files []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry)
		}
	}
	return files, nil
}

func showAvailableFiles(files []os.DirEntry, path string) {
	color.Cyan("\nДоступные файлы в %s:", path)
	if len(files) == 0 {
		color.Yellow("Папка не содержит файлов")
		color.Yellow("Поместите файлы в: %s", path)
		return
	}

	for i, file := range files {
		info, _ := file.Info()
		fmt.Printf("[%d] %s (%.2f KB)\n", i+1, file.Name(), float64(info.Size())/1024)
	}
}

func getFileInputFromUser(fileCount int) (string, bool) {
	prompt := promptui.Prompt{
		Label: fmt.Sprintf("Введите номер (1-%d) или имя файла (или 'назад')", fileCount),
	}

	input, err := prompt.Run()
	if err != nil || strings.ToLower(input) == "назад" {
		return "", true
	}

	return strings.TrimSpace(input), false
}

func resolveSelectedFile(input string, files []os.DirEntry, basePath string) (string, error) {
	if idx, err := strconv.Atoi(input); err == nil {
		if idx < 1 || idx > len(files) {
			return "", fmt.Errorf("неверный номер файла")
		}
		return filepath.Join(basePath, files[idx-1].Name()), nil
	}

	filePath := filepath.Join(basePath, input)
	if _, err := os.Stat(filePath); err != nil {
		return "", fmt.Errorf("файл '%s' не найден", input)
	}

	return filePath, nil
}

func analyzeSelectedFile(filePath string) (*AnalysisResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла: %v", err)
	}
	defer file.Close()

	// Читаем содержимое файла
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения файла: %v", err)
	}

	// Отправляем содержимое файла напрямую
	req, err := http.NewRequest("POST", "http://localhost:8080/analysis/analyze", bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ошибка сервера: %s", string(body))
	}

	var result AnalysisResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	return &result, nil
}

func handlePlagiarismCheck() bool {
	files, err := getStoredFilesList()
	if err != nil {
		color.Red("Ошибка получения списка файлов: %v", err)
		return false
	}

	if len(files) < 2 {
		color.Yellow("Для проверки нужно минимум 2 файла")
		return true
	}

	selectedFile, err := selectFileForCheck(files)
	if err != nil {
		return true
	}

	comparisons, err := getPlagiarismResults(selectedFile.ID)
	if err != nil {
		color.Red("Ошибка проверки плагиата: %v", err)
		return false
	}

	displayPlagiarismResults(comparisons, files)
	return true
}

func selectFileForCheck(files []FileInfo) (*FileInfo, error) {
	options := make([]string, len(files))
	for i, file := range files {
		options[i] = fmt.Sprintf("%s (ID: %s)", file.OriginalName, file.ID[:8])
	}

	prompt := promptui.Select{
		Label: "Выберите файл для проверки на плагиат",
		Items: options,
		Size:  10, // Показывать до 10 файлов одновременно
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	return &files[idx], nil
}

func getStoredFilesList() ([]FileInfo, error) {
	resp, err := http.Get("http://localhost:8080/storage/list")

	if err != nil {
		return nil, fmt.Errorf("ошибка соединения: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка сервера (код %d)", resp.StatusCode)
	}

	var files []FileInfo
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("ошибка чтения списка файлов: %v", err)
	}

	return files, nil
}

func getPlagiarismResults(fileID string) (map[string]float64, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:8080/analysis/compare/%s", fileID))

	if err != nil {
		return nil, fmt.Errorf("ошибка проверки плагиата: %v", err)
	}
	defer resp.Body.Close()

	var comparisons map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&comparisons); err != nil {
		return nil, fmt.Errorf("ошибка чтения результатов: %v", err)
	}

	return comparisons, nil
}

func findOriginalName(fileID string, files []FileInfo) string {
	for _, f := range files {
		if f.ID == fileID {
			return f.OriginalName
		}
	}
	return fileID[:8] + "..."
}

func printAnalysisResult(result *AnalysisResult) {
	color.Cyan("\n┌─────────────────────────────┐")
	color.Cyan("│        Результаты анализа    │")
	color.Cyan("├─────────────────────────────┤")
	color.Blue("│ ID документа: %-14s │", result.Fingerprint[:8])
	color.Blue("│ Символов: %-18d │", result.CharCount)
	color.Blue("│ Слов: %-21d │", result.WordCount)
	color.Blue("│ Абзацев: %-19d │", result.ParaCount)
	color.Blue("│ Уникальных слов: %-12d │", result.UniqueWords)
	color.Cyan("└─────────────────────────────┘\n")
}

func displayPlagiarismResults(comparisons map[string]float64, files []FileInfo) {
	found := false

	color.Cyan("\nРезультаты проверки на плагиат:")
	for fileID, percent := range comparisons {
		if percent == 100 {
			name := findOriginalName(fileID, files)
			color.Red("• Полный плагиат с файлом: %s", name)
			found = true
		}
	}

	if !found {
		color.Green("Плагиат не обнаружен")
	}
	fmt.Println()
}

func showWelcome() {
	color.Cyan(`
┌──────────────────────────────────────┐
│                                      │
│   ████████╗███████╗██╗  ██╗████████╗│
│   ╚══██╔══╝██╔════╝╚██╗██╔╝╚══██╔══╝│
│      ██║   █████╗   ╚███╔╝    ██║   │
│      ██║   ██╔══╝   ██╔██╗    ██║   │
│      ██║   ███████╗██╔╝ ██╗   ██║   │
│      ╚═╝   ╚══════╝╚═╝  ╚═╝   ╚═╝   │
│                                      │
└──────────────────────────────────────┘`)
}
