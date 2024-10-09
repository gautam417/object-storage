package handlers

import (
	"bytes"
	"context"
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

func TestHandleGetObject(t *testing.T) {
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

	testData := []byte("test data")
	mockObject := &MockMinioObject{Reader: bytes.NewReader(testData)}
	mockObject.On("Close").Return(nil)
	mockObject.On("Stat").Return(minio.ObjectInfo{
		ContentType: "application/octet-stream",
		Size:        int64(len(testData)),
	}, nil)

	mockClient.On("GetObject", mock.Anything, "default-bucket", "testid123", mock.Anything).
		Return(mockObject, nil)

	r := chi.NewRouter()
	r.Get("/object/{id}", h.HandleGetObject)

	req, _ := http.NewRequest("GET", "/object/testid123", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/octet-stream", rr.Header().Get("Content-Type"))
	assert.Equal(t, "9", rr.Header().Get("Content-Length"))
	assert.Equal(t, string(testData), rr.Body.String())
	mockClient.AssertExpectations(t)
	mockObject.AssertExpectations(t)
}
func TestHandlePutObject(t *testing.T) {
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

	mockClient.On("PutObject", mock.Anything, "default-bucket", "testid123", mock.Anything, int64(-1), mock.Anything).
		Return(minio.UploadInfo{}, nil)

	r := chi.NewRouter()
	r.Put("/object/{id}", h.HandlePutObject)

	body := bytes.NewBufferString("test")
	req, _ := http.NewRequest("PUT", "/object/testid123", body)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockClient.AssertExpectations(t)
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
