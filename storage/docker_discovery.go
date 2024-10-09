package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type MinioInstance struct {
	Endpoint  string
	AccessKey string
	SecretKey string
}

// DockerClient interface for the methods we need
type DockerClient interface {
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	Close() error
}

var newDockerClient = func() (DockerClient, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}

func DiscoverMinioInstances() ([]MinioInstance, error) {
	cli, err := newDockerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var instances []MinioInstance
	for _, container := range containers {
		if strings.Contains(strings.Join(container.Names, " "), "amazin-object-storage-node") {
			instance, err := getMinioInstanceInfo(cli, container.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get MinIO instance info: %w", err)
			}
			instances = append(instances, instance)
		}
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no MinIO instances found")
	}

	return instances, nil
}

func getMinioInstanceInfo(cli DockerClient, containerID string) (MinioInstance, error) {
	inspect, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return MinioInstance{}, fmt.Errorf("failed to inspect container: %w", err)
	}

	var ip string
	for _, network := range inspect.NetworkSettings.Networks {
		ip = network.IPAddress
		break
	}

	if ip == "" {
		return MinioInstance{}, fmt.Errorf("failed to get container IP")
	}

	var accessKey, secretKey string
	for _, env := range inspect.Config.Env {
		if strings.HasPrefix(env, "MINIO_ACCESS_KEY=") {
			accessKey = strings.TrimPrefix(env, "MINIO_ACCESS_KEY=")
		} else if strings.HasPrefix(env, "MINIO_SECRET_KEY=") {
			secretKey = strings.TrimPrefix(env, "MINIO_SECRET_KEY=")
		}
	}

	if accessKey == "" || secretKey == "" {
		return MinioInstance{}, fmt.Errorf("failed to get MinIO credentials")
	}

	return MinioInstance{
		Endpoint:  fmt.Sprintf("%s:9000", ip),
		AccessKey: accessKey,
		SecretKey: secretKey,
	}, nil
}