package anki

import (
	"atamagaii/internal/db"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockStorageProvider implements the storage.Provider interface for testing
type MockStorageProvider struct {
	mu                 sync.Mutex
	uploadedFiles      map[string]string
	uploadDelay        time.Duration
	concurrentCalls    int
	maxConcurrentCalls int
}

func NewMockStorageProvider(uploadDelay time.Duration) *MockStorageProvider {
	return &MockStorageProvider{
		uploadedFiles:      make(map[string]string),
		uploadDelay:        uploadDelay,
		concurrentCalls:    0,
		maxConcurrentCalls: 0,
	}
}

func (m *MockStorageProvider) UploadFile(ctx context.Context, data io.Reader, filename string, contentType string) (string, error) {
	m.mu.Lock()
	m.concurrentCalls++
	if m.concurrentCalls > m.maxConcurrentCalls {
		m.maxConcurrentCalls = m.concurrentCalls
	}
	m.mu.Unlock()

	// Simulate work with delay
	select {
	case <-time.After(m.uploadDelay):
	case <-ctx.Done():
		return "", ctx.Err()
	}

	mockURL := "https://example.com/" + filename

	m.mu.Lock()
	m.uploadedFiles[filename] = mockURL
	m.concurrentCalls--
	m.mu.Unlock()

	return mockURL, nil
}

func (m *MockStorageProvider) GetFileURL(filename string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if url, ok := m.uploadedFiles[filename]; ok {
		return url, nil
	}
	return "https://example.com/" + filename, nil
}

func TestUploadMediaFilesParallel(t *testing.T) {
	// Create a temp directory with test files
	tempDir, err := os.MkdirTemp("", "anki_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test media files
	mediaFiles := []MediaFile{}

	for i := 1; i <= 10; i++ {
		fileName := fmt.Sprintf("file%d.mp3", i)
		filePath := filepath.Join(tempDir, fileName)

		err := os.WriteFile(filePath, []byte("test audio content"), 0644)
		require.NoError(t, err)

		mediaFiles = append(mediaFiles, MediaFile{
			FileName:    fileName,
			ContentType: "audio/mpeg",
			FilePath:    filePath,
		})
	}

	tests := []struct {
		name            string
		workerCount     int
		uploadDelay     time.Duration
		expectedWorkers int
	}{
		{
			name:            "SingleWorker",
			workerCount:     1,
			uploadDelay:     50 * time.Millisecond,
			expectedWorkers: 1,
		},
		{
			name:            "MultipleWorkers",
			workerCount:     3,
			uploadDelay:     50 * time.Millisecond,
			expectedWorkers: 3,
		},
		{
			name:            "MoreWorkersThanFiles",
			workerCount:     15,
			uploadDelay:     50 * time.Millisecond,
			expectedWorkers: 10, // Should be limited to number of files
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock storage provider with configured delay
			mockStorage := NewMockStorageProvider(tt.uploadDelay)

			// Create the processor with mock storage
			processor := NewProcessor(&db.Storage{}, mockStorage)

			// Create context with worker count configuration
			ctx := context.WithValue(context.Background(), "uploadWorkers", tt.workerCount)

			// Measure upload time
			startTime := time.Now()

			// Run the upload
			mediaURLs, err := processor.UploadMediaFiles(ctx, mediaFiles)

			// Calculate elapsed time
			elapsed := time.Since(startTime)

			// Verify results
			require.NoError(t, err)
			assert.Equal(t, len(mediaFiles), len(mediaURLs), "All files should be uploaded")

			// Verify concurrency
			assert.Equal(t, tt.expectedWorkers, mockStorage.maxConcurrentCalls,
				"Expected %d concurrent workers, got %d", tt.expectedWorkers, mockStorage.maxConcurrentCalls)

			// Verify upload was actually faster with more workers (if we have multiple files)
			if tt.name != "SingleWorker" && tt.name != "MoreWorkersThanFiles" {
				// Expected time if serial: files * delay
				// Expected time if parallel: ~(files / workers) * delay
				expectedSerialTime := time.Duration(len(mediaFiles)) * tt.uploadDelay
				maxExpectedParallelTime := time.Duration(len(mediaFiles)/tt.workerCount+1) * tt.uploadDelay * 2 // Add buffer

				assert.Less(t, elapsed, expectedSerialTime,
					"Parallel upload (%v) should be faster than serial upload (%v)", elapsed, expectedSerialTime)
				assert.LessOrEqual(t, elapsed, maxExpectedParallelTime,
					"Parallel upload time (%v) should be proportional to worker count", elapsed)
			}
		})
	}
}

func TestUploadMediaFilesWithErrors(t *testing.T) {
	// Create a temp directory with one valid file
	tempDir, err := os.MkdirTemp("", "anki_test_errors_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a valid test file
	validFilePath := filepath.Join(tempDir, "valid.mp3")
	err = os.WriteFile(validFilePath, []byte("test audio content"), 0644)
	require.NoError(t, err)

	// Set up test media files - one valid, one invalid
	mediaFiles := []MediaFile{
		{
			FileName:    "valid.mp3",
			ContentType: "audio/mpeg",
			FilePath:    validFilePath,
		},
		{
			FileName:    "invalid.mp3",
			ContentType: "audio/mpeg",
			FilePath:    filepath.Join(tempDir, "nonexistent.mp3"), // This will cause an error
		},
	}

	// Create a mock storage provider
	mockStorage := NewMockStorageProvider(10 * time.Millisecond)

	// Create the processor with mock storage
	processor := NewProcessor(&db.Storage{}, mockStorage)

	// Create context
	ctx := context.Background()

	// Run the upload
	mediaURLs, err := processor.UploadMediaFiles(ctx, mediaFiles)

	// Verify results
	assert.Error(t, err, "Should return an error due to invalid file")
	assert.Contains(t, err.Error(), "media file not found")
	assert.Equal(t, 1, len(mediaURLs), "Should return URLs for the valid files even when errors occur")
	assert.Contains(t, mediaURLs, "valid.mp3", "Should contain URL for the valid file")
}
