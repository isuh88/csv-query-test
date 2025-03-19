# CSV Query Service API Documentation

## Overview
This document describes the API endpoints for uploading and querying CSV/TSV files. The service provides secure file handling with automatic chunking for large files.

## Endpoints

### 1. Upload CSV File
Upload a CSV/TSV file to be stored securely.

**Endpoint:** `POST /cht/v1/secure-file/csv/:channelId/:fileName`

**Access:** Public (requires x-account header)

**Description:**
- Files are uploaded through a media server
- Large files are automatically chunked internally
- Partial chunk upload failures result in total upload failure
- Uploaded files are automatically deleted after 30 days
- Maximum file size: 100MB

**Request:**
- Headers:
  - Required: `x-account`
- Content-Type: `multipart/form-data`
- Body:
  ```
  file: [CSV/TSV file]
  ```

**Response:**
- Success (201 Created):
  ```json
  {
    "bucket": "bin-secure.csv",
    "key": "csv/...",
    "id": "1234",
    "type": "text/csv" | "text/tsv",
    "name": "customers.csv",
    "ext": "csv" | "tsv",
    "size": 5000000,
    "contentType": "csv" | "tsv",
    "chunks": 5
  }
  ```

**Error Responses:**
- 401 Unauthorized
  - Missing or expired x-account header
- 413 Content Too Large
  - File size exceeds 100MB limit
- 422 Unprocessable Entity
  - Invalid or corrupted CSV/TSV file
- 500 Internal Server Error
  - Upload failure (partial or complete)

### 2. Query CSV Chunks
Retrieve partial content from an uploaded CSV file.

**Endpoint:** `GET /admin/cht/v1/secure-file/csv/:key`

**Access:** Admin (Internal network only)

**Description:**
- Returns specified rows from the stored CSV file
- Supports pagination through offset and limit parameters
- Includes 'next' flag indicating more data availability

**Request:**
- Query Parameters:
  - `offset` (optional): Starting row index (default: 0)
  - `limit` (optional): Number of rows to return (default: 100, max: 1000)

**Response:**
- Success (200 OK):
  ```json
  {
    "header": ["colAName", "colBName", "colCName", "..."],
    "data": [
      ["colA0", "colB0", "colC0", "..."],
      ["colA1", "colB1", "colC1", "..."],
      "..."
    ],
    "next": true
  }
  ```

**Error Responses:**
- 400 Bad Request
  - Invalid offset or limit values
- 404 Not Found
  - File not found for given key
- 422 Unprocessable Entity
  - Invalid file format

## Examples

### Upload Example
```bash
curl -X POST \
  -H "x-account: account_token" \
  -F "file=@customers.csv" \
  "https://api.example.com/cht/v1/secure-file/csv/channel123/customers.csv"
```
```json
{
  "bucket": "bin-secure.csv",
  "key": "csvUpload/channel123/2024-03-21/customers.csv",
  "id": "csv_12345",
  "type": "text/csv",
  "name": "customers.csv",
  "ext": "csv",
  "size": 5242880,
  "contentType": "csv",
  "chunks": 5
}
```

### Query Example
```bash
curl -X GET \
  "https://api.example.com/admin/cht/v1/secure-file/csv/csvUpload/channel123/2024-03-21/customers.csv?offset=2000&limit=500"
```
```json
{
  "header": ["id", "name", "email", "created_at"],
  "data": [
    ["2001", "John Doe", "john@example.com", "2024-03-21T10:00:00Z"],
    ["2002", "Jane Smith", "jane@example.com", "2024-03-21T10:01:00Z"],
    // ... more rows ...
  ],
  "next": true
}
``` 