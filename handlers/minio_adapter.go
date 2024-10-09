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
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (MinioObject, error)
	ListBuckets(ctx context.Context) ([]minio.BucketInfo, error)
}

type MinioAdapter struct {
	client *minio.Client
}

func NewMinioAdapter(client *minio.Client) *MinioAdapter {
	return &MinioAdapter{client: client}
}

func (m *MinioAdapter) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return m.client.PutObject(ctx, bucketName, objectName, reader, objectSize, opts)
}

func (m *MinioAdapter) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (MinioObject, error) {
	return m.client.GetObject(ctx, bucketName, objectName, opts)
}

func (m *MinioAdapter) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	return m.client.ListBuckets(ctx)
}
