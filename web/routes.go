package web

import (
	"github.com/gin-gonic/gin"
	"github.com/m-adawi/swarm-cd/util"
	"github.com/pkg/errors"
	sloggin "github.com/samber/slog-gin"
)

var router *gin.Engine = gin.New()

func init() {
	router.Use(sloggin.New(util.Logger))

	// Read endpoints — no auth
	router.GET("/stacks", getStacks)
	router.GET("/health", getHealth)

	// Static UI assets
	router.StaticFile("/ui", "ui/index.html")
	router.Static("/assets", "ui/assets")
	router.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/ui")
	})

	// Write endpoints — auth required
	write := router.Group("/")
	write.Use(authMiddleware())
	// Write routes will be registered here by later issues
	// (e.g. PATCH /stacks/:name, POST /stacks/:name/restart, etc.)
	_ = write
}

func RunServer(address string) error {
	if err := router.Run(address); err != nil {
		util.Logger.Error("router run", "address", address)
		return errors.Wrap(err, "router run")
	}
	return nil
}
