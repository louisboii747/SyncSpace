package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
)

// PairingService is the API-facing trust application contract.
type PairingService interface {
	TrustedDevices(context.Context) ([]pairing.TrustedDevice, error)
	RequestPairing(context.Context, string) (pairing.Request, error)
	Accept(context.Context, string) (pairing.TrustedDevice, error)
	Reject(string) (pairing.Request, error)
	RemoveTrustedDevice(context.Context, string) (pairing.TrustedDevice, error)
}

type pairingRequestBody struct {
	DeviceID string `json:"deviceId" binding:"required"`
}

type pairingDecisionBody struct {
	RequestID string `json:"requestId" binding:"required"`
}

func registerPairingRoutes(router *gin.Engine, service PairingService, socket gin.HandlerFunc, logger *slog.Logger) {
	pairingRoutes := router.Group("/pairing", localOnly())
	pairingRoutes.GET("/trusted-devices", func(c *gin.Context) {
		devices, err := service.TrustedDevices(c.Request.Context())
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, devices)
	})
	pairingRoutes.POST("/request", func(c *gin.Context) {
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
	pairingRoutes.POST("/accept", func(c *gin.Context) {
		var body pairingDecisionBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "requestId is required"})
			return
		}
		device, err := service.Accept(c.Request.Context(), body.RequestID)
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, device)
	})
	pairingRoutes.POST("/reject", func(c *gin.Context) {
		var body pairingDecisionBody
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "requestId is required"})
			return
		}
		request, err := service.Reject(body.RequestID)
		if err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, request)
	})
	pairingRoutes.DELETE("/trusted-devices/:deviceId", func(c *gin.Context) {
		if _, err := service.RemoveTrustedDevice(c.Request.Context(), c.Param("deviceId")); err != nil {
			writePairingError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/ws/pairing", localOnly(), socket)
}

func localOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			host = c.Request.RemoteAddr
		}
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if ip == nil || !ip.IsLoopback() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "pairing management is available only from this device"})
			return
		}
		c.Next()
	}
}

func writePairingError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, pairing.ErrInvalidIdentifier):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, pairing.ErrPeerNotDiscovered),
		errors.Is(err, pairing.ErrRequestNotFound),
		errors.Is(err, pairing.ErrTrustedDeviceNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, pairing.ErrAlreadyTrusted):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		logger.Error("Pairing API error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
