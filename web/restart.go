package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/gin-gonic/gin"
	"github.com/m-adawi/swarm-cd/swarmcd"
)

// dockerServiceAPI is the Docker Engine API client used by restart controllers.
// It is lazy-initialized on first use and can be overridden in tests.
var (
	dockerServiceAPI     swarmcd.ServiceAPI
	dockerServiceAPIOnce sync.Once
)

// getDockerServiceAPI returns the Docker service API client, initializing it
// from the swarmcd package on first call. Thread-safe via sync.Once.
func getDockerServiceAPI() swarmcd.ServiceAPI {
	dockerServiceAPIOnce.Do(func() {
		if dockerServiceAPI == nil {
			dockerServiceAPI = swarmcd.GetDockerServiceAPI()
		}
	})
	return dockerServiceAPI
}

// restartStackServices lists all services in a stack by label and force-updates each one.
// Returns the count of services restarted.
func restartStackServices(ctx context.Context, api swarmcd.ServiceAPI, stackName string) (int, error) {
	services, err := api.ServiceList(ctx, types.ServiceListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "com.docker.stack.namespace="+stackName)),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list services for stack %s: %w", stackName, err)
	}

	for _, svc := range services {
		svc.Spec.TaskTemplate.ForceUpdate++
		_, err := api.ServiceUpdate(ctx, svc.ID, svc.Version, svc.Spec, types.ServiceUpdateOptions{})
		if err != nil {
			return 0, fmt.Errorf("failed to update service %s: %w", svc.Spec.Name, err)
		}
	}

	return len(services), nil
}

// restartStack handles POST /stacks/:name/restart
func restartStack(c *gin.Context) {
	stackName := c.Param("name")

	if !swarmcd.StackExists(stackName) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("stack '%s' not found", stackName)})
		return
	}

	api := getDockerServiceAPI()
	count, err := restartStackServices(c.Request.Context(), api, stackName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            fmt.Sprintf("restart initiated for stack '%s'", stackName),
		"services_restarted": count,
	})
}

// restartService handles POST /stacks/:name/services/:service/restart
func restartService(c *gin.Context) {
	stackName := c.Param("name")
	serviceName := c.Param("service")

	if !swarmcd.StackExists(stackName) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("stack '%s' not found", stackName)})
		return
	}

	// Resolve service name: if it doesn't start with the stack prefix, prepend it
	fullServiceName := serviceName
	if !strings.HasPrefix(serviceName, stackName+"_") {
		fullServiceName = stackName + "_" + serviceName
	}

	api := getDockerServiceAPI()

	// List services in the stack and find the one matching
	services, err := api.ServiceList(c.Request.Context(), types.ServiceListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "com.docker.stack.namespace="+stackName)),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to list services for stack %s: %s", stackName, err)})
		return
	}

	var target *swarm.Service
	for i := range services {
		if services[i].Spec.Name == fullServiceName {
			target = &services[i]
			break
		}
	}

	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("service '%s' not found in stack '%s'", fullServiceName, stackName)})
		return
	}

	target.Spec.TaskTemplate.ForceUpdate++
	_, err = api.ServiceUpdate(c.Request.Context(), target.ID, target.Version, target.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to update service %s: %s", fullServiceName, err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("restart initiated for service '%s'", fullServiceName),
	})
}

// restartAll handles POST /restart
func restartAll(c *gin.Context) {
	api := getDockerServiceAPI()
	stackNames := swarmcd.GetStackNames()

	totalServices := 0
	for _, name := range stackNames {
		count, err := restartStackServices(c.Request.Context(), api, name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		totalServices += count
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "restart initiated for all stacks",
		"stacks_restarted":   len(stackNames),
		"services_restarted": totalServices,
	})
}
