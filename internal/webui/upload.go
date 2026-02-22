package webui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type UploadResponse struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	URL      string `json:"url"`
	Path     string `json:"path"`
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (32MB max memory)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create uploads directory
	uploadsDir := filepath.Join(s.cfg.Agents.Defaults.Workspace, ".uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		http.Error(w, "Failed to create uploads directory", http.StatusInternalServerError)
		return
	}

	// Generate unique filename
	id := uuid.New().String()[:8]
	ext := filepath.Ext(header.Filename)
	safeName := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102"), id, ext)
	targetPath := filepath.Join(uploadsDir, safeName)

	// Security check: ensure file is within uploads directory
	if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(uploadsDir)) {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	// Save file
	dst, err := os.Create(targetPath)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	size, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(targetPath)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	response := UploadResponse{
		ID:       id,
		Filename: header.Filename,
		Size:     size,
		URL:      fmt.Sprintf("/api/uploads/%s", safeName),
		Path:     targetPath,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleGetUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	filename := filepath.Base(r.URL.Path)
	uploadsDir := filepath.Join(s.cfg.Agents.Defaults.Workspace, ".uploads")
	filePath := filepath.Join(uploadsDir, filename)

	// Security check: ensure file is within uploads directory
	if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(uploadsDir)) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, filePath)
}
