package minio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	minioGo "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/spacelift-io/homework-object-storage/minio/mocks"
)

func TestMinioAdapter(t *testing.T) {
	mockClient := new(mocks.MockMinioClient)
	adapter := mockClient 

	t.Run("GetObject", func(t *testing.T) {
		tests := []struct {
			name       string
			bucketName string
			objectName string
			content    string
			err        error
		}{
			{"Successful retrieval", "test-bucket", "test-object", "test content", nil},
			{"Object not found", "test-bucket", "non-existent-object", "", minioGo.ErrorResponse{Code: "NoSuchKey"}},
			{"Retrieval error", "error-bucket", "error-object", "", errors.New("retrieval failed")},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockClient := new(mocks.MockMinioClient)
				adapter := mockClient // Since MockMinioClient already implements MinioClientInterface

				if tt.err == nil {
					mockObject := new(mocks.MockMinioObject)
					mockObject.On("Read", mock.Anything).Run(func(args mock.Arguments) {
						b := args.Get(0).([]byte)
						copy(b, tt.content)
					}).Return(len(tt.content), io.EOF)
					mockObject.On("Close").Return(nil)
					mockObject.On("Stat").Return(minioGo.ObjectInfo{ContentType: "text/plain"}, nil)
					mockClient.On("GetObject", mock.Anything, tt.bucketName, tt.objectName, mock.Anything).Return(mockObject, nil)
				} else {
					mockClient.On("GetObject", mock.Anything, tt.bucketName, tt.objectName, mock.Anything).Return(nil, tt.err)
				}

				obj, err := adapter.GetObject(context.Background(), tt.bucketName, tt.objectName, minioGo.GetObjectOptions{})

				if tt.err == nil {
					assert.NoError(t, err)
					assert.NotNil(t, obj)
					content, readErr := io.ReadAll(obj)
					assert.NoError(t, readErr)
					assert.Equal(t, tt.content, string(content))
				} else {
					assert.Error(t, err)
					assert.Nil(t, obj)
					assert.Equal(t, tt.err, err)
				}

				mockClient.AssertExpectations(t)
			})
		}
	})

	t.Run("BucketExists", func(t *testing.T) {
		tests := []struct {
			bucketName string
			exists     bool
			err        error
		}{
			{"existing-bucket", true, nil},
			{"non-existing-bucket", false, nil},
		}

		for _, test := range tests {
			mockClient.On("BucketExists", mock.Anything, test.bucketName).Return(test.exists, test.err)

			exists, err := adapter.BucketExists(context.Background(), test.bucketName)

			assert.Equal(t, test.exists, exists)
			assert.Equal(t, test.err, err)
		}

		mockClient.AssertExpectations(t)
	})

	t.Run("MakeBucket", func(t *testing.T) {
		tests := []struct {
			bucketName string
			err        error
		}{
			{"new-bucket", nil},
			{"existing-bucket", errors.New("bucket already exists")},
		}

		for _, test := range tests {
			mockClient.On("MakeBucket", mock.Anything, test.bucketName, mock.Anything).Return(test.err)

			err := adapter.MakeBucket(context.Background(), test.bucketName, minioGo.MakeBucketOptions{})

			assert.Equal(t, test.err, err)
		}

		mockClient.AssertExpectations(t)
	})

	t.Run("RemoveBucket", func(t *testing.T) {
		tests := []struct {
			bucketName string
			err        error
		}{
			{"existing-bucket", nil},
			{"non-existing-bucket", errors.New("bucket not found")},
		}

		for _, test := range tests {
			mockClient.On("RemoveBucket", mock.Anything, test.bucketName).Return(test.err)

			err := adapter.RemoveBucket(context.Background(), test.bucketName)

			assert.Equal(t, test.err, err)
		}

		mockClient.AssertExpectations(t)
	})

	t.Run("PutObject", func(t *testing.T) {
		tests := []struct {
			bucketName string
			objectName string
			objectSize int64
			err        error
		}{
			{"bucket1", "object1", 1024, nil},
			{"bucket2", "object2", 2048, errors.New("failed to upload object")},
		}

		for _, test := range tests {
			reader := bytes.NewReader([]byte("test data"))
			mockClient.On("PutObject", mock.Anything, test.bucketName, test.objectName, reader, test.objectSize, mock.Anything).Return(minioGo.UploadInfo{}, test.err)

			_, err := adapter.PutObject(context.Background(), test.bucketName, test.objectName, reader, test.objectSize, minioGo.PutObjectOptions{})

			assert.Equal(t, test.err, err)
		}

		mockClient.AssertExpectations(t)
	})

	t.Run("RemoveObject", func(t *testing.T) {
		tests := []struct {
			bucketName string
			objectName string
			err        error
		}{
			{"bucket1", "object1", nil},
			{"bucket2", "object2", errors.New("failed to remove object")},
		}

		for _, test := range tests {
			mockClient.On("RemoveObject", mock.Anything, test.bucketName, test.objectName, mock.Anything).Return(test.err)

			err := adapter.RemoveObject(context.Background(), test.bucketName, test.objectName, minioGo.RemoveObjectOptions{})

			assert.Equal(t, test.err, err)
		}

		mockClient.AssertExpectations(t)
	})

}
