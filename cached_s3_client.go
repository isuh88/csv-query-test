package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type CachedS3Client struct {
	client *s3.Client
	cache  map[string]*FileCache
	mutex  sync.RWMutex
}

type FileCache struct {
	content    []byte
	lastAccess time.Time
	mutex      sync.RWMutex
}

func NewCachedS3Client(baseClient *s3.Client) *CachedS3Client {
	return &CachedS3Client{
		client: baseClient,
		cache:  make(map[string]*FileCache),
	}
}

func (c *CachedS3Client) GetCSVContent(key string) (io.ReadSeeker, error) {
	if !validKeys[key] {
		return nil, fmt.Errorf("invalid key: %s", key)
	}

	// Try to get from cache first
	c.mutex.RLock()
	if cache, exists := c.cache[key]; exists {
		c.mutex.RUnlock()
		cache.mutex.Lock()
		cache.lastAccess = time.Now()
		cache.mutex.Unlock()
		return bytes.NewReader(cache.content), nil
	}
	c.mutex.RUnlock()

	// If not in cache, download and cache it
	output, err := c.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %v", err)
	}
	defer output.Body.Close()

	// Read the entire file
	content, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %v", err)
	}

	// Store in cache
	c.mutex.Lock()
	c.cache[key] = &FileCache{
		content:    content,
		lastAccess: time.Now(),
	}
	c.mutex.Unlock()

	return bytes.NewReader(content), nil
}

// Optional: Add cache cleanup method
func (c *CachedS3Client) CleanupCache(maxAge time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	for key, cache := range c.cache {
		cache.mutex.RLock()
		if now.Sub(cache.lastAccess) > maxAge {
			delete(c.cache, key)
		}
		cache.mutex.RUnlock()
	}
}
