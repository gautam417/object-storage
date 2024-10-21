package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	minio "github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
 
	minio_adapter "github.com/spacelift-io/homework-object-storage/minio"
	"github.com/spacelift-io/homework-object-storage/minio/mocks"
)

func TestHandleCreateBucket(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	minioInstances := []minio_adapter.MinioInstance{
		{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
	}

	tests := []struct {
		name           string
		bucketName     string
		mockSetup      func(*mocks.MockMinioClient)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:       "Successful bucket creation",
			bucketName: "new-bucket",
			mockSetup: func(m *mocks.MockMinioClient) {
				m.On("MakeBucket", mock.Anything, "new-bucket", mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"message":"Bucket created successfully"}`,
		},
		{
			name:       "Bucket already exists",
			bucketName: "existing-bucket",
			mockSetup: func(m *mocks.MockMinioClient) {
				m.On("MakeBucket", mock.Anything, "existing-bucket", mock.Anything).Return(
					errors.New("Your previous request to create the named bucket succeeded and you already own it."))
			},
			expectedStatus: http.StatusConflict,
			expectedBody:   "Bucket already exists",
		},
		{
			name:       "Bucket name already taken",
			bucketName: "taken-bucket",
			mockSetup: func(m *mocks.MockMinioClient) {
				m.On("MakeBucket", mock.Anything, "taken-bucket", mock.Anything).Return(
					errors.New("Bucket name already exists"))
			},
			expectedStatus: http.StatusConflict,
			expectedBody:   "Bucket name already taken",
		},
		{
			name:       "Internal server error",
			bucketName: "error-bucket",
			mockSetup: func(m *mocks.MockMinioClient) {
				m.On("MakeBucket", mock.Anything, "error-bucket", mock.Anything).Return(
					errors.New("Internal server error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to create bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockMinioClient)
			tt.mockSetup(mockClient)

			h := NewHandler(minioInstances, logger)
			h.getMinioClient = func(id string) (minio_adapter.MinioClientInterface, error) {
				return mockClient, nil
			}

			r := chi.NewRouter()
			r.Post("/buckets", h.HandleCreateBucket)

			body := bytes.NewBufferString(fmt.Sprintf(`{"bucketName":"%s"}`, tt.bucketName))
			req, _ := http.NewRequest("POST", "/buckets", body)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Equal(t, tt.expectedBody, strings.TrimSpace(rr.Body.String()))
			mockClient.AssertExpectations(t)
		})
	}
}

func TestHandleDeleteBucket(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	minioInstances := []minio_adapter.MinioInstance{
		{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
	}

	tests := []struct {
		name           string
		bucketName     string
		setupMock      func(*mocks.MockMinioClient)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:       "Successful bucket deletion",
			bucketName: "empty-bucket",
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("RemoveBucket", mock.Anything, "empty-bucket").Return(nil)
			},
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:       "Delete non-empty bucket",
			bucketName: "non-empty-bucket",
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("RemoveBucket", mock.Anything, "non-empty-bucket").Return(
					minio.ErrorResponse{Code: "BucketNotEmpty"})
			},
			expectedStatus: http.StatusConflict,
			expectedBody:   "The bucket you tried to delete is not empty\n",
		},
		{
			name:       "Delete non-existent bucket",
			bucketName: "non-existent-bucket",
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("RemoveBucket", mock.Anything, "non-existent-bucket").Return(
					minio.ErrorResponse{Code: "NoSuchBucket"})
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "The specified bucket does not exist\n",
		},
		{
			name:       "Internal server error",
			bucketName: "error-bucket",
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("RemoveBucket", mock.Anything, "error-bucket").Return(
					errors.New("Internal server error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to delete bucket\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockMinioClient)
			tt.setupMock(mockClient)

			h := NewHandler(minioInstances, logger)
			h.getMinioClient = func(id string) (minio_adapter.MinioClientInterface, error) {
				return mockClient, nil
			}

			r := chi.NewRouter()
			r.Delete("/buckets/{bucketName}", h.HandleDeleteBucket)

			req, _ := http.NewRequest("DELETE", "/buckets/"+tt.bucketName, nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Equal(t, tt.expectedBody, rr.Body.String())
			mockClient.AssertExpectations(t)
		})
	}
}

func TestBucketExists(t *testing.T) {
    mockClient := new(mocks.MockMinioClient)
    logger := logrus.New()
    logger.SetOutput(io.Discard)

    minioInstances := []minio_adapter.MinioInstance{
        {Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
    }
    h := NewHandler(minioInstances, logger)

    h.getMinioClient = func(id string) (minio_adapter.MinioClientInterface, error) {
        return mockClient, nil
    }

    tests := []struct {
        bucketName string
        expected   bool
        err        error
    }{
        {"existing-bucket", true, nil},
        {"non-existing-bucket", false, nil},
    }

    mockClient.On("BucketExists", mock.Anything, "existing-bucket").Return(true, nil)
    mockClient.On("BucketExists", mock.Anything, "non-existing-bucket").Return(false, nil)

    for _, test := range tests {
        t.Run(test.bucketName, func(t *testing.T) {
            exists, err := mockClient.BucketExists(context.Background(), test.bucketName)
            assert.NoError(t, err)
            assert.Equal(t, test.expected, exists)
        })
    }

    mockClient.AssertExpectations(t)
}

func TestHandlePutObject(t *testing.T) {
	testCases := []struct {
		name           string
		bucketName     string
		objectID       string
		bucketExists   bool
		expectedStatus int
		setupMock      func(*mocks.MockMinioClient)
	}{
		{
			name:           "Bucket exists",
			bucketName:     "existing-bucket",
			objectID:       "testid123",
			bucketExists:   true,
			expectedStatus: http.StatusOK,
			setupMock: func(m *mocks.MockMinioClient) {
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
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("BucketExists", mock.Anything, "non-existing-bucket").Return(false, nil).Once()
			},
		},
		{
			name:           "Invalid object ID",
			bucketName:     "existing-bucket",
			objectID:       "invalid_id!",
			expectedStatus: http.StatusBadRequest,
			setupMock:      func(m *mocks.MockMinioClient) {}, // No mocks needed for this case
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(mocks.MockMinioClient)
			logger := logrus.New()
			logger.SetOutput(io.Discard)

			minioInstances := []minio_adapter.MinioInstance{
				{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
			}
			h := NewHandler(minioInstances, logger)

			h.getMinioClient = func(id string) (minio_adapter.MinioClientInterface, error) {
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
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	minioInstances := []minio_adapter.MinioInstance{
		{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
	}

	tests := []struct {
		name           string
		bucketName     string
		objectID       string
		setupMock      func(*mocks.MockMinioClient)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:       "Successful object retrieval",
			bucketName: "testbucket",
			objectID:   "testobject123",
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("BucketExists", mock.Anything, "testbucket").Return(true, nil)
				mockObject := new(mocks.MockMinioObject)
				mockObject.On("Stat").Return(minio.ObjectInfo{ContentType: "text/plain"}, nil)
				mockObject.On("Read", mock.Anything).Run(func(args mock.Arguments) {
					b := args.Get(0).([]byte)
					copy(b, "test content")
				}).Return(12, io.EOF)
				mockObject.On("Close").Return(nil)
				m.On("GetObject", mock.Anything, "testbucket", "testobject123", mock.Anything).Return(mockObject, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "test content",
		},
		{
			name:       "Non-existent bucket",
			bucketName: "nonexistentbucket",
			objectID:   "testobject123",
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("BucketExists", mock.Anything, "nonexistentbucket").Return(false, nil)
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Bucket not found\n",
		},
		{
			name:       "Non-existent object",
			bucketName: "testbucket",
			objectID:   "nonexistentobject",
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("BucketExists", mock.Anything, "testbucket").Return(true, nil)
				m.On("GetObject", mock.Anything, "testbucket", "nonexistentobject", mock.Anything).Return(nil, minio.ErrorResponse{Code: "NoSuchKey"})
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Object not found\n",
		},
		{
			name:       "Server error",
			bucketName: "testbucket",
			objectID:   "errorobject",
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("BucketExists", mock.Anything, "testbucket").Return(true, nil)
				m.On("GetObject", mock.Anything, "testbucket", "errorobject", mock.Anything).Return(nil, errors.New("internal server error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal server error\n",
		},
		{
			name:       "Invalid object ID",
			bucketName: "testbucket",
			objectID:   "invalid_id!",
			setupMock:  func(m *mocks.MockMinioClient) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "ID must contain only alphanumeric characters\n",
		},
		{
			name:       "Error getting object stats",
			bucketName: "testbucket",
			objectID:   "staterrorobject",
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("BucketExists", mock.Anything, "testbucket").Return(true, nil)
				mockObject := new(mocks.MockMinioObject)
				mockObject.On("Stat").Return(minio.ObjectInfo{}, errors.New("error getting object stats"))
				mockObject.On("Close").Return(nil) // Add this line
				m.On("GetObject", mock.Anything, "testbucket", "staterrorobject", mock.Anything).Return(mockObject, nil)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to get object stats\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mocks.MockMinioClient)
			tt.setupMock(mockClient)

			h := NewHandler(minioInstances, logger)
			h.getMinioClient = func(id string) (minio_adapter.MinioClientInterface, error) {
				return mockClient, nil
			}

			r := chi.NewRouter()
			r.Get("/buckets/{bucketName}/objects/{id}", h.HandleGetObject)

			req, _ := http.NewRequest("GET", fmt.Sprintf("/buckets/%s/objects/%s", tt.bucketName, tt.objectID), nil)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Equal(t, tt.expectedBody, rr.Body.String())
			mockClient.AssertExpectations(t)
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
		setupMock      func(*mocks.MockMinioClient)
	}{
		{
			name:           "Bucket and object exist",
			bucketName:     "existing-bucket",
			objectID:       "testid123",
			bucketExists:   true,
			objectExists:   true,
			expectedStatus: http.StatusNoContent,
			setupMock: func(m *mocks.MockMinioClient) {
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
			setupMock: func(m *mocks.MockMinioClient) {
				m.On("BucketExists", mock.Anything, "non-existing-bucket").Return(false, nil).Once()
			},
		},
		{
			name:           "Invalid object ID",
			bucketName:     "existing-bucket",
			objectID:       "invalid_id!",
			expectedStatus: http.StatusBadRequest,
			setupMock:      func(m *mocks.MockMinioClient) {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := new(mocks.MockMinioClient)
			logger := logrus.New()
			logger.SetOutput(io.Discard)

			minioInstances := []minio_adapter.MinioInstance{
				{Endpoint: "localhost:9000", AccessKey: "test", SecretKey: "test"},
			}
			h := NewHandler(minioInstances, logger)

			h.getMinioClient = func(id string) (minio_adapter.MinioClientInterface, error) {
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

	minioInstances := []minio_adapter.MinioInstance{
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
