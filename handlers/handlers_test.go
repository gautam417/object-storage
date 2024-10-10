package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"github.com/spacelift-io/homework-object-storage/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockMinioClient struct {
	mock.Mock
}

func (m *MockMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, bucketName, objectName, reader, objectSize, opts)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}

func (m *MockMinioClient) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (MinioObject, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(MinioObject), args.Error(1)
}

func (m *MockMinioClient) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]minio.BucketInfo), args.Error(1)
}

func (m *MockMinioClient) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	args := m.Called(ctx, bucketName)
	return args.Bool(0), args.Error(1)
}

func (m *MockMinioClient) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	args := m.Called(ctx, bucketName, opts)
	return args.Error(0)
}

func (m *MockMinioClient) RemoveBucket(ctx context.Context, bucketName string) error {
	args := m.Called(ctx, bucketName)
	return args.Error(0)
}

func (m *MockMinioClient) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Error(0)
}

type MockMinioObject struct {
	mock.Mock
	io.Reader
}

func (m *MockMinioObject) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockMinioObject) Stat() (minio.ObjectInfo, error) {
	args := m.Called()
	return args.Get(0).(minio.ObjectInfo), args.Error(1)
}

func TestHandleCreateBucket(t *testing.T) {
	mockClient := new(MockMinioClient)
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	minioInstances := []storage.MinioInstance{
		{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
	}
	h := NewHandler(minioInstances, logger)

	h.getMinioClient = func(id string) (MinioClientInterface, error) {
		return mockClient, nil
	}

	mockClient.On("MakeBucket", mock.Anything, "test-bucket", mock.Anything).Return(nil)

	r := chi.NewRouter()
	r.Post("/buckets", h.HandleCreateBucket)

	body := bytes.NewBufferString(`{"bucketName":"test-bucket"}`)
	req, _ := http.NewRequest("POST", "/buckets", body)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	mockClient.AssertExpectations(t)
}

func TestHandleDeleteBucket(t *testing.T) {
	mockClient := new(MockMinioClient)
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	minioInstances := []storage.MinioInstance{
		{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
	}
	h := NewHandler(minioInstances, logger)

	h.getMinioClient = func(id string) (MinioClientInterface, error) {
		return mockClient, nil
	}

	mockClient.On("RemoveBucket", mock.Anything, "test-bucket").Return(nil)

	r := chi.NewRouter()
	r.Delete("/buckets/{bucketName}", h.HandleDeleteBucket)

	req, _ := http.NewRequest("DELETE", "/buckets/test-bucket", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	mockClient.AssertExpectations(t)
}

func TestBucketExists(t *testing.T) {
	mockClient := new(MockMinioClient)
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	minioInstances := []storage.MinioInstance{
		{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
	}
	h := NewHandler(minioInstances, logger)

	h.getMinioClient = func(id string) (MinioClientInterface, error) {
		return mockClient, nil
	}

	mockClient.On("BucketExists", mock.Anything, "existing-bucket").Return(true, nil)
	mockClient.On("BucketExists", mock.Anything, "non-existing-bucket").Return(false, nil)

	// Test existing bucket
	exists, err := mockClient.BucketExists(context.Background(), "existing-bucket")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test non-existing bucket
	exists, err = mockClient.BucketExists(context.Background(), "non-existing-bucket")
	assert.NoError(t, err)
	assert.False(t, exists)

	mockClient.AssertExpectations(t)
}

func TestHandlePutObject(t *testing.T) {
	testCases := []struct {
		name           string
		bucketName     string
		objectID       string
		bucketExists   bool
		expectedStatus int
		setupMock      func(*MockMinioClient)
	}{
		{
			name:           "Bucket exists",
			bucketName:     "existing-bucket",
			objectID:       "testid123",
			bucketExists:   true,
			expectedStatus: http.StatusOK,
			setupMock: func(m *MockMinioClient) {
				m.On("BucketExists", mock.Anything, "existing-bucket").Return(true, nil).Once()
				m.On("PutObject", mock.Anything, "existing-bucket", "testid123", mock.Anything, int64(-1), mock.Anything).
					Return(minio.UploadInfo{}, nil).Once()
			},
		},
		{
			name:           "Bucket doesn't exist",
			bucketName:     "non-existing-bucket",
			objectID:       "testid123",
			bucketExists:   false,
			expectedStatus: http.StatusNotFound,
			setupMock: func(m *MockMinioClient) {
				m.On("BucketExists", mock.Anything, "non-existing-bucket").Return(false, nil).Once()
			},
		},
		{
			name:           "Invalid object ID",
			bucketName:     "existing-bucket",
			objectID:       "invalid_id!",
			expectedStatus: http.StatusBadRequest,
			setupMock:      func(m *MockMinioClient) {}, // No mocks needed for this case
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockMinioClient)
			logger := logrus.New()
			logger.SetOutput(io.Discard)

			minioInstances := []storage.MinioInstance{
				{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
			}
			h := NewHandler(minioInstances, logger)

			h.getMinioClient = func(id string) (MinioClientInterface, error) {
				return mockClient, nil
			}

			tc.setupMock(mockClient)

			r := chi.NewRouter()
			r.Put("/buckets/{bucketName}/objects/{id}", h.HandlePutObject)

			body := bytes.NewBufferString("test")
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/buckets/%s/objects/%s", tc.bucketName, tc.objectID), body)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestHandleGetObject(t *testing.T) {
	testCases := []struct {
		name           string
		bucketName     string
		objectID       string
		bucketExists   bool
		objectExists   bool
		expectedStatus int
		expectedBody   string
		setupMock      func(*MockMinioClient, *MockMinioObject)
	}{
		{
			name:           "Bucket and object exist",
			bucketName:     "existing-bucket",
			objectID:       "testid123",
			bucketExists:   true,
			objectExists:   true,
			expectedStatus: http.StatusOK,
			expectedBody:   "test data",
			setupMock: func(mc *MockMinioClient, mo *MockMinioObject) {
				mc.On("BucketExists", mock.Anything, "existing-bucket").Return(true, nil).Once()
				mc.On("GetObject", mock.Anything, "existing-bucket", "testid123", mock.Anything).Return(mo, nil).Once()
				mo.On("Stat").Return(minio.ObjectInfo{ContentType: "application/octet-stream", Size: 9}, nil)
				mo.On("Close").Return(nil)
			},
		},
		{
			name:           "Bucket doesn't exist",
			bucketName:     "non-existing-bucket",
			objectID:       "testid123",
			bucketExists:   false,
			expectedStatus: http.StatusNotFound,
			setupMock: func(mc *MockMinioClient, mo *MockMinioObject) {
				mc.On("BucketExists", mock.Anything, "non-existing-bucket").Return(false, nil).Once()
			},
		},
		{
			name:           "Invalid object ID",
			bucketName:     "existing-bucket",
			objectID:       "invalid_id!",
			expectedStatus: http.StatusBadRequest,
			setupMock:      func(mc *MockMinioClient, mo *MockMinioObject) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockMinioClient)
			mockObject := new(MockMinioObject)
			if tc.objectExists {
				mockObject.Reader = bytes.NewReader([]byte(tc.expectedBody))
			}
			logger := logrus.New()
			logger.SetOutput(io.Discard)

			minioInstances := []storage.MinioInstance{
				{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
			}
			h := NewHandler(minioInstances, logger)

			h.getMinioClient = func(id string) (MinioClientInterface, error) {
				return mockClient, nil
			}

			tc.setupMock(mockClient, mockObject)

			r := chi.NewRouter()
			r.Get("/buckets/{bucketName}/objects/{id}", h.HandleGetObject)

			req, _ := http.NewRequest("GET", fmt.Sprintf("/buckets/%s/objects/%s", tc.bucketName, tc.objectID), nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			if tc.expectedStatus == http.StatusOK {
				assert.Equal(t, tc.expectedBody, rr.Body.String())
				assert.Equal(t, "application/octet-stream", rr.Header().Get("Content-Type"))
				assert.Equal(t, "9", rr.Header().Get("Content-Length"))
			}
			mockClient.AssertExpectations(t)
			mockObject.AssertExpectations(t)
		})
	}
}

func TestHandleDeleteObject(t *testing.T) {
	testCases := []struct {
		name           string
		bucketName     string
		objectID       string
		bucketExists   bool
		objectExists   bool
		expectedStatus int
		setupMock      func(*MockMinioClient)
	}{
		{
			name:           "Bucket and object exist",
			bucketName:     "existing-bucket",
			objectID:       "testid123",
			bucketExists:   true,
			objectExists:   true,
			expectedStatus: http.StatusNoContent,
			setupMock: func(m *MockMinioClient) {
				m.On("BucketExists", mock.Anything, "existing-bucket").Return(true, nil).Once()
				m.On("RemoveObject", mock.Anything, "existing-bucket", "testid123", mock.Anything).Return(nil).Once()
			},
		},
		{
			name:           "Bucket doesn't exist",
			bucketName:     "non-existing-bucket",
			objectID:       "testid123",
			bucketExists:   false,
			expectedStatus: http.StatusNotFound,
			setupMock: func(m *MockMinioClient) {
				m.On("BucketExists", mock.Anything, "non-existing-bucket").Return(false, nil).Once()
			},
		},
		{
			name:           "Invalid object ID",
			bucketName:     "existing-bucket",
			objectID:       "invalid_id!",
			expectedStatus: http.StatusBadRequest,
			setupMock:      func(m *MockMinioClient) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(MockMinioClient)
			logger := logrus.New()
			logger.SetOutput(io.Discard)

			minioInstances := []storage.MinioInstance{
				{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
			}
			h := NewHandler(minioInstances, logger)

			h.getMinioClient = func(id string) (MinioClientInterface, error) {
				return mockClient, nil
			}

			tc.setupMock(mockClient)

			r := chi.NewRouter()
			r.Delete("/buckets/{bucketName}/objects/{id}", h.HandleDeleteObject)

			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/buckets/%s/objects/%s", tc.bucketName, tc.objectID), nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			mockClient.AssertExpectations(t)
		})
	}
}
func TestHandleHealthCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	minioInstances := []storage.MinioInstance{
		{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
	}
	h := NewHandler(minioInstances, logger)

	r := chi.NewRouter()
	r.Get("/healthz", h.HandleHealthCheck)

	req, _ := http.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}