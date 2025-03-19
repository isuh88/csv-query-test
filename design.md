# CSV Query Service Technical Design

## Overview
This document outlines the internal technical design and implementation details for the CSV Query Service. The service provides transparent file segmentation for large CSV/TSV files while maintaining a simple external API.

## Core Constants
```go
const (
    MAX_FILE_SIZE = 100 * 1024 * 1024  // 100MB in bytes
    SEGMENT_SIZE  = 1000               // rows per segment (internal)
)
```

## Internal Storage Structure

### S3 Directory Layout
```
{bucket}/csv/{channelId}/{timestamp}/
  ├── segment-0.{csv|tsv} # First segment with header
  ├── segment-1.{csv|tsv} # Subsequent segments with header
  └── ...
```

## Implementation Details

### File Upload Process
1. **Initial Validation**
   ```go
   func validateUpload(file io.Reader, filename string) error {
       // Check file size
       if fileSize > MAX_FILE_SIZE {
           return ErrFileTooLarge
       }
       
       // Validate file type and extension
       if !isValidFileType(filename) {
           return ErrInvalidFileType
       }
       
       // Validate CSV/TSV format and header
       if err := validateFormat(file); err != nil {
           return ErrInvalidFormat
       }
   }
   ```

2. **File Processing**
   ```go
   func processFile(file io.Reader, channelId string, filename string) (*UploadResult, error) {
       // Read and validate header
       header, err := readHeader(file)
       if err != nil {
           return nil, err
       }
       
       // Generate storage path
       timestamp := time.Now().Format("2006-01-02-15-04-05")
       basePath := fmt.Sprintf("csv/%s/%s", channelId, timestamp)
       
       // Process file in segments
       segmentCount := 0
       
       for {
           rows := readNextNRows(file, SEGMENT_SIZE)
           if len(rows) == 0 {
               break
           }
           
           // Store segment with header
           if err := storeSegment(basePath, segmentCount, header, rows); err != nil {
               return nil, err
           }
           
           segmentCount++
       }
       
       return createUploadResponse(basePath, filename, segmentCount), nil
   }
   ```

### Query Processing
```go
func retrieveRows(key string, offset, limit int) (*QueryResult, error) {
    if limit > 1000 {
        return nil, ErrInvalidLimit
    }
    
    // Calculate required segments
    startSeg := offset / SEGMENT_SIZE
    endSeg := (offset + limit - 1) / SEGMENT_SIZE
    
    // Read first segment to get header
    firstSegment, err := loadSegment(key, startSeg)
    if err != nil {
        return nil, err
    }
    
    result := &QueryResult{
        Header: firstSegment.Header,
        Data:   make([][]string, 0, limit),
    }
    
    remainingRows := limit
    currentOffset := offset % SEGMENT_SIZE
    
    // Process segments
    for seg := startSeg; seg <= endSeg; seg++ {
        if seg != startSeg {
            segment, err := loadSegment(key, seg)
            if err != nil {
                return nil, err
            }
            firstSegment = segment
        }
        
        // Calculate row range for this segment
        start := currentOffset
        count := min(remainingRows, len(firstSegment.Data)-start)
        
        result.Data = append(result.Data, firstSegment.Data[start:start+count]...)
        
        remainingRows -= count
        currentOffset = 0
    }
    
    // Check if there are more segments
    nextSegment, err := loadSegment(key, endSeg+1)
    result.Next = err == nil && len(nextSegment.Data) > 0
    
    return result, nil
}
```

## Performance Considerations

### Memory Efficiency
- Streaming file processing during upload
- Row-based segmentation for consistent memory usage
- No full file loading required for queries

### Storage Optimization
- Each segment is a valid CSV/TSV file with header
- Segments sized for optimal S3 performance
- Automatic cleanup after 30 days

### Query Performance
- Direct segment access based on row offset
- Minimal segment reads
- Efficient row range calculations

## Security
- Access control via x-account header
- Internal network restriction for admin endpoints
- Automatic file expiration
- No exposure of internal storage structure

## Limitations
1. Maximum file size: 100MB
2. Maximum rows per query: 1000
3. CSV/TSV formats only
4. 30-day storage limit

## Future Considerations
1. Compression support
2. Parallel upload processing
3. Response caching
4. Additional file format support
5. Custom retention periods 