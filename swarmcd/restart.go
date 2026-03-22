package swarmcd

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
)

// ServiceAPI is the subset of the Docker Engine API needed for restart operations.
// It is satisfied by dockerCli.Client() and can be replaced with a mock in tests.
type ServiceAPI interface {
	ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error)
	ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options types.ServiceUpdateOptions) (swarm.ServiceUpdateResponse, error)
}

// GetDockerServiceAPI returns the Docker Engine API client for service operations.
func GetDockerServiceAPI() ServiceAPI {
	return dockerCli.Client()
}

// StackExists reports whether a stack with the given name is managed by SwarmCD.
func StackExists(name string) bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	_, ok := stackStatus[name]
	return ok
}

// GetStackNames returns the names of all managed stacks.
func GetStackNames() []string {
	stateMu.RLock()
	defer stateMu.RUnlock()
	names := make([]string, 0, len(stackStatus))
	for name := range stackStatus {
		names = append(names, name)
	}
	return names
}
