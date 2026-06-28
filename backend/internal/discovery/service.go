package discovery

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/services"
)

var errSessionRestart = errors.New("discovery session restart requested")

// ServiceConfig supplies dependencies and lifecycle settings for discovery.
type ServiceConfig struct {
	Identity        services.Identity
	Port            int
	AppVersion      string
	Registry        *Registry
	MDNS            MDNS
	Logger          *slog.Logger
	NetworkInterval time.Duration
	RestartInterval time.Duration
	SweepInterval   time.Duration
}

// Service supervises broadcasting, browsing, network-change recovery, peer
// expiry, and access to discovery state.
type Service struct {
	identity        services.Identity
	port            int
	appVersion      string
	registry        *Registry
	mdns            MDNS
	logger          *slog.Logger
	networkInterval time.Duration
	restartInterval time.Duration
	sweepInterval   time.Duration
	refresh         chan struct{}

	selfMu sync.RWMutex
	self   models.Device
}

// NewService constructs the continuously supervised discovery service.
func NewService(config ServiceConfig) (*Service, error) {
	if config.Registry == nil || config.MDNS == nil {
		return nil, errors.New("registry and mDNS adapter are required")
	}
	if config.Port < 1 || config.Port > 65535 {
		return nil, errors.New("discovery port must be between 1 and 65535")
	}
	if strings.TrimSpace(config.AppVersion) == "" {
		return nil, errors.New("app version is required")
	}
	if err := config.Identity.Validate(); err != nil {
		return nil, fmt.Errorf("invalid local identity: %w", err)
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.NetworkInterval <= 0 {
		config.NetworkInterval = 5 * time.Second
	}
	if config.RestartInterval <= 0 {
		config.RestartInterval = 60 * time.Second
	}
	if config.SweepInterval <= 0 {
		config.SweepInterval = 5 * time.Second
	}

	now := time.Now().UTC()
	return &Service{
		identity:        config.Identity,
		port:            config.Port,
		appVersion:      config.AppVersion,
		registry:        config.Registry,
		mdns:            config.MDNS,
		logger:          config.Logger,
		networkInterval: config.NetworkInterval,
		restartInterval: config.RestartInterval,
		sweepInterval:   config.SweepInterval,
		refresh:         make(chan struct{}, 1),
		self: models.Device{
			ID:              config.Identity.ID,
			Name:            config.Identity.Name,
			Type:            config.Identity.Type,
			Platform:        config.Identity.Platform,
			Port:            config.Port,
			AppVersion:      config.AppVersion,
			LastSeen:        now,
			Online:          true,
			ConnectionState: models.ConnectionOnline,
		},
	}, nil
}

// Run starts discovery and blocks until ctx is cancelled. Failed and panicked
// sessions are restarted with bounded exponential backoff.
func (s *Service) Run(ctx context.Context) {
	s.logger.Info("Discovery started", "service", serviceName, "device_id", s.identity.ID)
	go s.runSweeper(ctx)

	backoff := time.Second
	for ctx.Err() == nil {
		err := s.safeRunSession(ctx)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errSessionRestart) {
			backoff = time.Second
			continue
		}
		s.logger.Error("Discovery error", "error", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

// Devices returns the current peer registry snapshot.
func (s *Service) Devices() []models.Device {
	return s.registry.List()
}

// Self returns a snapshot of this device's current discovery identity and
// network endpoint.
func (s *Service) Self() models.Device {
	s.selfMu.RLock()
	defer s.selfMu.RUnlock()
	return s.self
}

// Refresh requests an immediate teardown and clean mDNS rescan. Calls are
// coalesced so HTTP handlers never block behind discovery work.
func (s *Service) Refresh() {
	select {
	case s.refresh <- struct{}{}:
	default:
	}
}

func (s *Service) safeRunSession(ctx context.Context) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("discovery session panicked: %v", recovered)
			s.logger.Error("Discovery error", "error", err, "stack", string(debug.Stack()))
		}
	}()
	return s.runSession(ctx)
}

