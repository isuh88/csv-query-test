package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

const (
	SEGMENT_SIZE  = 50000             // rows per segment (increased from 10,000)
	MAX_FILE_SIZE = 100 * 1024 * 1024 // 100MB in bytes

	UploadModeFineGrained   = "fine"
	UploadModeCoarseGrained = "coarse"
	UploadModeBatch         = "batch"
	UploadModeStream        = "stream"
)

type UploadHandler struct {
	s3Client *S3Client
}

type UploadResponse struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Ext         string `json:"ext"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Chunks      int    `json:"chunks"`
}

type UploadConfig struct {
	SegmentSize int
	UploadMode  string
	Workers     int // Number of concurrent workers for streaming mode
}

type SegmentStats struct {
	SegmentSize    int           // 세그먼트의 행 수
	UploadDuration time.Duration // 업로드에 걸린 시간
	DataSize       int           // 세그먼트 데이터 크기 (bytes)
}

func NewUploadHandler(s3Client *S3Client) *UploadHandler {
	return &UploadHandler{s3Client: s3Client}
}

func (h *UploadHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	// 기본 설정으로 HandleUploadWithConfig 호출
	h.HandleUploadWithConfig(w, r, UploadConfig{
		SegmentSize: SEGMENT_SIZE,
		UploadMode:  UploadModeFineGrained,
	})
}

func (h *UploadHandler) HandleUploadWithConfig(w http.ResponseWriter, r *http.Request, config UploadConfig) {
	timer := NewTimeCheck()
	defer timer.End()

	// Check content length
	if r.ContentLength > MAX_FILE_SIZE {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Get filename from URL
	fileName, ok := r.Context().Value(fileNameKey).(string)
	if !ok || fileName == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	// Validate file type
	ext := filepath.Ext(fileName)
	if ext != ".csv" && ext != ".tsv" {
		http.Error(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	// Generate storage path
	channelID, ok := r.Context().Value(channelIDKey).(string)
	if !ok || channelID == "" {
		http.Error(w, "Channel ID is required", http.StatusBadRequest)
		return
	}

	timestamp := time.Now().Format("2006-01-02-15-04-05")
	basePath := fmt.Sprintf("csv_upload/%s/%s", channelID, timestamp)

	// Validate upload path
	if err := h.s3Client.ValidateUploadKey(basePath); err != nil {
		http.Error(w, "Invalid upload path", http.StatusBadRequest)
		return
	}

	// Process file in segments
	reader := csv.NewReader(r.Body)
	csvHeader, err := reader.Read()
	if err != nil {
		http.Error(w, "Failed to read header", http.StatusUnprocessableEntity)
		return
	}

	// Set the number of expected fields per record -> 테스트 필요
	// reader.FieldsPerRecord = -1

	var segmentCount int
	if config.UploadMode == UploadModeStream {
		err = h.handleStreamUpload(basePath, csvHeader, reader, config)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to stream upload: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		var segments [][][]string
		var currentSegment [][]string
		segmentCount = 0

		// 데이터 읽기 및 세그먼트 구성
		for {
			row, err := reader.Read()
			if err == io.EOF {
				if len(currentSegment) > 0 {
					segments = append(segments, currentSegment)
					if config.UploadMode != UploadModeBatch {
						if err := h.storeSegment(basePath, segmentCount, csvHeader, currentSegment); err != nil {
							http.Error(w, fmt.Sprintf("Failed to upload segment %d: %v", segmentCount, err), http.StatusInternalServerError)
							return
						}
					}
					segmentCount++
				}
				break
			}
			if err != nil {
				http.Error(w, "Failed to read file", http.StatusUnprocessableEntity)
				return
			}

			currentSegment = append(currentSegment, row)
			if len(currentSegment) == config.SegmentSize {
				segments = append(segments, currentSegment)

				// fine/coarse-grained 모드에서는 즉시 업로드
				if config.UploadMode != UploadModeBatch {
					if err := h.storeSegment(basePath, segmentCount, csvHeader, currentSegment); err != nil {
						http.Error(w, fmt.Sprintf("Failed to upload segment %d: %v", segmentCount, err), http.StatusInternalServerError)
						return
					}
				}

				segmentCount++
				currentSegment = make([][]string, 0, config.SegmentSize)
			}
		}

		// 배치 업로드 처리
		if config.UploadMode == UploadModeBatch && len(segments) > 0 {
			log.Printf("Starting batch upload preparation for %d segments...", len(segments))
			var uploadTargets []S3UploadDTO

			for i, segment := range segments {
				log.Printf("Preparing segment %d of %d (size: %d rows)...", i+1, len(segments), len(segment))
				var buf bytes.Buffer
				writer := csv.NewWriter(&buf)

				if err := writer.Write(csvHeader); err != nil {
					http.Error(w, fmt.Sprintf("Failed to write header for segment %d: %v", i, err), http.StatusInternalServerError)
					return
				}

				for _, row := range segment {
					if err := writer.Write(row); err != nil {
						http.Error(w, fmt.Sprintf("Failed to write row in segment %d: %v", i, err), http.StatusInternalServerError)
						return
					}
				}
				writer.Flush()

				key := fmt.Sprintf("%s/segment-%d.csv", basePath, i)
				uploadTargets = append(uploadTargets, S3UploadDTO{
					Key:     key,
					Content: buf.Bytes(),
				})
			}

			log.Printf("Starting batch upload of %d segments to S3...", len(segments))
			start := time.Now()
			if err := h.s3Client.BatchUpload(uploadTargets); err != nil {
				http.Error(w, fmt.Sprintf("Failed to batch upload segments: %v", err), http.StatusInternalServerError)
				return
			}
			duration := time.Since(start)
			log.Printf("Batch upload completed successfully. Total time: %v", duration)
		}
	}

	// Create response
	response := UploadResponse{
		Bucket:      bucketName,
		Key:         basePath,
		ID:          fmt.Sprintf("csv_%s", timestamp),
		Type:        "text/" + ext[1:],
		Name:        fileName,
		Ext:         ext[1:],
		Size:        r.ContentLength,
		ContentType: ext[1:],
		Chunks:      segmentCount,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *UploadHandler) storeSegment(basePath string, segmentNum int, header []string, rows [][]string) error {
	start := time.Now()
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	// Write rows
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %v", err)
		}
	}
	writer.Flush()

	// Upload to S3
	key := fmt.Sprintf("%s/segment-%d.csv", basePath, segmentNum)
	err := h.s3Client.UploadSegment(key, buf.Bytes())

	// Log performance metrics
	duration := time.Since(start)
	dataSize := buf.Len()
	uploadSpeed := float64(dataSize) / duration.Seconds() / 1024 / 1024 // MB/s

	log.Printf("Segment %d stats: Size=%d rows, Data=%d bytes, Duration=%v, Speed=%.2f MB/s",
		segmentNum, len(rows), dataSize, duration, uploadSpeed)

	return err
}

// handleStreamUpload processes and uploads segments concurrently using goroutines
func (h *UploadHandler) handleStreamUpload(basePath string, header []string, reader *csv.Reader, config UploadConfig) error {
	type SegmentJob struct {
		number int
		rows   [][]string
	}

	numWorkers := 4 // default value
	if config.Workers > 0 {
		numWorkers = config.Workers
	}

	// 작업 채널 생성
	jobs := make(chan SegmentJob, numWorkers)        // 작업 큐
	results := make(chan error, numWorkers)          // 결과 채널
	done := make(chan bool)                          // 작업 완료 신호
	activeWorkers := make(chan struct{}, numWorkers) // 활성 워커 수 추적
	expectedSegments := 0                            // 예상되는 총 세그먼트 수

	log.Printf("Starting streaming upload with %d workers", numWorkers)

	// 워커 풀 생성
	for i := 0; i < numWorkers; i++ {
		go func(workerId int) {
			activeWorkers <- struct{}{} // 워커 활성화
			defer func() {
				<-activeWorkers // 워커 비활성화
			}()

			for job := range jobs {
				log.Printf("Worker %d/%d processing segment %d (%d rows)",
					workerId+1, numWorkers, job.number, len(job.rows))

				err := h.streamSegment(basePath, job.number, header, job.rows)
				results <- err
			}
		}(i)
	}

	// 결과 모니터링 goroutine
	go func() {
		segmentCount := 0
		for err := range results {
			if err != nil {
				log.Printf("Error uploading segment: %v", err)
				done <- false
				return
			}
			segmentCount++
			if segmentCount%10 == 0 {
				log.Printf("Successfully uploaded %d/%d segments", segmentCount, expectedSegments)
			}
			if segmentCount == expectedSegments {
				log.Printf("All %d segments uploaded successfully", expectedSegments)
				done <- true
				return
			}
		}
	}()

	// CSV 파일 읽기 및 작업 할당
	var currentSegment [][]string
	segmentNum := 0

	for {
		row, err := reader.Read()
		if err == io.EOF {
			if len(currentSegment) > 0 {
				jobs <- SegmentJob{number: segmentNum, rows: currentSegment}
				expectedSegments = segmentNum + 1
			} else {
				expectedSegments = segmentNum
			}
			break
		}
		if err != nil {
			close(jobs)
			return fmt.Errorf("failed to read file: %v", err)
		}

		currentSegment = append(currentSegment, row)
		if len(currentSegment) == config.SegmentSize {
			jobs <- SegmentJob{number: segmentNum, rows: currentSegment}
			segmentNum++
			currentSegment = make([][]string, 0, config.SegmentSize)
		}
	}

	// 모든 작업이 큐에 들어갔음을 표시
	close(jobs)

	// 작업 완료 대기
	if success := <-done; !success {
		return fmt.Errorf("one or more segments failed to upload")
	}

	return nil
}

// streamSegment uploads a single segment to S3
func (h *UploadHandler) streamSegment(basePath string, segmentNum int, header []string, rows [][]string) error {
	start := time.Now()
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	// Write rows
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %v", err)
		}
	}
	writer.Flush()

	// Upload to S3
	key := fmt.Sprintf("%s/segment-%d.csv", basePath, segmentNum)
	err := h.s3Client.UploadSegment(key, buf.Bytes())

	// Log performance metrics
	duration := time.Since(start)
	dataSize := buf.Len()
	uploadSpeed := float64(dataSize) / duration.Seconds() / 1024 / 1024 // MB/s

	log.Printf("Segment %d streaming stats: Size=%d rows, Data=%d bytes, Duration=%v, Speed=%.2f MB/s",
		segmentNum, len(rows), dataSize, duration, uploadSpeed)

	return err
}
