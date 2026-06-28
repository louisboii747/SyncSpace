package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/louisboii747/syncspace/backend/internal/api"
	"github.com/louisboii747/syncspace/backend/internal/discovery"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
	"github.com/louisboii747/syncspace/backend/internal/services"
	discoveryws "github.com/louisboii747/syncspace/backend/internal/websocket"
)

var buildVersion = "dev"

const (
	defaultPort         = 8384
	defaultOfflineAfter = 30 * time.Second
	defaultRemoveAfter  = 2 * time.Minute
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("Server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	identityStore := services.NewFileIdentityStore(filepath.Join(config.dataDirectory, "identity.json"))
	identity, err := identityStore.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("load device identity: %w", err)
	}
	database, err := sql.Open("sqlite", filepath.Join(config.dataDirectory, "syncspace.db"))
	if err != nil {
		return fmt.Errorf("open local database: %w", err)
	}
	database.SetMaxOpenConns(1)
	defer database.Close()
	storeContext, cancelStore := context.WithTimeout(context.Background(), 5*time.Second)
	trustedDeviceStore, err := pairing.NewSQLiteTrustedDeviceStore(storeContext, database)
	cancelStore()
	if err != nil {
		return fmt.Errorf("create trusted device store: %w", err)
	}

	listener, err := net.Listen("tcp", config.listenAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", config.listenAddress, err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	discoveryBroker := discoveryws.NewBroker()
	registry, err := discovery.NewRegistry(discovery.RegistryConfig{
		SelfID:       identity.ID,
		OfflineAfter: defaultOfflineAfter,
		RemoveAfter:  defaultRemoveAfter,
		Logger:       logger,
		Publisher:    discoveryBroker,
	})
	if err != nil {
		return fmt.Errorf("create device registry: %w", err)
	}
	discoveryService, err := discovery.NewService(discovery.ServiceConfig{
		Identity:   identity,
		Port:       port,
		AppVersion: config.appVersion,
		Registry:   registry,
		MDNS:       discovery.NewZeroconfMDNS(),
		Logger:     logger,
	})
	if err != nil {
		return fmt.Errorf("create discovery service: %w", err)
	}
	pairingBroker := discoveryws.NewPairingBroker()
	pairingService, err := pairing.NewService(pairing.ServiceConfig{
		Store:     trustedDeviceStore,
		Peers:     discoveryService,
		Publisher: pairingBroker,
		Logger:    logger,
	})
	if err != nil {
		return fmt.Errorf("create pairing service: %w", err)
	}

	discoverySocketHandler := discoveryws.NewHandler(discoveryBroker, discoveryService, logger)
	pairingSocketHandler := discoveryws.NewPairingHandler(pairingBroker, pairingService, logger)
	router := api.NewRouter(api.RouterConfig{
		Discovery:       discoveryService,
		DiscoverySocket: discoverySocketHandler.Serve,
		Pairing:         pairingService,
		PairingSocket:   pairingSocketHandler.Serve,
		Logger:          logger,
	})
	httpServer := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go discoveryService.Run(ctx)

	serveResult := make(chan error, 1)
	go func() {
		logger.Info("Server started",
			"address", listener.Addr().String(),
			"device_id", identity.ID,
			"device_name", identity.Name,
			"version", config.appVersion,
		)
		serveResult <- httpServer.Serve(listener)
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-serveResult:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = fmt.Errorf("serve HTTP: %w", err)
		}
	}
	stop()

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shut down HTTP server: %w", err)
	}
	if serveErr != nil {
		return serveErr
	}
	logger.Info("Server stopped gracefully")
	return nil
}

type serverConfig struct {
	listenAddress string
	dataDirectory string
	appVersion    string
}

func loadConfig() (serverConfig, error) {
	port := defaultPort
	if value := os.Getenv("SYNCSPACE_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 65535 {
			return serverConfig{}, fmt.Errorf("SYNCSPACE_PORT must be between 1 and 65535")
		}
		port = parsed
	}
	host := os.Getenv("SYNCSPACE_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	dataDirectory := os.Getenv("SYNCSPACE_DATA_DIR")
	if dataDirectory == "" {
		configDirectory, err := os.UserConfigDir()
		if err != nil {
			return serverConfig{}, fmt.Errorf("resolve user config directory: %w", err)
		}
		dataDirectory = filepath.Join(configDirectory, "SyncSpace")
	}

	appVersion := os.Getenv("SYNCSPACE_APP_VERSION")
	if appVersion == "" {
		appVersion = buildVersion
	}
	return serverConfig{
		listenAddress: net.JoinHostPort(host, strconv.Itoa(port)),
		dataDirectory: dataDirectory,
		appVersion:    appVersion,
	}, nil
}
