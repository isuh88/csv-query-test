package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// OptimizedHandler implements both caching and optimized line skipping
type OptimizedHandler struct {
	s3Client *CachedS3Client
}

// CachedHandler implements only caching
type CachedHandler struct {
	s3Client *CachedS3Client
}

// FastSkipHandler implements only optimized line skipping
type FastSkipHandler struct {
	s3Client *S3Client
}

func NewOptimizedHandler(baseClient *CachedS3Client) *OptimizedHandler {
	return &OptimizedHandler{s3Client: baseClient}
}

func NewCachedHandler(baseClient *CachedS3Client) *CachedHandler {
	return &CachedHandler{s3Client: baseClient}
}

func NewFastSkipHandler(baseClient *S3Client) *FastSkipHandler {
	return &FastSkipHandler{s3Client: baseClient}
}

// optimizedSkipLines implements the optimized line skipping logic
func optimizedSkipLines(reader *bufio.Reader, content io.ReadSeeker, offset int) error {
	if offset == 0 {
		return nil
	}

	skipped := 0
	buf := make([]byte, 4096)
	for skipped < offset {
		_, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return err
			}
			return fmt.Errorf("failed to read line: %v", err)
		}
		skipped++

		// If we're reading a large chunk that won't be used,
		// try to skip ahead using the buffer
		if skipped < offset-100 { // threshold before switching to buffer reading
			n, err := reader.Read(buf)
			if err != nil && err != io.EOF {
				return fmt.Errorf("failed to skip lines: %v", err)
			}
			newLines := bytes.Count(buf[:n], []byte{'\n'})
			skipped += newLines

			// If we overshot, reset and go back to line-by-line
			if skipped > offset {
				if _, err := content.Seek(0, io.SeekStart); err != nil {
					return fmt.Errorf("failed to seek: %v", err)
				}
				reader.Reset(content)
				// Skip header
				if _, err := reader.ReadString('\n'); err != nil {
					return fmt.Errorf("failed to skip header: %v", err)
				}
				// Skip to just before where we need to be
				for i := 0; i < offset; i++ {
					if _, err := reader.ReadString('\n'); err != nil {
						return fmt.Errorf("failed to skip line: %v", err)
					}
				}
				skipped = offset
			}
		}
	}
	return nil
}

// optimizedSkipLinesNoSeek implements optimized line skipping for non-seekable readers
func optimizedSkipLinesNoSeek(reader *bufio.Reader, offset int) error {
	if offset == 0 {
		return nil
	}

	skipped := 0
	buf := make([]byte, 4096)
	for skipped < offset {
		_, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return err
			}
			return fmt.Errorf("failed to read line: %v", err)
		}
		skipped++

		// If we're reading a large chunk that won't be used,
		// try to skip ahead using the buffer
		if skipped < offset-100 { // threshold before switching to buffer reading
			n, err := reader.Read(buf)
			if err != nil && err != io.EOF {
				return fmt.Errorf("failed to skip lines: %v", err)
			}
			newLines := bytes.Count(buf[:n], []byte{'\n'})
			skipped += newLines
		}
	}
	return nil
}

func (h *OptimizedHandler) handleCSV(w http.ResponseWriter, r *http.Request) {
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

	reader := bufio.NewReader(content)

	// Read and write header
	header, err := reader.ReadString('\n')
	if err != nil {
		http.Error(w, "Failed to read header", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Write([]byte(header))

	// Skip lines using optimized method
	if err := optimizedSkipLines(reader, content, offset); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Read requested lines
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

func (h *CachedHandler) handleCSV(w http.ResponseWriter, r *http.Request) {
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

	reader := bufio.NewReader(content)

	// Read and write header
	header, err := reader.ReadString('\n')
	if err != nil {
		http.Error(w, "Failed to read header", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Write([]byte(header))

	// Skip lines using simple method
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

	// Read requested lines
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

func (h *FastSkipHandler) handleCSV(w http.ResponseWriter, r *http.Request) {
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

	// Skip lines using optimized method for non-seekable readers
	if err := optimizedSkipLinesNoSeek(reader, offset); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Read requested lines
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

// UpdateMainToIncludeOptimized adds the optimized endpoint to the main server
func UpdateMainToIncludeOptimized(s3Client *s3.Client) {
	cachedClient := NewCachedS3Client(s3Client)
	optimizedHandler := NewOptimizedHandler(cachedClient)

	// Add the new optimized endpoint
	http.HandleFunc("/csv/optimized", optimizedHandler.handleCSV)
}
