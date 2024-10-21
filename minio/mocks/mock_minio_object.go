package mocks

import (
	"io"

	minioGo "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/mock"
)

// MockMinioObject is a mock implementation of MinioObject interface
type MockMinioObject struct {
	mock.Mock
	io.Reader
}

func (m *MockMinioObject) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

func (m *MockMinioObject) Stat() (minioGo.ObjectInfo, error) {
	args := m.Called()
	return args.Get(0).(minioGo.ObjectInfo), args.Error(1)
}

func (m *MockMinioObject) Close() error {
	args := m.Called()
	return args.Error(0)
}