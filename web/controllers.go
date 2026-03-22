package web

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/m-adawi/swarm-cd/swarmcd"
	"github.com/m-adawi/swarm-cd/util"
)

type stackResponse struct {
	Name           string     `json:"name"`
	RepoURL        string     `json:"repo_url"`
	RefType        string     `json:"ref_type"`
	RefValue       string     `json:"ref_value"`
	Revision       string     `json:"revision"`
	ComposeFile    string     `json:"compose_file"`
	Error          string     `json:"error"`
	LastChangeAt   *time.Time `json:"last_change_at"`
	LastDeployedAt *time.Time `json:"last_deployed_at"`
}

func getHealth(ctx *gin.Context) {
	info := swarmcd.GetRuntimeInfo()
	stacksStatus := swarmcd.GetStackStatus()
	uptime := time.Since(info.BootedAt).Seconds()

	ctx.JSON(http.StatusOK, gin.H{
		"status":                  "healthy",
		"booted_at":               info.BootedAt,
		"version":                 info.Version,
		"uptime_seconds":          math.Floor(uptime),
		"update_interval_seconds": util.Configs.UpdateInterval,
		"stacks_managed":          len(stacksStatus),
		"mutation_api_enabled":    MutationAPIEnabled(),
	})
}

func getStack(ctx *gin.Context) {
	name := ctx.Param("name")
	stacksStatus := swarmcd.GetStackStatus()
	v, ok := stacksStatus[name]
	if !ok {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("stack '%s' not found", name),
		})
		return
	}
	ctx.JSON(http.StatusOK, stackResponse{
		Name:           name,
		RepoURL:        v.RepoURL,
		RefType:        v.RefType,
		RefValue:       v.RefValue,
		Revision:       v.Revision,
		ComposeFile:    v.ComposeFile,
		Error:          v.Error,
		LastChangeAt:   v.LastChangeAt,
		LastDeployedAt: v.LastDeployedAt,
	})
}

func getStacks(ctx *gin.Context) {
	stacksStatus := swarmcd.GetStackStatus()
	var stacks []stackResponse
	for k, v := range stacksStatus {
		stacks = append(stacks, stackResponse{
			Name:           k,
			RepoURL:        v.RepoURL,
			RefType:        v.RefType,
			RefValue:       v.RefValue,
			Revision:       v.Revision,
			ComposeFile:    v.ComposeFile,
			Error:          v.Error,
			LastChangeAt:   v.LastChangeAt,
			LastDeployedAt: v.LastDeployedAt,
		})
	}
	sort.Slice(stacks, func(i, j int) bool {
		return stacks[i].Name < stacks[j].Name
	})
	ctx.JSON(http.StatusOK, stacks)
}

func patchStack(ctx *gin.Context) {
	name := ctx.Param("name")

	var req swarmcd.PatchRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	result, err := swarmcd.PatchStack(name, req)
	if err != nil {
		var valErr *swarmcd.ValidationError
		var notFoundErr *swarmcd.NotFoundError
		if errors.As(err, &valErr) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": valErr.Msg})
			return
		}
		if errors.As(err, &notFoundErr) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": notFoundErr.Msg})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := gin.H{
		"stack": stackResponse{
			Name:           name,
			RepoURL:        result.Status.RepoURL,
			RefType:        result.Status.RefType,
			RefValue:       result.Status.RefValue,
			Revision:       result.Status.Revision,
			ComposeFile:    result.Status.ComposeFile,
			Error:          result.Status.Error,
			LastChangeAt:   result.Status.LastChangeAt,
			LastDeployedAt: result.Status.LastDeployedAt,
		},
	}
	if result.Warning != "" {
		resp["warning"] = result.Warning
	}

	ctx.JSON(http.StatusOK, resp)
}
