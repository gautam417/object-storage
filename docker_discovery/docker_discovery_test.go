package docker_discovery

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	minio_adapter "github.com/spacelift-io/homework-object-storage/minio"
	"github.com/spacelift-io/homework-object-storage/docker_discovery/mocks"
)

func TestDiscoverMinioInstances(t *testing.T) {
	mockClient := new(mocks.MockDockerClient)

	mockContainers := []types.Container{
		{
			ID:    "container1",
			Names: []string{"/amazin-object-storage-node-1"},
		},
		{
			ID:    "container2",
			Names: []string{"/amazin-object-storage-node-2"},
		},
		{
			ID:    "container3",
			Names: []string{"/some-other-container"},
		},
	}
	mockClient.On("ContainerList", mock.Anything, mock.Anything).Return(mockContainers, nil)

	mockInspect1 := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: "container1",
		},
		Config: &container.Config{
			Env: []string{
				"MINIO_ACCESS_KEY=access1",
				"MINIO_SECRET_KEY=secret1",
			},
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"bridge": {
					IPAddress: "172.17.0.2",
				},
			},
		},
	}
	mockInspect2 := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: "container2",
		},
		Config: &container.Config{
			Env: []string{
				"MINIO_ACCESS_KEY=access2",
				"MINIO_SECRET_KEY=secret2",
			},
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"bridge": {
					IPAddress: "172.17.0.3",
				},
			},
		},
	}
	mockClient.On("ContainerInspect", mock.Anything, "container1").Return(mockInspect1, nil)
	mockClient.On("ContainerInspect", mock.Anything, "container2").Return(mockInspect2, nil)

	mockClient.On("Close").Return(nil)

	oldNewDockerClient := newDockerClient
	newDockerClient = func() (DockerClient, error) {
		return mockClient, nil
	}
	defer func() { newDockerClient = oldNewDockerClient }()

	instances, err := DiscoverMinioInstances()

	assert.NoError(t, err)
	assert.Len(t, instances, 2)
	assert.Equal(t, "172.17.0.2:9000", instances[0].Endpoint)
	assert.Equal(t, "access1", instances[0].AccessKey)
	assert.Equal(t, "secret1", instances[0].SecretKey)
	assert.Equal(t, "172.17.0.3:9000", instances[1].Endpoint)
	assert.Equal(t, "access2", instances[1].AccessKey)
	assert.Equal(t, "secret2", instances[1].SecretKey)

	mockClient.AssertExpectations(t)
}

func TestDiscoverMinioInstances_NoInstancesFound(t *testing.T) {
	mockClient := new(mocks.MockDockerClient)

	mockContainers := []types.Container{
		{
			ID:    "container3",
			Names: []string{"/some-other-container"},
		},
	}
	mockClient.On("ContainerList", mock.Anything, mock.Anything).Return(mockContainers, nil)
	mockClient.On("Close").Return(nil)

	oldNewDockerClient := newDockerClient
	newDockerClient = func() (DockerClient, error) {
		return mockClient, nil
	}
	defer func() { newDockerClient = oldNewDockerClient }()

	instances, err := DiscoverMinioInstances()

	assert.Error(t, err)
	assert.Nil(t, instances)
	assert.Equal(t, "no MinIO instances found", err.Error())

	mockClient.AssertExpectations(t)
}

func TestGetMinioInstanceInfo(t *testing.T) {
	mockClient := new(mocks.MockDockerClient)

	mockInspect := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: "container1",
		},
		Config: &container.Config{
			Env: []string{
				"MINIO_ACCESS_KEY=access1",
				"MINIO_SECRET_KEY=secret1",
			},
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"bridge": {
					IPAddress: "172.17.0.2",
				},
			},
		},
	}
	mockClient.On("ContainerInspect", mock.Anything, "container1").Return(mockInspect, nil)

	instance, err := getMinioInstanceInfo(mockClient, "container1")

	assert.NoError(t, err)
	assert.Equal(t, "172.17.0.2:9000", instance.Endpoint)
	assert.Equal(t, "access1", instance.AccessKey)
	assert.Equal(t, "secret1", instance.SecretKey)

	mockClient.AssertExpectations(t)
}

func TestGetMinioInstanceInfo_NoIP(t *testing.T) {
	mockClient := new(mocks.MockDockerClient)

	mockInspect := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: "container1",
		},
		Config: &container.Config{
			Env: []string{
				"MINIO_ACCESS_KEY=access1",
				"MINIO_SECRET_KEY=secret1",
			},
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{},
		},
	}
	mockClient.On("ContainerInspect", mock.Anything, "container1").Return(mockInspect, nil)

	instance, err := getMinioInstanceInfo(mockClient, "container1")

	assert.Error(t, err)
	assert.Equal(t, minio_adapter.MinioInstance{}, instance)
	assert.Equal(t, "failed to get container IP", err.Error())

	mockClient.AssertExpectations(t)
}
