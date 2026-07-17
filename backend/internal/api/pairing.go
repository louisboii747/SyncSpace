package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
)

type PairingService interface {
	TrustedDevices(context.Context) ([]pairing.TrustedDevice, error)
	Requests() []pairing.Request
	RequestPairing(context.Context, string) (pairing.Request, error)
	Accept(context.Context, string) (pairing.Decision, error)
	Refresh(context.Context, string) (pairing.Decision, error)
	Reject(context.Context, string) (pairing.Request, error)
	ReceiveBegin(context.Context, pairing.BeginRequest, string) (pairing.BeginResponse, error)
	ReceiveProof(context.Context, pairing.Proof) (pairing.PeerDecision, error)
	RemoveTrustedDevice(context.Context, string) (pairing.TrustedDevice, error)
	SetBlocked(context.Context, string, bool) (pairing.TrustedDevice, error)
}

type pairingRequestBody struct {
	DeviceID string `json:"deviceId" binding:"required"`
}
type pairingDecisionBody struct {
	RequestID string `json:"requestId" binding:"required"`
}

func registerPairingRoutes(router *gin.Engine, service PairingService, socket gin.HandlerFunc, logger *slog.Logger) {
	local := router.Group("/pairing", localOnly())
	local.GET("/trusted-devices", func(c *gin.Context) {
		devices, err := service.TrustedDevices(c.Request.Context())
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, devices)
	})
	local.GET("/requests", func(c *gin.Context) { c.JSON(http.StatusOK, service.Requests()) })
	local.GET("/requests/:id", func(c *gin.Context) {
		decision, err := service.Refresh(c.Request.Context(), c.Param("id"))
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, decision)
	})
	local.POST("/request", func(c *gin.Context) {
		var body pairingRequestBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId is required"})
			return
		}
		request, err := service.RequestPairing(c.Request.Context(), body.DeviceID)
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusAccepted, request)
	})
	local.POST("/accept", func(c *gin.Context) {
		var body pairingDecisionBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "requestId is required"})
			return
		}
		decision, err := service.Accept(c.Request.Context(), body.RequestID)
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, decision)
	})
	local.POST("/reject", func(c *gin.Context) {
		var body pairingDecisionBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "requestId is required"})
			return
		}
		request, err := service.Reject(c.Request.Context(), body.RequestID)
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, request)
	})
	local.POST("/trusted-devices/:deviceId/block", func(c *gin.Context) {
		device, err := service.SetBlocked(c.Request.Context(), c.Param("deviceId"), true)
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, device)
	})
	local.POST("/trusted-devices/:deviceId/unblock", func(c *gin.Context) {
		device, err := service.SetBlocked(c.Request.Context(), c.Param("deviceId"), false)
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, device)
	})
	local.DELETE("/trusted-devices/:deviceId", func(c *gin.Context) {
		if _, err := service.RemoveTrustedDevice(c.Request.Context(), c.Param("deviceId")); err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/ws/pairing", localOnly(), socket)

	peer := router.Group("/v1/pairing")
	peer.POST("/requests", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var body pairing.BeginRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pairing request"})
			return
		}
		response, err := service.ReceiveBegin(c.Request.Context(), body, remoteHost(c.Request.RemoteAddr))
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusAccepted, response)
	})
	peer.POST("/proof", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		var body pairing.Proof
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pairing proof"})
			return
		}
		response, err := service.ReceiveProof(c.Request.Context(), body)
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, response)
	})
}

func localOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isLoopbackRequest(c.Request.RemoteAddr) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "local management is available only from this device"})
			return
		}
		if origin := c.GetHeader("Origin"); origin != "" && !sameLocalOrigin(origin, c.Request.Host) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-origin local API requests are not allowed"})
			return
		}
		if site := c.GetHeader("Sec-Fetch-Site"); site == "cross-site" && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-site local API requests are not allowed"})
			return
		}
		c.Next()
	}
}

func isLoopbackRequest(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func sameLocalOrigin(origin, requestHost string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	return strings.EqualFold(parsed.Host, requestHost)
}

func writePairingError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, pairing.ErrInvalidIdentifier), errors.Is(err, pairing.ErrProtocol):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pairing message"})
	case errors.Is(err, pairing.ErrPeerNotDiscovered), errors.Is(err, pairing.ErrRequestNotFound), errors.Is(err, pairing.ErrTrustedDeviceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, pairing.ErrAlreadyTrusted), errors.Is(err, pairing.ErrIdentityChanged):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, pairing.ErrPairingRateLimit):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
	default:
		logger.Error("Pairing API error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
