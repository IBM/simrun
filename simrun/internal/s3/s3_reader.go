package s3

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Reader provides functionality to read files from S3 using URLs
type S3Reader struct {
	client *s3.Client
}

// S3ObjectInfo contains parsed S3 object information from URL
type S3ObjectInfo struct {
	Bucket string
	Key    string
	Region string
}

// NewS3Reader creates a new S3Reader instance
func NewS3Reader(ctx context.Context) (*S3Reader, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %w", err)
	}

	return &S3Reader{
		client: s3.NewFromConfig(cfg),
	}, nil
}

// NewS3ReaderWithRegion creates a new S3Reader instance with a specific region
func NewS3ReaderWithRegion(ctx context.Context, region string) (*S3Reader, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config for region %s: %w", region, err)
	}

	return &S3Reader{
		client: s3.NewFromConfig(cfg),
	}, nil
}

// ParseS3URL parses an S3 object URL and extracts bucket, key, and region information
// Expected format: https://bucket-name.s3.region.amazonaws.com/object-key
// Or: https://s3.region.amazonaws.com/bucket-name/object-key
func ParseS3URL(s3URL string) (*S3ObjectInfo, error) {
	parsedURL, err := url.Parse(s3URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %w", err)
	}

	if parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("only HTTPS URLs are supported: %s", s3URL)
	}

	host := parsedURL.Host
	path := strings.TrimPrefix(parsedURL.Path, "/")

	if path == "" {
		return nil, fmt.Errorf("missing object key in URL: %s", s3URL)
	}

	var bucket, key, region string

	// Check if it's a virtual-hosted-style URL: bucket-name.s3.region.amazonaws.com
	if strings.Contains(host, ".s3.") && strings.HasSuffix(host, ".amazonaws.com") {
		hostParts := strings.Split(host, ".")
		if len(hostParts) >= 4 {
			bucket = hostParts[0]
			// Extract region from s3.region.amazonaws.com
			if len(hostParts) >= 5 && hostParts[1] == "s3" {
				region = hostParts[2]
			}
			key = path
		} else {
			return nil, fmt.Errorf("invalid S3 virtual-hosted-style URL format: %s", s3URL)
		}
	} else if strings.HasPrefix(host, "s3.") && strings.HasSuffix(host, ".amazonaws.com") {
		// Path-style URL: s3.region.amazonaws.com/bucket-name/object-key
		hostParts := strings.Split(host, ".")
		if len(hostParts) >= 4 {
			region = hostParts[1]
		}

		pathParts := strings.SplitN(path, "/", 2)
		if len(pathParts) < 2 {
			return nil, fmt.Errorf("invalid S3 path-style URL format, missing object key: %s", s3URL)
		}
		bucket = pathParts[0]
		key = pathParts[1]
	} else {
		return nil, fmt.Errorf("invalid S3 URL format: %s", s3URL)
	}

	if bucket == "" || key == "" {
		return nil, fmt.Errorf("invalid S3 URL format, empty bucket or key: %s", s3URL)
	}

	return &S3ObjectInfo{
		Bucket: bucket,
		Key:    key,
		Region: region,
	}, nil
}

// ReadFileFromURL reads a file from S3 using the provided URL
func (r *S3Reader) ReadFileFromURL(ctx context.Context, s3URL string) ([]byte, error) {
	s3Info, err := ParseS3URL(s3URL)
	if err != nil {
		return nil, err
	}

	// If a region is specified in the URL and it's different from the current client's region,
	// create a new client for that region
	if s3Info.Region != "" {
		regionSpecificReader, err := NewS3ReaderWithRegion(ctx, s3Info.Region)
		if err != nil {
			return nil, fmt.Errorf("failed to create region-specific S3 reader for region %s: %w", s3Info.Region, err)
		}
		return regionSpecificReader.ReadFile(ctx, s3Info.Bucket, s3Info.Key)
	}

	return r.ReadFile(ctx, s3Info.Bucket, s3Info.Key)
}

// ReadFile reads a file from S3 using bucket and key
func (r *S3Reader) ReadFile(ctx context.Context, bucket, key string) ([]byte, error) {
	result, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get object from S3 bucket %s, key %s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	body, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read object body: %w", err)
	}

	return body, nil
}

// IsS3URL checks if a string is a valid S3 URL
func IsS3URL(input string) bool {
	if !strings.HasPrefix(input, "https://") {
		return false
	}

	parsedURL, err := url.Parse(input)
	if err != nil {
		return false
	}

	host := parsedURL.Host

	// Check for virtual-hosted-style: bucket-name.s3.region.amazonaws.com
	if strings.Contains(host, ".s3.") && strings.HasSuffix(host, ".amazonaws.com") {
		return true
	}

	// Check for path-style: s3.region.amazonaws.com
	if strings.HasPrefix(host, "s3.") && strings.HasSuffix(host, ".amazonaws.com") {
		return true
	}

	return false
}
