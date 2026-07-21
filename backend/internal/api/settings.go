package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/settings"
)

func registerSettingsRoutes(router *gin.Engine, store *settings.Store, onChange func(settings.Values)) {
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
		if onChange != nil {
			onChange(updated)
		}
		c.JSON(http.StatusOK, updated)
	})
	local.POST("/reset", func(c *gin.Context) {
		defaults := settings.Defaults()
		defaults.DeviceName = store.Get().DeviceName
		updated, err := store.Update(defaults)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to reset settings"})
			return
		}
		if onChange != nil {
			onChange(updated)
		}
		c.JSON(http.StatusOK, updated)
	})

	privacy := router.Group("/privacy-policy", localOnly())
	privacy.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, settings.Policy(store.Get()))
	})
	privacy.POST("/accept", func(c *gin.Context) {
		var body struct {
			Version string `json:"version"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Version == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "privacy policy version is required"})
			return
		}
		updated, err := store.AcceptPrivacy(body.Version, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if onChange != nil {
			onChange(updated)
		}
		c.JSON(http.StatusOK, settings.Policy(updated))
	})
}
