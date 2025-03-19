package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type contextKey string

const (
	channelIDKey contextKey = "channelId"
	fileNameKey  contextKey = "fileName"
)

// extractPathParams extracts channelId and fileName from the URL path
func extractPathParams(prefix, path string) (channelId, fileName string, ok bool) {
	// Remove prefix from path
	trimmedPath := strings.TrimPrefix(path, prefix)
	if trimmedPath == path {
		return "", "", false
	}

	// Split remaining path
	parts := strings.Split(strings.Trim(trimmedPath, "/"), "/")
	if len(parts) != 2 {
		return "", "", false
	}

	return parts[0], parts[1], true
}

func main() {
	s3Client, err := NewS3Client()
	if err != nil {
		log.Fatalf("Failed to create S3 client: %v", err)
	}

	mux := http.NewServeMux()

	// Upload handlers
	uploadHandler := NewUploadHandler(s3Client)

	// Default upload endpoint (fine-grained)
	mux.HandleFunc("/cht/v1/secure-file/csv/", func(w http.ResponseWriter, r *http.Request) {
		channelId, fileName, ok := extractPathParams("/cht/v1/secure-file/csv/", r.URL.Path)
		if !ok {
			http.Error(w, "Invalid path format. Expected: /cht/v1/secure-file/csv/{channelId}/{fileName}", http.StatusBadRequest)
			return
		}
		ctx := context.WithValue(r.Context(), channelIDKey, channelId)
		ctx = context.WithValue(ctx, fileNameKey, fileName)
		uploadHandler.HandleUpload(w, r.WithContext(ctx))
	})

	// Test endpoints for different upload modes
	mux.HandleFunc("/test/fine-grained/csv/", func(w http.ResponseWriter, r *http.Request) {
		channelId, fileName, ok := extractPathParams("/test/fine-grained/csv/", r.URL.Path)
		if !ok {
			http.Error(w, "Invalid path format. Expected: /test/fine-grained/csv/{channelId}/{fileName}", http.StatusBadRequest)
			return
		}
		ctx := context.WithValue(r.Context(), channelIDKey, channelId)
		ctx = context.WithValue(ctx, fileNameKey, fileName)
		uploadHandler.HandleUploadWithConfig(w, r.WithContext(ctx), UploadConfig{
			SegmentSize: 1000,
			UploadMode:  UploadModeFineGrained,
		})
	})

	mux.HandleFunc("/test/coarse-grained/csv/", func(w http.ResponseWriter, r *http.Request) {
		channelId, fileName, ok := extractPathParams("/test/coarse-grained/csv/", r.URL.Path)
		if !ok {
			http.Error(w, "Invalid path format. Expected: /test/coarse-grained/csv/{channelId}/{fileName}", http.StatusBadRequest)
			return
		}
		ctx := context.WithValue(r.Context(), channelIDKey, channelId)
		ctx = context.WithValue(ctx, fileNameKey, fileName)
		uploadHandler.HandleUploadWithConfig(w, r.WithContext(ctx), UploadConfig{
			SegmentSize: 10000,
			UploadMode:  UploadModeCoarseGrained,
		})
	})

	mux.HandleFunc("/test/batch-upload/csv/", func(w http.ResponseWriter, r *http.Request) {
		channelId, fileName, ok := extractPathParams("/test/batch-upload/csv/", r.URL.Path)
		if !ok {
			http.Error(w, "Invalid path format. Expected: /test/batch-upload/csv/{channelId}/{fileName}", http.StatusBadRequest)
			return
		}
		ctx := context.WithValue(r.Context(), channelIDKey, channelId)
		ctx = context.WithValue(ctx, fileNameKey, fileName)
		uploadHandler.HandleUploadWithConfig(w, r.WithContext(ctx), UploadConfig{
			SegmentSize: 1000,
			UploadMode:  UploadModeBatch,
		})
	})

	// New streaming upload endpoint with goroutines
	mux.HandleFunc("/test/stream-upload/csv/", func(w http.ResponseWriter, r *http.Request) {
		channelId, fileName, ok := extractPathParams("/test/stream-upload/csv/", r.URL.Path)
		if !ok {
			http.Error(w, "Invalid path format. Expected: /test/stream-upload/csv/{channelId}/{fileName}", http.StatusBadRequest)
			return
		}

		// Get number of workers from query parameter
		workers := 4 // default value
		if workersStr := r.URL.Query().Get("workers"); workersStr != "" {
			if worker, err := strconv.Atoi(workersStr); err == nil && worker > 0 {
				workers = worker
			} else {
				http.Error(w, "Invalid workers parameter. Must be a positive integer", http.StatusBadRequest)
				return
			}
		}

		ctx := context.WithValue(r.Context(), channelIDKey, channelId)
		ctx = context.WithValue(ctx, fileNameKey, fileName)
		uploadHandler.HandleUploadWithConfig(w, r.WithContext(ctx), UploadConfig{
			SegmentSize: 1000,
			UploadMode:  UploadModeStream,
			Workers:     workers, // Pass workers to config
		})
	})

	// Query handler
	queryHandler := NewQueryHandler(s3Client)
	mux.HandleFunc("/admin/cht/v1/secure-file/csv-upload/", func(w http.ResponseWriter, r *http.Request) {
		prefix := "/admin/cht/v1/secure-file/csv-upload/"
		if key := strings.TrimPrefix(r.URL.Path, prefix); key != "" {
			r = r.WithContext(r.Context())
			r.URL.Path = "/" + key
			queryHandler.HandleQuery(w, r)
			return
		}
		http.NotFound(w, r)
	})

	fmt.Println("Server starting on :8080...")
	fmt.Println("\nAvailable endpoints:")
	fmt.Println("1. Default Upload (fine-grained):")
	fmt.Println("   POST /cht/v1/secure-file/csv/{channelId}/{fileName}")
	fmt.Println("\n2. Test Endpoints:")
	fmt.Println("   a) Fine-grained upload (1,000 rows/segment):")
	fmt.Println("      POST /test/fine-grained/csv/{channelId}/{fileName}")
	fmt.Println("   b) Coarse-grained upload (10,000 rows/segment):")
	fmt.Println("      POST /test/coarse-grained/csv/{channelId}/{fileName}")
	fmt.Println("   c) Batch upload (1,000 rows/segment, uploaded in batch):")
	fmt.Println("      POST /test/batch-upload/csv/{channelId}/{fileName}")
	fmt.Println("   d) Stream upload (1,000 rows/segment, concurrent streaming):")
	fmt.Println("      POST /test/stream-upload/csv/{channelId}/{fileName}")
	fmt.Println("\n3. Query CSV segments:")
	fmt.Println("   GET /admin/cht/v1/secure-file/csv-upload/csv/{channelId}/{timestamp}")
	fmt.Println("   Example: /admin/cht/v1/secure-file/csv-upload/csv/1/2025-03-19-10-45-09")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
