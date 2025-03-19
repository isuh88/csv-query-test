package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	MAX_LIMIT = 1000 // maximum number of rows that can be returned in a single request
)

// QueryHandler handles CSV segment queries
type QueryHandler struct {
	s3Client *S3Client
}

type QueryResponse struct {
	Header []string   `json:"header"`
	Data   [][]string `json:"data"`
	Next   bool       `json:"next"`
}

func NewQueryHandler(s3Client *S3Client) *QueryHandler {
	return &QueryHandler{s3Client: s3Client}
}

func (h *QueryHandler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	timer := NewTimeCheck()
	defer timer.End()

	// Get key from URL path
	key := strings.TrimPrefix(r.URL.Path, "/")
	if key == "" {
		http.Error(w, "key parameter is required", http.StatusBadRequest)
		return
	}
	log.Printf("Querying with key: %s", key)

	offset, limit, err := getOffsetAndLimit(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Calculate which segment to read based on offset
	// SEGMENT_SIZE is imported from upload_handler.go (50000 rows per segment)
	segmentNum := offset / SEGMENT_SIZE
	offsetInSegment := offset % SEGMENT_SIZE
	log.Printf("Reading from segment %d at offset %d (total offset: %d)", segmentNum, offsetInSegment, offset)

	// Construct the segment file path
	segmentKey := fmt.Sprintf("%s/segment-%d.csv", key, segmentNum)
	log.Printf("Accessing segment file: %s", segmentKey)

	content, err := h.s3Client.GetCSVContent(segmentKey)
	if err != nil {
		if err.Error() == "file not found" {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer content.Close()

	// Create CSV reader
	csvReader := csv.NewReader(content)

	// Read header
	header, err := csvReader.Read()
	if err != nil {
		http.Error(w, "Failed to read header", http.StatusUnprocessableEntity)
		return
	}

	// Skip to offset within the segment
	for i := 0; i < offsetInSegment; i++ {
		_, err := csvReader.Read()
		if err == io.EOF {
			http.Error(w, "Offset exceeds file size", http.StatusBadRequest)
			return
		}
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusUnprocessableEntity)
			return
		}
	}

	// Read requested rows
	var data [][]string
	remainingRows := limit
	currentSegment := segmentNum

	for len(data) < limit {
		row, err := csvReader.Read()
		if err == io.EOF {
			// Try next segment
			currentSegment++
			content.Close()

			nextSegmentKey := fmt.Sprintf("%s/segment-%d.csv", key, currentSegment)
			content, err = h.s3Client.GetCSVContent(nextSegmentKey)
			if err != nil {
				// No more segments available
				break
			}

			csvReader = csv.NewReader(content)
			// Skip header of the next segment
			_, err = csvReader.Read()
			if err != nil {
				http.Error(w, "Failed to read next segment", http.StatusUnprocessableEntity)
				return
			}
			continue
		}
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusUnprocessableEntity)
			return
		}
		data = append(data, row)
		remainingRows--
		if remainingRows == 0 {
			break
		}
	}

	// Check if there are more rows
	hasMore := false
	if len(data) == limit {
		// Try to read one more row from current or next segment
		if currentSegment == segmentNum {
			_, err = csvReader.Read()
			hasMore = err == nil
		} else {
			hasMore = true
		}
	}

	// Prepare response
	response := QueryResponse{
		Header: header,
		Data:   data,
		Next:   hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func getOffsetAndLimit(r *http.Request) (offset, limit int, err error) {
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")

	// Handle offset
	offset = 0
	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return 0, 0, fmt.Errorf("invalid offset value")
		}
	}

	// Handle limit
	limit = 100 // default
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			return 0, 0, fmt.Errorf("invalid limit value")
		}
	}

	// Check max limit
	if limit > MAX_LIMIT {
		return 0, 0, fmt.Errorf("limit exceeds maximum allowed value of %d", MAX_LIMIT)
	}

	return offset, limit, nil
}
