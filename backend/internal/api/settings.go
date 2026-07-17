package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/settings"
)

func registerSettingsRoutes(router *gin.Engine, store *settings.Store) {
	if store == nil {
		return
	}
	local := router.Group("/settings", localOnly())
	local.GET("", func(c *gin.Context) { c.JSON(http.StatusOK, store.Get()) })
	local.PUT("", func(c *gin.Context) {
		var values settings.Values
		if err := c.ShouldBindJSON(&values); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings document"})
			return
		}
		updated, err := store.Update(values)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, updated)
	})
	local.POST("/reset", func(c *gin.Context) {
		updated, err := store.Update(settings.Defaults())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reset settings"})
			return
		}
		c.JSON(http.StatusOK, updated)
	})
}
