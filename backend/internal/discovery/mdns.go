package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	serviceName   = "_syncspace._tcp"
	serviceDomain = "local."
	browseWindow  = 10 * time.Second
)

// Advertisement contains the untrusted wire data received from mDNS.
type Advertisement struct {
	Port     int
	Text     []string
	IPv4     []net.IP
	IPv6     []net.IP
	Hostname string
}

// LocalAdvertisement describes this process's mDNS service record.
type LocalAdvertisement struct {
	Instance      string
	InterfaceName string
	Port          int
	Text          []string
}

// AdvertisementHandle controls a live local broadcast.
type AdvertisementHandle interface {
	Shutdown()
}

// MDNS abstracts Zeroconf so discovery supervision and registry behavior can
// be tested without binding multicast sockets.
type MDNS interface {
	Advertise(LocalAdvertisement) (AdvertisementHandle, error)
	Browse(context.Context, chan<- Advertisement) error
}

// ZeroconfMDNS is the production mDNS implementation.
type ZeroconfMDNS struct{}

// NewZeroconfMDNS creates the production mDNS adapter.
func NewZeroconfMDNS() *ZeroconfMDNS {
	return &ZeroconfMDNS{}
}

// Advertise publishes the local SyncSpace service.
func (z *ZeroconfMDNS) Advertise(advertisement LocalAdvertisement) (AdvertisementHandle, error) {
	networkInterface, err := net.InterfaceByName(advertisement.InterfaceName)
	if err != nil {
		return nil, fmt.Errorf("resolve selected mDNS interface %q: %w", advertisement.InterfaceName, err)
	}
	server, err := zeroconf.Register(
		advertisement.Instance,
		serviceName,
		serviceDomain,
		advertisement.Port,
		advertisement.Text,
		[]net.Interface{*networkInterface},
	)
	if err != nil {
		return nil, fmt.Errorf("register mDNS service: %w", err)
	}
	return server, nil
}

// Browse continuously translates Zeroconf entries until the context ends. The
// upstream resolver suppresses duplicate records after its first match, so a
// fresh query window is opened regularly to provide reliable LastSeen updates.
func (z *ZeroconfMDNS) Browse(ctx context.Context, output chan<- Advertisement) error {
	for ctx.Err() == nil {
		if err := z.browseOnce(ctx, output); err != nil {
			return err
		}
	}
	return nil
}

func (z *ZeroconfMDNS) browseOnce(ctx context.Context, output chan<- Advertisement) error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return fmt.Errorf("create mDNS resolver: %w", err)
	}
	windowContext, cancel := context.WithCancel(ctx)
	defer cancel()
	entries := make(chan *zeroconf.ServiceEntry, 32)
	if err := resolver.Browse(windowContext, serviceName, serviceDomain, entries); err != nil {
		return fmt.Errorf("start mDNS browse: %w", err)
	}
	timer := time.NewTimer(browseWindow)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return nil
		case entry, ok := <-entries:
			if !ok {
				if ctx.Err() != nil || windowContext.Err() != nil {
					return nil
				}
				return errors.New("mDNS browse stopped unexpectedly")
			}
			if entry == nil {
				continue
			}
			advertisement := Advertisement{
				Port:     entry.Port,
				Text:     append([]string(nil), entry.Text...),
				IPv4:     cloneIPs(entry.AddrIPv4),
				IPv6:     cloneIPs(entry.AddrIPv6),
				Hostname: entry.HostName,
			}
			select {
			case output <- advertisement:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

func cloneIPs(input []net.IP) []net.IP {
	result := make([]net.IP, 0, len(input))
	for _, ip := range input {
		result = append(result, append(net.IP(nil), ip...))
	}
	return result
}

func safeInstanceName(name, id string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "SyncSpace"
	}
	if len(name) > 48 {
		name = name[:48]
	}
	shortID := id
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return name + "-" + shortID
}
