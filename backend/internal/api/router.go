// Package api exposes SyncSpace application services over HTTP.
package api

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/diagnostics"
	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/settings"
)

// DiscoveryService is the API-facing discovery application contract.
type DiscoveryService interface {
	Devices() []models.Device
	Self() models.Device
	Refresh()
}

// RouterConfig supplies HTTP dependencies.
type RouterConfig struct {
	Discovery       DiscoveryService
	DiscoverySocket gin.HandlerFunc
	Pairing         PairingService
	PairingSocket   gin.HandlerFunc
	Transfer        TransferService
	TransferSocket  gin.HandlerFunc
	Diagnostics     *diagnostics.Service
	Simulator       SimulatorService
	Settings        *settings.Store
	SettingsChanged func(settings.Values)
	Frontend        http.Handler
	Logger          *slog.Logger
}

// NewRouter creates the SyncSpace HTTP router.
func NewRouter(config RouterConfig) *gin.Engine {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(requestLogger(logger), recovery(logger), privacyGate(config.Settings))

	router.GET("/devices", localOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, config.Discovery.Devices())
	})
	router.GET("/device/self", localOnly(), func(c *gin.Context) {
		c.JSON(http.StatusOK, config.Discovery.Self())
	})
	router.POST("/discovery/refresh", localOnly(), func(c *gin.Context) {
		config.Discovery.Refresh()
		c.JSON(http.StatusAccepted, gin.H{"status": "refresh_requested"})
	})
	router.GET("/ws/discovery", localOnly(), config.DiscoverySocket)
	registerPairingRoutes(router, config.Pairing, config.PairingSocket, logger)
	if config.Transfer != nil {
		registerTransferRoutes(router, config.Transfer, config.TransferSocket, config.Settings, logger)
	}
	registerDiagnosticsRoutes(router, config.Diagnostics, config.Simulator, config.Pairing)
	registerSettingsRoutes(router, config.Settings, config.SettingsChanged)
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/") && !c.GetBool("syncspace_api_reroute") {
			if !isLoopbackRequest(c.Request.RemoteAddr) {
				c.JSON(http.StatusForbidden, gin.H{"error": "local management is available only from this device"})
				return
			}
			c.Set("syncspace_api_reroute", true)
			c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, "/api/v1")
			router.HandleContext(c)
			return
		}
		if c.GetBool("syncspace_api_reroute") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API route not found"})
			return
		}
		if config.Frontend == nil {
			c.Status(http.StatusNotFound)
			return
		}
		localOnly()(c)
		if c.IsAborted() {
			return
		}
		gin.WrapH(config.Frontend)(c)
	})
	return router
}

// privacyGate keeps all peer-facing pairing and transfer routes closed until
// the current policy has been accepted. The local UI and management API remain
// available so the policy can be read and accepted safely.
func privacyGate(store *settings.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store != nil && !store.PrivacyAccepted() {
			path := strings.TrimPrefix(c.Request.URL.Path, "/api/v1")
			apiRequest := strings.HasPrefix(c.Request.URL.Path, "/api/v1/") || strings.HasPrefix(path, "/v1/") ||
				path == "/devices" || strings.HasPrefix(path, "/discovery/") || strings.HasPrefix(path, "/pairing/") ||
				path == "/transfers" || strings.HasPrefix(path, "/transfers/") || strings.HasPrefix(path, "/ws/") ||
				path == "/settings" || strings.HasPrefix(path, "/settings/") || path == "/privacy-policy" || strings.HasPrefix(path, "/privacy-policy/") ||
				strings.HasPrefix(path, "/diagnostics") || strings.HasPrefix(path, "/dev/")
			allowed := (c.Request.Method == http.MethodGet && (path == "/privacy-policy" || path == "/settings" || path == "/device/self" || path == "/health")) ||
				(c.Request.Method == http.MethodPost && path == "/privacy-policy/accept")
			if apiRequest && !allowed {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "accept the current privacy policy before using discovery, pairing, or transfers"})
				return
			}
		}
		c.Next()
	}
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("HTTP request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(started).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

func recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("HTTP panic", "error", recovered, "stack", string(debug.Stack()))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}
