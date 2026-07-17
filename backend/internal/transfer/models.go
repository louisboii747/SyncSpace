// Package transfer implements SyncSpace's persistent, resumable local-network
// file transfer engine. The package is transport-neutral except for Sender,
// which speaks the documented HTTP peer protocol.
package transfer

import (
	"time"

	"github.com/louisboii747/syncspace/backend/internal/models"
)

const ProtocolVersion = 1

// Status is the durable transfer state machine shared by APIs and native UIs.
type Status string

const (
	StatusQueued      Status = "Queued"
	StatusPreparing   Status = "Preparing"
	StatusConnecting  Status = "Connecting"
	StatusNegotiating Status = "Negotiating"
	StatusSending     Status = "Sending"
	StatusReceiving   Status = "Receiving"
	StatusPaused      Status = "Paused"
	StatusResuming    Status = "Resuming"
	StatusCompleted   Status = "Completed"
	StatusCancelled   Status = "Cancelled"
	StatusFailed      Status = "Failed"
	StatusVerifying   Status = "Verifying"
)

type Direction string

const (
	DirectionOutbound Direction = "outbound"
	DirectionInbound  Direction = "inbound"
)

type ConflictPolicy string

const (
	ConflictPrompt    ConflictPolicy = "prompt"
	ConflictOverwrite ConflictPolicy = "overwrite"
	ConflictRename    ConflictPolicy = "rename"
)

type ChunkStatus string

const (
	ChunkPending  ChunkStatus = "pending"
	ChunkComplete ChunkStatus = "complete"
	ChunkFailed   ChunkStatus = "failed"
)

// Capabilities are advertised through discovery and used during negotiation.
type Capabilities = models.TransferCapabilities

// File describes one regular file. RelativePath always uses slash separators
// on the wire and is validated before it is joined to a destination root.
type File struct {
	ID              string `json:"fileId"`
	RelativePath    string `json:"relativePath"`
	Directory       bool   `json:"directory"`
	SourcePath      string `json:"-"`
	DestinationPath string `json:"-"`
	Size            int64  `json:"size"`
	Checksum        string `json:"checksum"`
	ChunkSize       int64  `json:"chunkSize"`
	ChunkCount      int64  `json:"chunkCount"`
}

// Transfer is the complete durable session projection.
type Transfer struct {
	ID               string         `json:"uuid"`
	Direction        Direction      `json:"direction"`
	DeviceID         string         `json:"deviceId"`
	DeviceName       string         `json:"deviceName,omitempty"`
	RemoteAddress    string         `json:"remoteAddress,omitempty"`
	Filename         string         `json:"filename"`
	Path             string         `json:"path"`
	SourcePaths      []string       `json:"sourcePaths,omitempty"`
	Files            []File         `json:"files,omitempty"`
	Size             int64          `json:"size"`
	Checksum         string         `json:"checksum,omitempty"`
	Status           Status         `json:"status"`
	Progress         int64          `json:"progress"`
	Speed            int64          `json:"speed"`
	ETASeconds       int64          `json:"etaSeconds"`
	StartedAt        *time.Time     `json:"startTime,omitempty"`
	FinishedAt       *time.Time     `json:"finishTime,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	Error            string         `json:"error,omitempty"`
	Attempts         int            `json:"attempts"`
	Priority         int64          `json:"priority"`
	Approved         bool           `json:"approved"`
	ApprovalRequired bool           `json:"approvalRequired"`
	ConflictPolicy   ConflictPolicy `json:"conflictPolicy"`
	ChunkSize        int64          `json:"chunkSize"`
	Compression      bool           `json:"compression"`
	ProtocolVersion  int            `json:"protocolVersion"`
	SessionToken     string         `json:"-"`
	SessionTokenHash string         `json:"-"`
}

type Chunk struct {
	TransferID string      `json:"transferId"`
	FileID     string      `json:"fileId"`
	Index      int64       `json:"index"`
	Offset     int64       `json:"offset"`
	Size       int64       `json:"size"`
	Checksum   string      `json:"checksum"`
	Status     ChunkStatus `json:"status"`
	Attempts   int         `json:"attempts"`
	UpdatedAt  time.Time   `json:"updatedAt"`
}

type QueueRequest struct {
	DeviceID       string         `json:"deviceId"`
	Paths          []string       `json:"paths"`
	ConflictPolicy ConflictPolicy `json:"conflictPolicy"`
}

// StagingSession is a short-lived, loopback-only upload target used by the web
// frontend. Native clients should continue to queue filesystem paths directly.
type StagingSession struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
}

// StageQueueRequest promotes a completed browser upload into the durable
// transfer queue. Roots are top-level names inside the staging session.
type StageQueueRequest struct {
	DeviceID       string         `json:"deviceId"`
	Roots          []string       `json:"roots"`
	ConflictPolicy ConflictPolicy `json:"conflictPolicy"`
}

type Offer struct {
	TransferID      string `json:"transferId"`
	DeviceID        string `json:"deviceId"`
	DeviceName      string `json:"deviceName"`
	SessionToken    string `json:"sessionToken"`
	Filename        string `json:"filename"`
	Size            int64  `json:"size"`
	Files           []File `json:"files"`
	ChunkSize       int64  `json:"chunkSize"`
	Compression     bool   `json:"compression"`
	ProtocolVersion int    `json:"protocolVersion"`
}

type OfferResponse struct {
	TransferID string `json:"transferId"`
	Status     Status `json:"status"`
	Approved   bool   `json:"approved"`
}

// PeerAuthentication authenticates the initial offer with the shared key
// established by verified pairing. Subsequent requests use the offer session
// token over the pinned TLS channel.
type PeerAuthentication struct {
	DeviceID  string
	Timestamp string
	Nonce     string
	Signature string
}

type ResumeMap struct {
	TransferID string             `json:"transferId"`
	Status     Status             `json:"status"`
	Approved   bool               `json:"approved"`
	Chunks     map[string][]int64 `json:"receivedChunks"`
}

type AcceptRequest struct {
	DestinationPath string         `json:"destinationPath"`
	ConflictPolicy  ConflictPolicy `json:"conflictPolicy"`
}

type EventType string

const (
	EventQueueUpdated EventType = "QueueUpdated"
	EventProgress     EventType = "Progress"
	EventPaused       EventType = "Pause"
	EventResumed      EventType = "Resume"
	EventCompleted    EventType = "Complete"
	EventFailed       EventType = "Failure"
	EventVerification EventType = "Verification"
)

type Event struct {
	Type      EventType `json:"type"`
	Transfer  Transfer  `json:"transfer"`
	Timestamp time.Time `json:"timestamp"`
}

type EventPublisher interface{ Publish(Event) }
