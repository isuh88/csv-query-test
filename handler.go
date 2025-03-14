package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
)

type Handler struct {
	s3Client *S3Client
}

func NewHandler(s3Client *S3Client) *Handler {
	return &Handler{s3Client: s3Client}
}

func (h *Handler) handleCSV(w http.ResponseWriter, r *http.Request, key string) {
	timer := NewTimeCheck()
	defer timer.End()

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

func main() {
	s3Client, err := NewS3Client()
	if err != nil {
		log.Fatalf("Failed to create S3 client: %v", err)
	}

	handler := NewHandler(s3Client)

	http.HandleFunc("/csv/500k", func(w http.ResponseWriter, r *http.Request) {
		handler.handleCSV(w, r, "customers-500000.csv")
	})
	http.HandleFunc("/csv/1m", func(w http.ResponseWriter, r *http.Request) {
		handler.handleCSV(w, r, "customers-1000000.csv")
	})
	http.HandleFunc("/csv/2m", func(w http.ResponseWriter, r *http.Request) {
		handler.handleCSV(w, r, "customers-2000000.csv")
	})

	fmt.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
