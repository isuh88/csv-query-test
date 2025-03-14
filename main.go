package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	s3Client, err := NewS3Client()
	if err != nil {
		log.Fatalf("Failed to create S3 client: %v", err)
	}

	// Original handler (no optimizations)
	handler := NewHandler(s3Client)
	http.HandleFunc("/csv/original", handler.handleCSV)

	// Create cached client for handlers that need it
	cachedClient := NewCachedS3Client(s3Client.client)

	// Cached only handler
	cachedHandler := NewCachedHandler(cachedClient)
	http.HandleFunc("/csv/cached", cachedHandler.handleCSV)

	// Fast skip only handler
	fastSkipHandler := NewFastSkipHandler(s3Client)
	http.HandleFunc("/csv/fast-skip", fastSkipHandler.handleCSV)

	// Both optimizations handler
	optimizedHandler := NewOptimizedHandler(cachedClient)
	http.HandleFunc("/csv/optimized", optimizedHandler.handleCSV)

	fmt.Println("Server starting on :8080...")
	fmt.Println("\nAvailable endpoints:")
	fmt.Println("1. Original (no optimizations):")
	fmt.Println("   /csv/original?file=customers-500000.csv&offset=0&limit=100")
	fmt.Println("\n2. Cached only:")
	fmt.Println("   /csv/cached?file=customers-500000.csv&offset=0&limit=100")
	fmt.Println("\n3. Fast skip only:")
	fmt.Println("   /csv/fast-skip?file=customers-500000.csv&offset=0&limit=100")
	fmt.Println("\n4. Both optimizations:")
	fmt.Println("   /csv/optimized?file=customers-500000.csv&offset=0&limit=100")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
