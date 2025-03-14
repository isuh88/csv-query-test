package main

import (
	"bufio"
	"io"
	"net/http"
	"strconv"
)

type Handler struct {
	s3Client *S3Client
}

func NewHandler(s3Client *S3Client) *Handler {
	return &Handler{s3Client: s3Client}
}

func (h *Handler) handleCSV(w http.ResponseWriter, r *http.Request) {
	timer := NewTimeCheck()
	defer timer.End()

	key := r.URL.Query().Get("file")
	if key == "" {
		http.Error(w, "file parameter is required", http.StatusBadRequest)
		return
	}

	offset, limit := getOffsetAndLimit(r)

	content, err := h.s3Client.GetCSVContent(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer content.Close()

	reader := bufio.NewReader(content)

	// Read and write header
	header, err := reader.ReadString('\n')
	if err != nil {
		http.Error(w, "Failed to read header", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Write([]byte(header))

	// Skip lines until offset
	for i := 0; i < offset; i++ {
		_, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			http.Error(w, "Failed to read line", http.StatusInternalServerError)
			return
		}
	}

	// Read specified number of lines
	for i := 0; i < limit; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			http.Error(w, "Failed to read line", http.StatusInternalServerError)
			return
		}
		w.Write([]byte(line))
	}
}

func getOffsetAndLimit(r *http.Request) (offset, limit int) {
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")

	offset = 0
	if offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil && val >= 0 {
			offset = val
		}
	}

	limit = 100 // default limit
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
		}
	}

	return offset, limit
}
