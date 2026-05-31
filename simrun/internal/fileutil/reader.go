package fileutil

import (
	"context"
	"fmt"
	"os"

	"github.com/IBM/simrun/simrun/internal/s3"
)

// FileReader provides a unified interface for reading files from local filesystem or S3
type FileReader struct {
	s3Reader *s3.S3Reader
}

// NewFileReader creates a new FileReader instance
func NewFileReader() (*FileReader, error) {
	s3Reader, err := s3.NewS3Reader(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 reader: %w", err)
	}

	return &FileReader{
		s3Reader: s3Reader,
	}, nil
}

// ReadFile reads a file from either local filesystem or S3 based on the input
// If input is an S3 URL, it reads from S3, otherwise it reads from local filesystem
func (fr *FileReader) ReadFile(ctx context.Context, input string) ([]byte, error) {
	if s3.IsS3URL(input) {
		return fr.s3Reader.ReadFileFromURL(ctx, input)
	}

	// Read from local filesystem
	content, err := os.ReadFile(input)
	if err != nil {
		return nil, fmt.Errorf("unable to read local file %s: %w", input, err)
	}

	return content, nil
}

// GetEnvironmentS3URL retrieves the S3 URL from the specified environment variable
// Returns empty string if the environment variable is not set
func GetEnvironmentS3URL(envVar string) string {
	return os.Getenv(envVar)
}

// ResolveInputSource determines the input source based on command line arguments and environment variables
// Priority: 1. Command line arguments, 2. Environment variable with S3 URL
func ResolveInputSource(cmdLineInputs []string, envVarName string) ([]string, error) {
	// If command line inputs are provided, use them
	if len(cmdLineInputs) > 0 {
		return cmdLineInputs, nil
	}

	// Check for S3 URL in environment variable
	s3URL := GetEnvironmentS3URL(envVarName)
	if s3URL != "" {
		if !s3.IsS3URL(s3URL) {
			return nil, fmt.Errorf("environment variable %s contains invalid S3 URL: %s", envVarName, s3URL)
		}
		return []string{s3URL}, nil
	}

	return nil, fmt.Errorf("no input source specified. Provide file paths as arguments or set %s environment variable with S3 URL", envVarName)
}
