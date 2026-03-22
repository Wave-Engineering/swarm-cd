package web

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/m-adawi/swarm-cd/swarmcd"
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
	ctx.JSON(http.StatusOK, gin.H{
		"status":               "ok",
		"mutation_api_enabled": MutationAPIEnabled(),
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
