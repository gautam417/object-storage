package handlers

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

type MinioObject interface {
	io.Reader
	Stat() (minio.ObjectInfo, error)
	Close() error
}

type MinioClientInterface interface {
	// Bucket operations
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	RemoveBucket(ctx context.Context, bucketName string) error
	ListBuckets(ctx context.Context) ([]minio.BucketInfo, error)

	// Object operations
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (MinioObject, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

type MinioAdapter struct {
	client *minio.Client
}

func NewMinioAdapter(client *minio.Client) *MinioAdapter {
	return &MinioAdapter{client: client}
}

// Bucket operations
func (m *MinioAdapter) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	return m.client.MakeBucket(ctx, bucketName, opts)
}

func (m *MinioAdapter) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	return m.client.BucketExists(ctx, bucketName)
}

func (m *MinioAdapter) RemoveBucket(ctx context.Context, bucketName string) error {
	return m.client.RemoveBucket(ctx, bucketName)
}

func (m *MinioAdapter) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	return m.client.ListBuckets(ctx)
}

// Object operations
func (m *MinioAdapter) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return m.client.PutObject(ctx, bucketName, objectName, reader, objectSize, opts)
}

func (m *MinioAdapter) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (MinioObject, error) {
	return m.client.GetObject(ctx, bucketName, objectName, opts)
}

func (m *MinioAdapter) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	return m.client.RemoveObject(ctx, bucketName, objectName, opts)
}
