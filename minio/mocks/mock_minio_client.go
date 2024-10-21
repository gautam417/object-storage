package mocks

import (
	"context"
	"io"

	minioGo "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/mock"

	minio_adapter "github.com/spacelift-io/homework-object-storage/minio"
)

// MockMinioClient is a mock implementation of MinioClientInterface
type MockMinioClient struct {
	mock.Mock
}

func (m *MockMinioClient) MakeBucket(ctx context.Context, bucketName string, opts minioGo.MakeBucketOptions) error {
	args := m.Called(ctx, bucketName, opts)
	return args.Error(0)
}

func (m *MockMinioClient) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	args := m.Called(ctx, bucketName)
	return args.Bool(0), args.Error(1)
}

func (m *MockMinioClient) RemoveBucket(ctx context.Context, bucketName string) error {
	args := m.Called(ctx, bucketName)
	return args.Error(0)
}

func (m *MockMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minioGo.PutObjectOptions) (minioGo.UploadInfo, error) {
	args := m.Called(ctx, bucketName, objectName, reader, objectSize, opts)
	return args.Get(0).(minioGo.UploadInfo), args.Error(1)
}

func (m *MockMinioClient) GetObject(ctx context.Context, bucketName, objectName string, opts minioGo.GetObjectOptions) (minio_adapter.MinioObject, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	obj, _ := args.Get(0).(minio_adapter.MinioObject)
	return obj, args.Error(1)
}

func (m *MockMinioClient) RemoveObject(ctx context.Context, bucketName, objectName string, opts minioGo.RemoveObjectOptions) error {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Error(0)
}
