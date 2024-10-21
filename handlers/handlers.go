package handlers

import (
	"encoding/json"
	"fmt"
	"errors"
	"hash/fnv"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"

	minio_adapter "github.com/spacelift-io/homework-object-storage/minio"
)

type Handler struct {
	minioInstances []minio_adapter.MinioInstance
	logger         *logrus.Logger
	getMinioClient func(id string) (minio_adapter.MinioClientInterface, error)
}

func NewHandler(minioInstances []minio_adapter.MinioInstance, logger *logrus.Logger) *Handler {
	h := &Handler{
		minioInstances: minioInstances,
		logger:         logger,
	}
	h.getMinioClient = h.defaultGetMinioClient
	return h
}

func (h *Handler) defaultGetMinioClient(id string) (minio_adapter.MinioClientInterface, error) {
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
	return minio_adapter.NewMinioAdapter(client), nil
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
		if strings.Contains(err.Error(), "Your previous request to create the named bucket succeeded") {
			h.logger.WithField("bucketName", req.BucketName).Info("Bucket already exists")
			http.Error(w, "Bucket already exists", http.StatusConflict)
			return
		}
		if strings.Contains(err.Error(), "Bucket name already exists") {
			h.logger.WithField("bucketName", req.BucketName).Info("Bucket name already taken")
			http.Error(w, "Bucket name already taken", http.StatusConflict)
			return
		}
		h.logger.WithError(err).Error("Failed to create bucket")
		http.Error(w, "Failed to create bucket", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Bucket created successfully"})
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
		minioErr, ok := err.(minio.ErrorResponse)
		if ok {
			switch minioErr.Code {
			case "BucketNotEmpty":
				h.logger.WithField("bucket", bucketName).Info("Attempted to delete non-empty bucket")
				http.Error(w, "The bucket you tried to delete is not empty", http.StatusConflict)
				return
			case "NoSuchBucket":
				h.logger.WithField("bucket", bucketName).Info("Attempted to delete non-existent bucket")
				http.Error(w, "The specified bucket does not exist", http.StatusNotFound)
				return
			}
		}
		
		h.logger.WithError(err).WithField("bucket", bucketName).Error("Failed to delete bucket")
		http.Error(w, "Failed to delete bucket", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) HandlePutObject(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")
	id := chi.URLParam(r, "id")

	if err := validateID(id); err != nil {
		h.logger.WithField("id", id).WithError(err).Error("Invalid ID")
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	
	h.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"id":     id,
		"method": r.Method,
		"path":   r.URL.Path,
	}).Info("Received GetObject request")

	if err := validateID(id); err != nil {
		h.logger.WithFields(logrus.Fields{
			"bucket": bucketName,
			"id":     id,
			"error":  err.Error(),
		}).Error("Invalid ID")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	minioClient, err := h.getMinioClient(bucketName)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"bucket": bucketName,
			"error":  err.Error(),
		}).Error("Failed to get MinIO client")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	exists, err := minioClient.BucketExists(r.Context(), bucketName)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"bucket": bucketName,
			"error":  err.Error(),
		}).Error("Failed to check bucket existence")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !exists {
		h.logger.WithField("bucket", bucketName).Error("Bucket does not exist")
		http.Error(w, "Bucket not found", http.StatusNotFound)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"id":     id,
	}).Info("Attempting to get object")

	object, err := minioClient.GetObject(r.Context(), bucketName, id, minio.GetObjectOptions{})
	if err != nil {
		errorResponse := minio.ToErrorResponse(err)
		h.logger.WithFields(logrus.Fields{
			"bucket":      bucketName,
			"id":          id,
			"errorCode":   errorResponse.Code,
			"errorMessage": errorResponse.Message,
		}).Error("Failed to get object")

		if errorResponse.Code == "NoSuchKey" {
			http.Error(w, "Object not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	defer object.Close()

	stat, err := object.Stat()
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"bucket": bucketName,
			"id":     id,
			"error":  err.Error(),
		}).Error("Failed to get object stats")
		http.Error(w, "Failed to get object stats", http.StatusInternalServerError)
		return
	}

	h.logger.WithFields(logrus.Fields{
		"bucket":      bucketName,
		"id":          id,
		"contentType": stat.ContentType,
		"size":        stat.Size,
	}).Info("Successfully retrieved object stats")

	w.Header().Set("Content-Type", stat.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size))

	_, err = io.Copy(w, object)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"bucket": bucketName,
			"id":     id,
			"error":  err.Error(),
		}).Error("Failed to stream object")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"id":     id,
	}).Info("Successfully streamed object")
}

func (h *Handler) HandleDeleteObject(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")
	id := chi.URLParam(r, "id")

	if err := validateID(id); err != nil {
		h.logger.WithField("id", id).WithError(err).Error("Invalid ID")
		http.Error(w, err.Error(), http.StatusBadRequest)
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

func validateID(id string) error {
	if len(id) > 32 {
		return fmt.Errorf("ID must not exceed 32 characters (current length: %d)", len(id))
	}
	if !isAlphanumeric(id) {
		return errors.New("ID must contain only alphanumeric characters")
	}
	return nil
}

func isAlphanumeric(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
