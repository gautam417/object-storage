package handlers

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
	"github.com/spacelift-io/homework-object-storage/storage"
)

type Handler struct {
	minioInstances []storage.MinioInstance
	logger         *logrus.Logger
	getMinioClient func(id string) (MinioClientInterface, error)
}

func NewHandler(minioInstances []storage.MinioInstance, logger *logrus.Logger) *Handler {
	h := &Handler{
		minioInstances: minioInstances,
		logger:         logger,
	}
	h.getMinioClient = h.defaultGetMinioClient
	return h
}

func (h *Handler) defaultGetMinioClient(id string) (MinioClientInterface, error) {
	hash := fnv.New32a()
	hash.Write([]byte(id))
	index := int(hash.Sum32()) % len(h.minioInstances)
	instance := h.minioInstances[index]

	client, err := minio.New(instance.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(instance.AccessKey, instance.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}
	return NewMinioAdapter(client), nil
}

func (h *Handler) HandleCreateBucket(w http.ResponseWriter, r *http.Request) {
    var req struct {
        BucketName string `json:"bucketName"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        h.logger.WithError(err).Error("Failed to decode request")
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    minioClient, err := h.getMinioClient(req.BucketName)
    if err != nil {
        h.logger.WithError(err).Error("Failed to get MinIO client")
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    err = minioClient.MakeBucket(r.Context(), req.BucketName, minio.MakeBucketOptions{})
    if err != nil {
        h.logger.WithError(err).Error("Failed to create bucket")
        http.Error(w, "Failed to create bucket", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
}

func (h *Handler) HandleDeleteBucket(w http.ResponseWriter, r *http.Request) {
    bucketName := chi.URLParam(r, "bucketName")

    minioClient, err := h.getMinioClient(bucketName)
    if err != nil {
        h.logger.WithError(err).Error("Failed to get MinIO client")
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    err = minioClient.RemoveBucket(r.Context(), bucketName)
    if err != nil {
        h.logger.WithError(err).Error("Failed to delete bucket")
        http.Error(w, "Failed to delete bucket", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HandlePutObject(w http.ResponseWriter, r *http.Request) {
    bucketName := chi.URLParam(r, "bucketName")
    id := chi.URLParam(r, "id")
    if len(id) > 32 || !isAlphanumeric(id) {
        h.logger.WithField("id", id).Error("Invalid ID")
        http.Error(w, "Invalid ID", http.StatusBadRequest)
        return
    }

    minioClient, err := h.getMinioClient(bucketName)
    if err != nil {
        h.logger.WithError(err).Error("Failed to get MinIO client")
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    exists, err := minioClient.BucketExists(r.Context(), bucketName)
    if err != nil {
        h.logger.WithError(err).Error("Failed to check bucket existence")
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
    if !exists {
        h.logger.WithField("bucket", bucketName).Error("Bucket does not exist")
        http.Error(w, "Bucket not found", http.StatusNotFound)
        return
    }

    _, err = minioClient.PutObject(r.Context(), bucketName, id, r.Body, -1, minio.PutObjectOptions{})
    if err != nil {
        h.logger.WithError(err).Error("Failed to put object")
        http.Error(w, "Failed to store object", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleGetObject(w http.ResponseWriter, r *http.Request) {
    bucketName := chi.URLParam(r, "bucketName")
    id := chi.URLParam(r, "id")
    if len(id) > 32 || !isAlphanumeric(id) {
        h.logger.WithField("id", id).Error("Invalid ID")
        http.Error(w, "Invalid ID", http.StatusBadRequest)
        return
    }

    minioClient, err := h.getMinioClient(bucketName)
    if err != nil {
        h.logger.WithError(err).Error("Failed to get MinIO client")
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }

    exists, err := minioClient.BucketExists(r.Context(), bucketName)
    if err != nil {
        h.logger.WithError(err).Error("Failed to check bucket existence")
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
    if !exists {
        h.logger.WithField("bucket", bucketName).Error("Bucket does not exist")
        http.Error(w, "Bucket not found", http.StatusNotFound)
        return
    }

	object, err := minioClient.GetObject(r.Context(), bucketName, id, minio.GetObjectOptions{})
    if err != nil {
        if minio.ToErrorResponse(err).Code == "NoSuchKey" {
            h.logger.WithError(err).Error("Object not found")
            http.Error(w, "Object not found", http.StatusNotFound)
        } else {
            h.logger.WithError(err).Error("Failed to get object")
            http.Error(w, "Internal server error", http.StatusInternalServerError)
        }
        return
    }
    defer object.Close()

    stat, err := object.Stat()
    if err != nil {
        h.logger.WithError(err).Error("Failed to get object stats")
        http.Error(w, "Failed to get object stats", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", stat.ContentType)
    w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size))

    _, err = io.Copy(w, object)
    if err != nil {
        h.logger.WithError(err).Error("Failed to stream object")
        http.Error(w, "Failed to retrieve object", http.StatusInternalServerError)
        return
    }
}

func (h *Handler) HandleDeleteObject(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")
	id := chi.URLParam(r, "id")

	if len(id) > 32 || !isAlphanumeric(id) {
		h.logger.WithField("id", id).Error("Invalid ID")
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	minioClient, err := h.getMinioClient(bucketName)
	if err != nil {
		h.logger.WithError(err).Error("Failed to get MinIO client")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	exists, err := minioClient.BucketExists(r.Context(), bucketName)
    if err != nil {
        h.logger.WithError(err).Error("Failed to check bucket existence")
        http.Error(w, "Internal server error", http.StatusInternalServerError)
        return
    }
    if !exists {
        h.logger.WithField("bucket", bucketName).Error("Bucket does not exist")
        http.Error(w, "Bucket not found", http.StatusNotFound)
        return
    }

	err = minioClient.RemoveObject(r.Context(), bucketName, id, minio.RemoveObjectOptions{})
    if err != nil {
        if minio.ToErrorResponse(err).Code == "NoSuchKey" {
            h.logger.WithError(err).WithFields(logrus.Fields{
                "bucket": bucketName,
                "id":     id,
            }).Error("Object not found")
            http.Error(w, "Object not found", http.StatusNotFound)
        } else {
            h.logger.WithError(err).WithFields(logrus.Fields{
                "bucket": bucketName,
                "id":     id,
            }).Error("Failed to delete object")
            http.Error(w, "Failed to delete object", http.StatusInternalServerError)
        }
        return
    }

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func isAlphanumeric(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