func (s *Service) runSession(ctx context.Context) error {
	snapshot, err := inspectNetwork()
	if err != nil {
		return err
	}
	s.updateSelf(snapshot.primaryIP)

	local := s.Self()
	advertisement := LocalAdvertisement{
		Instance:      safeInstanceName(local.Name, local.ID),
		InterfaceName: snapshot.interfaceName,
		Port:          local.Port,
		Text: []string{
			"id=" + local.ID,
			"name=" + local.Name,
			"type=" + local.Type,
			"platform=" + local.Platform,
			"version=" + local.AppVersion,
			"protocol=1",
		},
	}
	advertiser, err := s.mdns.Advertise(advertisement)
	if err != nil {
		return err
	}
	defer advertiser.Shutdown()
	s.logger.Info("Broadcasting",
		"device_id", local.ID,
		"ip", local.LocalIP,
		"interface", snapshot.interfaceName,
		"interface_description", snapshot.interfaceDescription,
		"ip_selection_reason", snapshot.selectionReason,
		"port", local.Port,
		"service", serviceName,
	)

	sessionContext, cancel := context.WithCancel(ctx)
	defer cancel()
	entries := make(chan Advertisement, 32)
	browseResult := make(chan error, 1)
	go func() { browseResult <- s.mdns.Browse(sessionContext, entries) }()

	networkTicker := time.NewTicker(s.networkInterval)
	defer networkTicker.Stop()
	restartTicker := time.NewTicker(s.restartInterval)
	defer restartTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-s.refresh:
			return errSessionRestart
		case err := <-browseResult:
			if err == nil {
				err = errors.New("mDNS browser exited unexpectedly")
			}
			return err
		case entry := <-entries:
			s.handleAdvertisement(entry)
		case <-networkTicker.C:
			current, err := inspectNetwork()
			if err != nil {
				s.logger.Error("Discovery error", "operation", "inspect_network", "error", err)
				continue
			}
			if current.fingerprint != snapshot.fingerprint {
				s.logger.Info("Network changed",
					"previous", snapshot.fingerprint,
					"current", current.fingerprint,
					"selected_interface", current.interfaceName,
					"selected_interface_description", current.interfaceDescription,
					"selected_ip", current.primaryIP,
					"ip_selection_reason", current.selectionReason,
				)
				return errSessionRestart
			}
		case <-restartTicker.C:
			return errSessionRestart
		}
	}
}

func (s *Service) runSweeper(ctx context.Context) {
	ticker := time.NewTicker(s.sweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.registry.Sweep(now)
		}
	}
}

func (s *Service) updateSelf(localIP string) {
	s.selfMu.Lock()
	s.self.LocalIP = localIP
	s.self.LastSeen = time.Now().UTC()
	s.self.Online = true
	s.self.ConnectionState = models.ConnectionOnline
	s.selfMu.Unlock()
}

func (s *Service) handleAdvertisement(advertisement Advertisement) {
	device, err := parseAdvertisement(advertisement)
	if err != nil {
		s.logger.Warn("Discovery error", "operation", "parse_advertisement", "hostname", advertisement.Hostname, "error", err)
		return
	}
	if err := s.registry.Upsert(device, time.Now()); err != nil {
		level := slog.LevelWarn
		if errors.Is(err, ErrDuplicateIdentity) {
			s.logger.Log(context.Background(), level, "Duplicate device detected",
				"device_id", device.ID,
				"ip", device.LocalIP,
				"hostname", advertisement.Hostname,
			)
			return
		}
		s.logger.Warn("Discovery error", "operation", "update_registry", "error", err)
	}
}

func parseAdvertisement(advertisement Advertisement) (models.Device, error) {
	values := make(map[string]string, len(advertisement.Text))
	for _, item := range advertisement.Text {
		key, value, ok := strings.Cut(item, "=")
		if !ok || key == "" {
			continue
		}
		if _, exists := values[key]; !exists {
			values[key] = value
		}
	}
	if values["protocol"] != "1" {
		return models.Device{}, errors.New("unsupported discovery protocol")
	}
	ip := bestAddress(advertisement.IPv4, advertisement.IPv6)
	if ip == nil {
		return models.Device{}, errors.New("advertisement has no usable local address")
	}
	device := models.Device{
		ID:         values["id"],
		Name:       values["name"],
		Type:       values["type"],
		Platform:   values["platform"],
		LocalIP:    ip.String(),
		Port:       advertisement.Port,
		AppVersion: values["version"],
	}
	if err := validateDiscoveredDevice(device); err != nil {
		return models.Device{}, err
	}
	return device, nil
}

func bestAddress(ipv4, ipv6 []net.IP) net.IP {
	for _, preferredRange := range []string{"192.168/16", "10/8", "172.16/12"} {
		for _, ip := range ipv4 {
			if usableLocalIP(ip) && privateIPv4Range(ip) == preferredRange {
				return ip
			}
		}
	}
	for _, ip := range ipv4 {
		if usableLocalIP(ip) {
			return ip
		}
	}
	for _, ip := range ipv6 {
		if usableLocalIP(ip) && ip.IsPrivate() {
			return ip
		}
	}
	for _, ip := range ipv6 {
		if usableLocalIP(ip) {
			return ip
		}
	}
	return nil
}
