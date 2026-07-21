package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/louisboii747/syncspace/backend/internal/api"
	appdatabase "github.com/louisboii747/syncspace/backend/internal/database"
	"github.com/louisboii747/syncspace/backend/internal/devsim"
	"github.com/louisboii747/syncspace/backend/internal/diagnostics"
	"github.com/louisboii747/syncspace/backend/internal/discovery"
	"github.com/louisboii747/syncspace/backend/internal/frontend"
	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
	"github.com/louisboii747/syncspace/backend/internal/services"
	"github.com/louisboii747/syncspace/backend/internal/settings"
	"github.com/louisboii747/syncspace/backend/internal/transfer"
	discoveryws "github.com/louisboii747/syncspace/backend/internal/websocket"
)

var buildVersion = "dev"

const (
	defaultPort         = 8384
	defaultOfflineAfter = 30 * time.Second
	defaultRemoveAfter  = 2 * time.Minute
)

func main() {
	if handled, code := handleCommand(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	logBuffer := diagnostics.NewLogBuffer(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}), 500)
	logger := slog.New(logBuffer)
	if err := run(logger, logBuffer); err != nil {
		logger.Error("Server stopped", "error", err)
		os.Exit(1)
	}
}

func handleCommand(args []string, stdout, stderr io.Writer) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	if len(args) == 1 {
		switch args[0] {
		case "version", "--version":
			fmt.Fprintf(stdout, "SyncSpace %s\n", buildVersion)
			return true, 0
		case "help", "-h", "--help":
			fmt.Fprintln(stdout, "Usage: syncspace [--version|--help]")
			fmt.Fprintln(stdout, "Run without arguments to start the local SyncSpace service.")
			return true, 0
		}
	}
	fmt.Fprintf(stderr, "syncspace: unsupported argument: %s\n", strings.Join(args, " "))
	fmt.Fprintln(stderr, "Run 'syncspace --help' for usage.")
	return true, 2
}

