package web

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// apiToken holds the value of SWARMCD_API_TOKEN read at init time.
// An empty string means the mutation API is disabled.
var apiToken string

func init() {
	apiToken = os.Getenv("SWARMCD_API_TOKEN")
}

// MutationAPIEnabled reports whether a non-empty SWARMCD_API_TOKEN was
// configured, meaning the write/mutation endpoints are available.
func MutationAPIEnabled() bool {
	return apiToken != ""
}

// authMiddleware returns a Gin middleware that enforces bearer-token
// authentication on every request in the group.
//
// Behaviour matrix (see issue #10):
//
//	SWARMCD_API_TOKEN not set → 403 "mutation API is disabled"
//	Token set, missing/wrong  → 401 "invalid or missing authorization token"
//	Token set, valid           → request proceeds
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiToken == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "mutation API is disabled — set SWARMCD_API_TOKEN to enable",
			})
			return
		}

		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or missing authorization token",
			})
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(token), []byte(apiToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or missing authorization token",
			})
			return
		}

		c.Next()
	}
}
