package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/louisboii747/syncspace/backend/internal/transfer"
)

type TransferService interface {
	Queue(context.Context, transfer.QueueRequest) (transfer.Transfer, error)
	ReceiveOffer(context.Context, transfer.Offer, string) (transfer.Transfer, error)
	List(context.Context) ([]transfer.Transfer, error)
	Get(context.Context, string) (transfer.Transfer, error)
	DeleteHistory(context.Context) error
	Accept(context.Context, string, transfer.AcceptRequest) (transfer.Transfer, error)
	Reject(context.Context, string) (transfer.Transfer, error)
	Pause(context.Context, string) (transfer.Transfer, error)
	Resume(context.Context, string) (transfer.Transfer, error)
	Cancel(context.Context, string) (transfer.Transfer, error)
	Retry(context.Context, string) (transfer.Transfer, error)
	ProtocolStatus(context.Context, string, string) (transfer.ResumeMap, error)
	ProtocolCancel(context.Context, string, string) (transfer.Transfer, error)
	ReceiveChunk(context.Context, string, string, string, int64, string, string, io.Reader) (transfer.Transfer, error)
	Finalize(context.Context, string, string) (transfer.Transfer, error)
	CreateStage(context.Context) (transfer.StagingSession, error)
	UploadStagedFile(context.Context, string, string, int64, io.Reader) error
	QueueStage(context.Context, string, transfer.StageQueueRequest) (transfer.Transfer, error)
	DeleteStage(context.Context, string) error
}

func registerTransferRoutes(router *gin.Engine, service TransferService, socket gin.HandlerFunc, logger *slog.Logger) {
	local := router.Group("/transfers", localOnly())
	local.POST("/staging", func(c *gin.Context) {
		value, err := service.CreateStage(c.Request.Context())
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusCreated, value)
	})
	local.PUT("/staging/:id/files", func(c *gin.Context) {
		if c.Request.ContentLength < 0 {
			c.JSON(http.StatusLengthRequired, gin.H{"error": "content length is required"})
			return
		}
		if err := service.UploadStagedFile(c.Request.Context(), c.Param("id"), c.Query("path"), c.Request.ContentLength, c.Request.Body); err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	local.POST("/staging/:id/queue", func(c *gin.Context) {
		var body transfer.StageQueueRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId and roots are required"})
			return
		}
		value, err := service.QueueStage(c.Request.Context(), c.Param("id"), body)
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusAccepted, value)
	})
	local.DELETE("/staging/:id", func(c *gin.Context) {
		if err := service.DeleteStage(c.Request.Context(), c.Param("id")); err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	local.POST("", func(c *gin.Context) {
		var body transfer.QueueRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "deviceId and paths are required"})
			return
		}
		value, err := service.Queue(c.Request.Context(), body)
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusAccepted, value)
	})
	local.GET("", func(c *gin.Context) {
		values, err := service.List(c.Request.Context())
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, values)
	})
	local.DELETE("/history", func(c *gin.Context) {
		if err := service.DeleteHistory(c.Request.Context()); err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	local.GET("/:id", func(c *gin.Context) {
		value, err := service.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, value)
	})
	local.GET("/:id/status", func(c *gin.Context) {
		value, err := service.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, value)
	})
	local.POST("/:id/accept", func(c *gin.Context) {
		var body transfer.AcceptRequest
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "destinationPath is required"})
			return
		}
		value, err := service.Accept(c.Request.Context(), c.Param("id"), body)
		writeTransferResult(c, logger, value, err)
	})
	local.POST("/:id/reject", func(c *gin.Context) {
		value, err := service.Reject(c.Request.Context(), c.Param("id"))
		writeTransferResult(c, logger, value, err)
	})
	local.POST("/:id/pause", func(c *gin.Context) {
		value, err := service.Pause(c.Request.Context(), c.Param("id"))
		writeTransferResult(c, logger, value, err)
	})
	local.POST("/:id/resume", func(c *gin.Context) {
		value, err := service.Resume(c.Request.Context(), c.Param("id"))
		writeTransferResult(c, logger, value, err)
	})
	local.POST("/:id/cancel", func(c *gin.Context) {
		value, err := service.Cancel(c.Request.Context(), c.Param("id"))
		writeTransferResult(c, logger, value, err)
	})
	local.POST("/:id/retry", func(c *gin.Context) {
		value, err := service.Retry(c.Request.Context(), c.Param("id"))
		writeTransferResult(c, logger, value, err)
	})
	router.GET("/ws/transfers", localOnly(), socket)

	peer := router.Group("/v1/transfers")
	peer.POST("/offers", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<20)
		var body transfer.Offer
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transfer offer"})
			return
		}
		value, err := service.ReceiveOffer(c.Request.Context(), body, remoteHost(c.Request.RemoteAddr))
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusAccepted, transfer.OfferResponse{TransferID: value.ID, Status: value.Status, Approved: value.Approved})
	})
	peer.GET("/:id/status", func(c *gin.Context) {
		value, err := service.ProtocolStatus(c.Request.Context(), c.Param("id"), bearerToken(c))
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, value)
	})
	peer.PUT("/:id/files/:fileId/chunks/:index", func(c *gin.Context) {
		index, err := strconv.ParseInt(c.Param("index"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk index"})
			return
		}
		value, err := service.ReceiveChunk(c.Request.Context(), c.Param("id"), c.Param("fileId"), bearerToken(c), index, c.GetHeader("X-Chunk-SHA256"), c.GetHeader("Content-Encoding"), c.Request.Body)
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"transferId": value.ID, "progress": value.Progress, "status": value.Status})
	})
	peer.POST("/:id/complete", func(c *gin.Context) {
		value, err := service.Finalize(c.Request.Context(), c.Param("id"), bearerToken(c))
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, value)
	})
	peer.DELETE("/:id", func(c *gin.Context) {
		value, err := service.ProtocolCancel(c.Request.Context(), c.Param("id"), bearerToken(c))
		if err != nil {
			writeTransferError(c, logger, err)
			return
		}
		c.JSON(http.StatusOK, value)
	})
}

func writeTransferResult(c *gin.Context, logger *slog.Logger, value transfer.Transfer, err error) {
	if err != nil {
		writeTransferError(c, logger, err)
		return
	}
	c.JSON(http.StatusOK, value)
}
func writeTransferError(c *gin.Context, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, transfer.ErrInvalidRequest), errors.Is(err, transfer.ErrPathTraversal):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, transfer.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid transfer authorization"})
	case errors.Is(err, transfer.ErrUntrustedDevice):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, transfer.ErrApprovalRequired), errors.Is(err, transfer.ErrConflictResolution), errors.Is(err, transfer.ErrInvalidState):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, transfer.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, transfer.ErrChecksumMismatch):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		logger.Error("Transfer API error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
func bearerToken(c *gin.Context) string {
	scheme, value, ok := strings.Cut(c.GetHeader("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(value)
}
func remoteHost(address string) string {
	host, _, err := net.SplitHostPort(address)
	if err == nil {
		return host
	}
	return strings.Trim(address, "[]")
}
