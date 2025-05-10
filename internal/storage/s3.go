package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"net/url"
	"strings"
	"time"
)

type S3Config struct {
	Endpoint        string `json:"endpoint" yaml:"endpoint"`
	AccessKeyID     string `json:"access_key_id" yaml:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret" yaml:"access_key_secret"`
	BucketName      string `json:"bucket_name" yaml:"bucket_name"`
	PublicURL       string `json:"public_url" yaml:"public_url"`
	Region          string `json:"region" yaml:"region"`
}

type Provider interface {
	UploadFile(ctx context.Context, data io.Reader, filename string, contentType string) (string, error)
	GetFileURL(filename string) (string, error)
}

type S3Provider struct {
	config S3Config
	client *manager.Uploader
}

func NewS3Provider(cfg S3Config) (*S3Provider, error) {
	if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" || cfg.BucketName == "" || cfg.Endpoint == "" {
		return nil, fmt.Errorf("incomplete S3 configuration")
	}

	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: cfg.Endpoint,
		}, nil
	})

	s3Cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.AccessKeySecret, "")),
		config.WithRegion("auto"),
	)

	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(s3Cfg)

	uploader := manager.NewUploader(s3Client)

	return &S3Provider{
		config: cfg,
		client: uploader,
	}, nil
}

func (s *S3Provider) UploadFile(ctx context.Context, data io.Reader, filename string, contentType string) (string, error) {
	fileBytes, err := io.ReadAll(data)
	if err != nil {
		return "", fmt.Errorf("failed to read file data: %w", err)
	}

	if _, err := s.client.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.BucketName),
		Key:         aws.String(filename),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	return s.GetFileURL(filename)
}

func (s *S3Provider) GetFileURL(filename string) (string, error) {
	if s.config.PublicURL != "" {

		baseURL := strings.TrimSuffix(s.config.PublicURL, "/")
		return fmt.Sprintf("%s/%s", baseURL, url.PathEscape(filename)), nil
	}

	endpoint := s.config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("s3.%s.amazonaws.com", s.config.Region)
	}

	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	return fmt.Sprintf("https://%s/%s/%s",
		endpoint, s.config.BucketName, url.PathEscape(filename)), nil
}

type FileInfo struct {
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	UploadedAt  time.Time `json:"uploaded_at"`
	URL         string    `json:"url"`
}
