package main

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	bucketName = "bin-secure.exp.channel.io"
	awsProfile = "ch-dev"
	awsRegion  = "ap-northeast-2"
)

var (
	validKeys = map[string]bool{
		"customers-500000.csv":  true,
		"customers-1000000.csv": true,
		"customers-2000000.csv": true,
	}
)

type S3Client struct {
	client *s3.Client
}

func NewS3Client() (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(awsRegion),
		config.WithSharedConfigProfile(awsProfile),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	client := s3.NewFromConfig(cfg)
	return &S3Client{client: client}, nil
}

func (c *S3Client) GetCSVContent(key string) (io.ReadCloser, error) {
	if !validKeys[key] {
		return nil, fmt.Errorf("invalid key: %s", key)
	}

	output, err := c.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %v", err)
	}

	return output.Body, nil
}
