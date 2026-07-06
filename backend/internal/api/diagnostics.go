package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/devsim"
	"github.com/louisboii747/syncspace/backend/internal/diagnostics"
)

type SimulatorService interface {
	Enabled() bool
	List() []devsim.Peer
	Simulate(context.Context, string, devsim.Scenario) (devsim.Peer, error)
	Remove(string) error
}

func registerDiagnosticsRoutes(router *gin.Engine, service *diagnostics.Service, simulator SimulatorService, pairing PairingService) {
	if service == nil {
		return
	}
	router.GET("/health", localOnly(), func(c *gin.Context) {
		health := service.Health(c.Request.Context())
		status := http.StatusOK
		if health.Status != "ok" {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, health)
	})
	router.GET("/diagnostics", localOnly(), func(c *gin.Context) { c.JSON(http.StatusOK, service.Snapshot(c.Request.Context())) })
	router.GET("/diagnostics/export", localOnly(), func(c *gin.Context) {
		contents, err := service.Export(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=syncspace-diagnostics-%s.zip", time.Now().UTC().Format("20060102-150405")))
		c.Data(http.StatusOK, "application/zip", contents)
	})
	router.POST("/diagnostics/test-transfer", localOnly(), func(c *gin.Context) {
		result, err := service.QueueTestTransfer(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, result)
	})
	router.POST("/diagnostics/clear", localOnly(), func(c *gin.Context) {
		if err := service.ClearTestData(c.Request.Context()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if simulator != nil && simulator.Enabled() {
			for _, peer := range simulator.List() {
				if pairing != nil {
					_, _ = pairing.RemoveTrustedDevice(c.Request.Context(), peer.Device.ID)
				}
				_ = simulator.Remove(peer.Device.ID)
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "test_data_cleared"})
	})
	if simulator == nil {
		return
	}
	router.GET("/dev/simulated-devices", localOnly(), func(c *gin.Context) { c.JSON(http.StatusOK, simulator.List()) })
	router.POST("/dev/simulated-devices", localOnly(), func(c *gin.Context) {
		if !simulator.Enabled() {
			c.JSON(http.StatusNotFound, gin.H{"error": "developer mode is disabled"})
			return
		}
		var body struct {
			Name     string          `json:"name"`
			Scenario devsim.Scenario `json:"scenario"`
			Trusted  bool            `json:"trusted"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		peer, err := simulator.Simulate(c.Request.Context(), body.Name, body.Scenario)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if body.Trusted || body.Scenario == devsim.ScenarioTrusted {
			request, requestErr := pairing.RequestPairing(c.Request.Context(), peer.Device.ID)
			if requestErr == nil {
				_, requestErr = pairing.Accept(c.Request.Context(), request.RequestID)
			}
			if requestErr != nil {
				_ = simulator.Remove(peer.Device.ID)
				c.JSON(http.StatusInternalServerError, gin.H{"error": requestErr.Error()})
				return
			}
		}
		c.JSON(http.StatusCreated, peer)
	})
	router.DELETE("/dev/simulated-devices/:id", localOnly(), func(c *gin.Context) {
		if err := simulator.Remove(c.Param("id")); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.POST("/diagnostics/simulate-failure", localOnly(), func(c *gin.Context) {
		if !simulator.Enabled() {
			c.JSON(http.StatusNotFound, gin.H{"error": "developer mode is disabled"})
			return
		}
		peer, err := simulator.Simulate(c.Request.Context(), "Interrupted transfer peer", devsim.ScenarioInterrupted)
		if err == nil {
			request, requestErr := pairing.RequestPairing(c.Request.Context(), peer.Device.ID)
			if requestErr == nil {
				_, requestErr = pairing.Accept(c.Request.Context(), request.RequestID)
			}
			if requestErr != nil {
				err = requestErr
			}
		}
		if err == nil {
			_, err = service.QueueTestTransferTo(c.Request.Context(), peer.Device.ID)
		}
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"status": "failure_simulation_started", "deviceId": peer.Device.ID})
	})
}
