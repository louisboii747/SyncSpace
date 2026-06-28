package discovery

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/louisboii747/syncspace/backend/internal/services"
)

type fakeAdvertisementHandle struct {
	shutdown atomic.Bool
}

func (h *fakeAdvertisementHandle) Shutdown() {
	h.shutdown.Store(true)
}

type fakeMDNS struct {
	advertised chan LocalAdvertisement
	peer       *Advertisement
}

func (m *fakeMDNS) Advertise(advertisement LocalAdvertisement) (AdvertisementHandle, error) {
	m.advertised <- advertisement
	return &fakeAdvertisementHandle{}, nil
}

func (m *fakeMDNS) Browse(ctx context.Context, entries chan<- Advertisement) error {
	if m.peer != nil {
		select {
		case entries <- *m.peer:
		case <-ctx.Done():
			return nil
		}
	}
	<-ctx.Done()
	return nil
}

func TestServiceRefreshRestartsDiscoveryAndFindsPeer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	identity := services.Identity{
		ID:       uuid.NewString(),
		Name:     "Local-PC",
		Type:     "desktop",
		Platform: "windows",
	}
	registry, err := NewRegistry(RegistryConfig{
		SelfID:       identity.ID,
		OfflineAfter: time.Minute,
		RemoveAfter:  2 * time.Minute,
		Logger:       logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	peerID := uuid.NewString()
	mdns := &fakeMDNS{
		advertised: make(chan LocalAdvertisement, 2),
		peer: &Advertisement{
			Port: 8384,
			Text: []string{
				"id=" + peerID,
				"name=Peer-Mac",
				"type=desktop",
				"platform=darwin",
				"version=1.0.0",
				"protocol=1",
			},
			IPv4: []net.IP{net.ParseIP("192.168.1.20")},
		},
	}
	service, err := NewService(ServiceConfig{
		Identity:        identity,
		Port:            8384,
		AppVersion:      "1.0.0",
		Registry:        registry,
		MDNS:            mdns,
		Logger:          logger,
		NetworkInterval: time.Hour,
		RestartInterval: time.Hour,
		SweepInterval:   time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		service.Run(ctx)
		close(done)
	}()
	firstAdvertisement := waitForAdvertisement(t, mdns.advertised)
	if firstAdvertisement.InterfaceName == "" {
		t.Fatal("mDNS advertisement did not use the selected interface")
	}
	waitForDevice(t, service, peerID)

	service.Refresh()
	waitForAdvertisement(t, mdns.advertised)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("discovery service did not stop after cancellation")
	}
}

func TestParseAdvertisementRejectsUnsupportedProtocol(t *testing.T) {
	_, err := parseAdvertisement(Advertisement{
		Port: 8384,
		Text: []string{"protocol=2"},
		IPv4: []net.IP{net.ParseIP("192.168.1.2")},
	})
	if err == nil {
		t.Fatal("expected unsupported protocol error")
	}
}

func waitForAdvertisement(t *testing.T, advertisements <-chan LocalAdvertisement) LocalAdvertisement {
	t.Helper()
	select {
	case advertisement := <-advertisements:
		return advertisement
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for mDNS advertisement")
		return LocalAdvertisement{}
	}
}

func waitForDevice(t *testing.T, service *Service, id string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, device := range service.Devices() {
			if device.ID == id {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for discovered peer")
}
