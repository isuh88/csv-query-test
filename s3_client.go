package main

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	bucketName = "bin.exp.channel.io"
	awsProfile = "ch-dev"
	awsRegion  = "ap-northeast-2"
)

type S3Client struct {
	client   *s3.S3
	uploader *s3manager.Uploader
}

type S3UploadDTO struct {
	Key     string
	Content []byte
}

func NewS3Client() (*S3Client, error) {
	// Load shared config and credentials
	cfg := aws.NewConfig().
		WithRegion(awsRegion).
		WithCredentialsChainVerboseErrors(true)

	// Create session with shared config enabled
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile:           awsProfile,
		Config:            *cfg,
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create AWS session: %v", err)
	}

	client := s3.New(sess)
	uploader := s3manager.NewUploaderWithClient(client)

	return &S3Client{
		client:   client,
		uploader: uploader,
	}, nil
}

func (c *S3Client) GetCSVContent(key string) (io.ReadCloser, error) {
	output, err := c.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %v", err)
	}

	return output.Body, nil
}

// UploadSegment uploads a segment of CSV data to S3
func (c *S3Client) UploadSegment(key string, data []byte) error {
	_, err := c.client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to upload segment: %v", err)
	}
	return nil
}

func (c *S3Client) BatchUpload(targets []S3UploadDTO) error {
	log.Printf("Initializing batch upload for %d files...", len(targets))
	objects := make([]s3manager.BatchUploadObject, len(targets))

	totalSize := int64(0)
	for i, target := range targets {
		totalSize += int64(len(target.Content))
		input := &s3manager.UploadInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(target.Key),
			Body:   bytes.NewReader(target.Content),
		}
		objects[i] = s3manager.BatchUploadObject{
			Object: input,
			After: func() error {
				log.Printf("Successfully uploaded segment %d/%d", i+1, len(targets))
				return nil
			},
		}
	}

	log.Printf("Starting batch upload (total size: %.2f MB)...", float64(totalSize)/1024/1024)
	iter := &s3manager.UploadObjectsIterator{Objects: objects}
	if err := c.uploader.UploadWithIterator(aws.BackgroundContext(), iter); err != nil {
		return fmt.Errorf("batch upload failed: %v", err)
	}

	log.Printf("Batch upload completed successfully")
	return nil
}

// ValidateUploadKey checks if the upload path is valid
func (c *S3Client) ValidateUploadKey(key string) error {
	if key == "" {
		return fmt.Errorf("empty key is not allowed")
	}
	return nil
}