func run(logger *slog.Logger, logBuffer *diagnostics.LogBuffer) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}

	identityStore := services.NewFileIdentityStore(filepath.Join(config.dataDirectory, "identity.json"))
	identity, err := identityStore.LoadOrCreate()
	if err != nil {
		return fmt.Errorf("load device identity: %w", err)
	}
	settingsStore, err := settings.NewStore(filepath.Join(config.dataDirectory, "settings.json"))
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	preferences := settingsStore.Get()
	if strings.TrimSpace(preferences.DeviceName) == "" {
		preferences.DeviceName = identity.Name
		preferences, err = settingsStore.Update(preferences)
		if err != nil {
			return fmt.Errorf("initialise device display name: %w", err)
		}
	}
	identity.Name = preferences.DeviceName
	databasePath := filepath.Join(config.dataDirectory, "syncspace.db")
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return fmt.Errorf("open local database: %w", err)
	}
	database.SetMaxOpenConns(4)
	defer database.Close()
	migrationContext, cancelMigrations := context.WithTimeout(context.Background(), 10*time.Second)
	err = appdatabase.Migrate(migrationContext, database)
	cancelMigrations()
	if err != nil {
		return fmt.Errorf("migrate local database: %w", err)
	}
	storeContext, cancelStore := context.WithTimeout(context.Background(), 5*time.Second)
	trustedDeviceStore, err := pairing.NewSQLiteTrustedDeviceStore(storeContext, database)
	cancelStore()
	if err != nil {
		return fmt.Errorf("create trusted device store: %w", err)
	}

	managementListener, err := net.Listen("tcp", config.managementAddress)
	if err != nil {
		return fmt.Errorf("listen for local management on %s: %w", config.managementAddress, err)
	}
	defer managementListener.Close()
	peerListener, err := net.Listen("tcp", config.peerAddress)
	if err != nil {
		return fmt.Errorf("listen for encrypted peers on %s: %w", config.peerAddress, err)
	}
	defer peerListener.Close()
	managementPort := managementListener.Addr().(*net.TCPAddr).Port
	peerPort := peerListener.Addr().(*net.TCPAddr).Port
	certificate, err := identity.TLSCertificate(time.Now())
	if err != nil {
		return fmt.Errorf("create peer TLS identity: %w", err)
	}
	peerTLSListener := tls.NewListener(peerListener, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}})

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
		Identity:             identity,
		Port:                 peerPort,
		AppVersion:           config.appVersion,
		Registry:             registry,
		MDNS:                 discovery.NewZeroconfMDNS(),
		Logger:               logger,
		TransferCapabilities: transfer.DetectCapabilities(filepath.Join(config.dataDirectory, "transfers")),
	})
	if err != nil {
		return fmt.Errorf("create discovery service: %w", err)
	}
	peerDirectory, err := devsim.New(devsim.Config{Enabled: config.developerMode, Base: discoveryService, Logger: logger, StaticPeers: config.staticPeers})
	if err != nil {
		return fmt.Errorf("create developer peer directory: %w", err)
	}
	defer peerDirectory.Close()
	pairingBroker := discoveryws.NewPairingBroker()
	pairingService, err := pairing.NewService(pairing.ServiceConfig{
		Store:     trustedDeviceStore,
		Peers:     peerDirectory,
		Identity:  identity,
		Publisher: pairingBroker,
		Logger:    logger,
	})
	if err != nil {
		return fmt.Errorf("create pairing service: %w", err)
	}
	transferStoreContext, cancelTransferStore := context.WithTimeout(context.Background(), 5*time.Second)
	transferStore, err := transfer.NewSQLiteStore(transferStoreContext, database)
	cancelTransferStore()
	if err != nil {
		return fmt.Errorf("create transfer store: %w", err)
	}
	transferBroker := discoveryws.NewTransferBroker()
	transferService, err := transfer.NewService(transfer.ServiceConfig{
		Store: transferStore, Peers: peerDirectory, Authorizer: pairingService,
		Identity: identity, DataDirectory: filepath.Join(config.dataDirectory, "transfers"),
		Publisher: transferBroker, Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("create transfer service: %w", err)
	}
	discoverySocketHandler := discoveryws.NewHandler(discoveryBroker, peerDirectory, logger)
	pairingSocketHandler := discoveryws.NewPairingHandler(pairingBroker, pairingService, logger)
	transferSocketHandler := discoveryws.NewTransferHandler(transferBroker, transferService, logger)
	var discoveryRunning atomic.Bool
	diagnosticsService, err := diagnostics.New(diagnostics.Config{
		Database: database, DatabasePath: databasePath, StoragePath: filepath.Join(config.dataDirectory, "transfers"),
		BackendURL: "http://127.0.0.1:" + strconv.Itoa(managementPort), Identity: identity, Discovery: peerDirectory,
		Trust: pairingService, Transfers: transferService, Logs: logBuffer, DeveloperMode: config.developerMode,
		DiscoveryState: discoveryRunning.Load,
		WebSocketState: func() diagnostics.WebSocketState {
			return diagnostics.WebSocketState{Discovery: discoveryBroker.SubscriberCount(), Pairing: pairingBroker.SubscriberCount(), Transfers: transferBroker.SubscriberCount()}
		},
	})
	if err != nil {
		return fmt.Errorf("create diagnostics service: %w", err)
	}
	applySettings := func(values settings.Values) {
		name := strings.TrimSpace(values.DeviceName)
		if name == "" {
			return
		}
		if err := discoveryService.SetDisplayName(name); err != nil {
			logger.Warn("Unable to apply device display name", "error", err)
			return
		}
		pairingService.SetDisplayName(name)
		transferService.SetDisplayName(name)
	}
	router := api.NewRouter(api.RouterConfig{
		Discovery:       peerDirectory,
		DiscoverySocket: discoverySocketHandler.Serve,
		Pairing:         pairingService,
		PairingSocket:   pairingSocketHandler.Serve,
		Transfer:        transferService,
		TransferSocket:  transferSocketHandler.Serve,
		Diagnostics:     diagnosticsService,
		Simulator:       peerDirectory,
		Settings:        settingsStore,
		SettingsChanged: applySettings,
		Frontend:        frontend.Handler(),
		Logger:          logger,
	})
	managementServer := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      0,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	peerServer := &http.Server{Handler: router, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 0, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go superviseRuntime(ctx, discoveryService, transferService, settingsStore, &discoveryRunning)

	serveResult := make(chan error, 2)
	go func() {
		logger.Info("Server started",
			"management_address", managementListener.Addr().String(),
			"peer_address", peerListener.Addr().String(),
			"device_id", identity.ID,
			"device_name", identity.Name,
			"version", config.appVersion,
		)
		serveResult <- managementServer.Serve(managementListener)
	}()
	go func() { serveResult <- peerServer.Serve(peerTLSListener) }()

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
	managementErr := managementServer.Shutdown(shutdownContext)
	peerErr := peerServer.Shutdown(shutdownContext)
	if managementErr != nil {
		return fmt.Errorf("shut down management server: %w", managementErr)
	}
	if peerErr != nil {
		return fmt.Errorf("shut down peer server: %w", peerErr)
	}
	if serveErr != nil {
		return serveErr
	}
	logger.Info("Server stopped gracefully")
	return nil
}

func superviseRuntime(ctx context.Context, discoveryService *discovery.Service, transferService *transfer.Service, preferences *settings.Store, discoveryRunning *atomic.Bool) {
	var discoveryCancel context.CancelFunc
	var transferCancel context.CancelFunc
	var discoveryDone <-chan struct{}
	var transferDone <-chan struct{}
	stopDiscovery := func() {
		if discoveryCancel != nil {
			discoveryCancel()
			<-discoveryDone
			discoveryCancel, discoveryDone = nil, nil
		}
		discoveryRunning.Store(false)
	}
	stopTransfers := func() {
		if transferCancel != nil {
			transferCancel()
			<-transferDone
			transferCancel, transferDone = nil, nil
		}
	}
	defer stopDiscovery()
	defer stopTransfers()

	for {
		values := preferences.Get()
		accepted := preferences.PrivacyAccepted()
		if accepted && transferCancel == nil {
			serviceContext, cancel := context.WithCancel(ctx)
			done := make(chan struct{})
			transferCancel, transferDone = cancel, done
			go func() { transferService.Run(serviceContext); close(done) }()
		} else if !accepted {
			stopTransfers()
		}
		if accepted && values.Discoverable && discoveryCancel == nil {
			serviceContext, cancel := context.WithCancel(ctx)
			done := make(chan struct{})
			discoveryCancel, discoveryDone = cancel, done
			discoveryRunning.Store(true)
			go func() { discoveryService.Run(serviceContext); close(done) }()
		} else if (!accepted || !values.Discoverable) && discoveryCancel != nil {
			stopDiscovery()
		}
		select {
		case <-ctx.Done():
			return
		case <-preferences.Changes():
			continue
		case <-discoveryDone:
			discoveryCancel, discoveryDone = nil, nil
			discoveryRunning.Store(false)
		case <-transferDone:
			transferCancel, transferDone = nil, nil
		}
	}
}

type serverConfig struct {
	managementAddress string
	peerAddress       string
	dataDirectory     string
	appVersion        string
	developerMode     bool
	staticPeers       []models.Device
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
		host = "127.0.0.1"
	}
	parsedHost := net.ParseIP(strings.Trim(host, "[]"))
	if parsedHost == nil || !parsedHost.IsLoopback() {
		return serverConfig{}, errors.New("SYNCSPACE_HOST must be a loopback address; peer traffic uses SYNCSPACE_PEER_HOST")
	}
	peerHost := os.Getenv("SYNCSPACE_PEER_HOST")
	if peerHost == "" {
		peerHost = "0.0.0.0"
	}
	peerPort := port + 1
	if value := os.Getenv("SYNCSPACE_PEER_PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 65535 {
			return serverConfig{}, errors.New("SYNCSPACE_PEER_PORT must be between 1 and 65535")
		}
		peerPort = parsed
	}
	if peerPort > 65535 {
		return serverConfig{}, errors.New("peer port is outside the valid range; set SYNCSPACE_PEER_PORT")
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
	developerMode := strings.EqualFold(os.Getenv("SYNCSPACE_DEV_MODE"), "true") || os.Getenv("SYNCSPACE_DEV_MODE") == "1"
	var staticPeers []models.Device
	if value := strings.TrimSpace(os.Getenv("SYNCSPACE_STATIC_PEERS")); value != "" {
		if !developerMode {
			return serverConfig{}, fmt.Errorf("SYNCSPACE_STATIC_PEERS requires SYNCSPACE_DEV_MODE=true")
		}
		if err := json.Unmarshal([]byte(value), &staticPeers); err != nil {
			return serverConfig{}, fmt.Errorf("decode SYNCSPACE_STATIC_PEERS: %w", err)
		}
	}
	return serverConfig{
		managementAddress: net.JoinHostPort(host, strconv.Itoa(port)),
		peerAddress:       net.JoinHostPort(peerHost, strconv.Itoa(peerPort)),
		dataDirectory:     dataDirectory,
		appVersion:        appVersion,
		developerMode:     developerMode,
		staticPeers:       staticPeers,
	}, nil
}
