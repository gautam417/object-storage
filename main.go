package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sirupsen/logrus"
	"github.com/spacelift-io/homework-object-storage/docker_discovery"
	"github.com/spacelift-io/homework-object-storage/handlers"
	customMiddleware "github.com/spacelift-io/homework-object-storage/middleware"
	"golang.org/x/time/rate"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)

	minioInstances, err := docker_discovery.DiscoverMinioInstances()
	if err != nil {
		logger.WithError(err).Fatal("Failed to discover MinIO instances")
	}

	logger.Infof("Discovered %d MinIO instances", len(minioInstances))

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(customMiddleware.RateLimiter(rate.Limit(100), 50))
	h := handlers.NewHandler(minioInstances, logger)
	
	r.Get("/healthz", h.HandleHealthCheck)

	r.Route("/buckets", func(r chi.Router) {
		r.Post("/", h.HandleCreateBucket)
		r.Delete("/{bucketName}", h.HandleDeleteBucket)
		r.Route("/{bucketName}/objects", func(r chi.Router) {
			r.Put("/{id}", h.HandlePutObject)
			r.Get("/{id}", h.HandleGetObject)
			r.Delete("/{id}", h.HandleDeleteObject)
		})
	})

	srv := &http.Server{
		Addr:    ":3000",
		Handler: r,
	}

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		logger.Info("Shutting down server")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.WithError(err).Error("Server shutdown error")
		}
	}()

	logger.Info("Starting server on :3000")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.WithError(err).Fatal("Server error")
	}
}
