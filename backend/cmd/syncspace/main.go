// Command syncspace provides local development and diagnostics workflows.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/pairing"
	"github.com/louisboii747/syncspace/backend/internal/services"
	"github.com/louisboii747/syncspace/backend/internal/transfer"
)

const (
	deviceAID = "00000000-0000-4000-8000-00000000000a"
	deviceBID = "00000000-0000-4000-8000-00000000000b"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "syncspace:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "doctor":
		return doctor(args[1:])
	case "dev":
		return dev(args[1:])
	case "test-transfer":
		return testTransfer(args[1:])
	case "export-diagnostics":
		return exportDiagnostics(args[1:])
	case "help", "-h", "--help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() error {
	fmt.Println(`SyncSpace developer CLI

  syncspace doctor [--url http://127.0.0.1:8384]
  syncspace dev start [--port-a 8384 --port-b 8385]
  syncspace dev verify [--port-a 8384 --port-b 8385]
  syncspace dev device-a [--port 8384 --peer-port 18385 --listen-peer-port 18384]
  syncspace dev device-b [--port 8385 --peer-port 18384 --listen-peer-port 18385]
  syncspace dev reset
  syncspace dev seed
  syncspace test-transfer
  syncspace export-diagnostics [--output diagnostics.zip]`)
	return nil
}

func dev(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "start":
		return devStart(args[1:])
	case "verify":
		return devStart(append([]string{"--verify"}, args[1:]...))
	case "device-a":
		return devDevice("a", args[1:])
	case "device-b":
		return devDevice("b", args[1:])
	case "reset":
		return devReset()
	case "seed":
		_, err := seedFiles()
		return err
	case "simulate-device":
		return simulateDevice(args[1:])
	default:
		return fmt.Errorf("unknown dev command %q", args[0])
	}
}

func devRoot() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if _, err = os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", errors.New("run this command from the SyncSpace repository root")
	}
	return filepath.Join(root, ".syncspace-dev"), nil
}

func devStart(args []string) error {
	flags := flag.NewFlagSet("dev start", flag.ContinueOnError)
	portA := flags.Int("port-a", 8384, "Device A port")
	portB := flags.Int("port-b", 8385, "Device B port")
	verify := flags.Bool("verify", false, "run a real transfer and stop")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := devRoot()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	serverBinary, err := buildServer(root)
	if err != nil {
		return err
	}
	identityA, err := writeIdentity(filepath.Join(root, "device-a"), services.Identity{ID: deviceAID, Name: "SyncSpace Device A", Type: "desktop", Platform: runtime.GOOS})
	if err != nil {
		return err
	}
	identityB, err := writeIdentity(filepath.Join(root, "device-b"), services.Identity{ID: deviceBID, Name: "SyncSpace Device B", Type: "desktop", Platform: runtime.GOOS})
	if err != nil {
		return err
	}
	peerPortA, peerPortB := *portA+10000, *portB+10000
	if peerPortA > 65535 || peerPortB > 65535 {
		return errors.New("development ports are too high to allocate peer TLS ports")
	}
	peerA := localPeer(identityA, peerPortA)
	peerB := localPeer(identityB, peerPortB)
	encodedA, _ := json.Marshal([]models.Device{peerB})
	encodedB, _ := json.Marshal([]models.Device{peerA})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	commands := []*exec.Cmd{
		serverCommand(ctx, serverBinary, *portA, peerPortA, filepath.Join(root, "device-a"), string(encodedA)),
		serverCommand(ctx, serverBinary, *portB, peerPortB, filepath.Join(root, "device-b"), string(encodedB)),
	}
	for index, command := range commands {
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err = command.Start(); err != nil {
			stopCommands(commands)
			return fmt.Errorf("start device %c: %w", 'A'+index, err)
		}
	}
	defer stopCommands(commands)
	urlA, urlB := fmt.Sprintf("http://127.0.0.1:%d", *portA), fmt.Sprintf("http://127.0.0.1:%d", *portB)
	if err = waitHealthy(ctx, urlA, 30*time.Second); err != nil {
		return fmt.Errorf("device A: %w", err)
	}
	if err = waitHealthy(ctx, urlB, 30*time.Second); err != nil {
		return fmt.Errorf("device B: %w", err)
	}
	if err = waitFrontend(urlA); err != nil {
		return fmt.Errorf("Device A frontend: %w", err)
	}
	if err = waitFrontend(urlB); err != nil {
		return fmt.Errorf("Device B frontend: %w", err)
	}
	if err = ensureMutualTrust(urlA, urlB); err != nil {
		return fmt.Errorf("complete verified development pairing: %w", err)
	}
	if *verify {
		fmt.Println("Two-device lab is healthy; running end-to-end transfer verification...")
		if err = testTransfer([]string{"--source", urlA, "--destination", urlB}); err != nil {
			return fmt.Errorf("end-to-end verification: %w", err)
		}
		fmt.Println("End-to-end verification passed; both local devices are stopping.")
		return nil
	}
	fmt.Printf("\nTwo-device lab is ready.\n  Device A: %s\n  Device B: %s\n  Data: %s\n\nRun `go run ./backend/cmd/syncspace test-transfer` in another terminal. Press Ctrl+C to stop.\n", urlA, urlB, root)
	exited := make(chan error, 2)
	for _, command := range commands {
		go func(cmd *exec.Cmd) { exited <- cmd.Wait() }(command)
	}
	select {
	case <-ctx.Done():
		return nil
	case err = <-exited:
		if ctx.Err() == nil {
			if err == nil {
				return errors.New("a local device stopped unexpectedly")
			}
			return fmt.Errorf("a local device stopped: %w", err)
		}
		return nil
	}
}

func buildServer(root string) (string, error) {
	name := "syncspace-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(root, "bin", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	command := exec.Command("go", "build", "-o", path, "./backend/cmd/server")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("build local server: %w", err)
	}
	return path, nil
}

func serverCommand(ctx context.Context, binary string, port, peerPort int, dataDir, peers string) *exec.Cmd {
	command := exec.CommandContext(ctx, binary)
	command.Env = append(os.Environ(), fmt.Sprintf("SYNCSPACE_PORT=%d", port), fmt.Sprintf("SYNCSPACE_PEER_PORT=%d", peerPort), "SYNCSPACE_HOST=127.0.0.1", "SYNCSPACE_PEER_HOST=127.0.0.1", "SYNCSPACE_DEV_MODE=true", "SYNCSPACE_APP_VERSION=dev-local", "SYNCSPACE_DATA_DIR="+dataDir, "SYNCSPACE_STATIC_PEERS="+peers)
	return command
}

func devDevice(label string, args []string) error {
	flags := flag.NewFlagSet("dev device-"+label, flag.ContinueOnError)
	defaultPort, defaultPeerPort := 8384, 8385
	if label == "b" {
		defaultPort, defaultPeerPort = 8385, 8384
	}
	defaultPeerPort += 10000
	port := flags.Int("port", defaultPort, "local device port")
	peerPort := flags.Int("peer-port", defaultPeerPort, "other local device port")
	listenPeerPort := flags.Int("listen-peer-port", defaultPort+10000, "this device's encrypted peer port")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := devRoot()
	if err != nil {
		return err
	}
	binary, err := buildServer(root)
	if err != nil {
		return err
	}
	selfID, selfName, peerID, peerName := deviceAID, "SyncSpace Device A", deviceBID, "SyncSpace Device B"
	if label == "b" {
		selfID, selfName, peerID, peerName = deviceBID, "SyncSpace Device B", deviceAID, "SyncSpace Device A"
	}
	dataDir := filepath.Join(root, "device-"+label)
	_, err = writeIdentity(dataDir, services.Identity{ID: selfID, Name: selfName, Type: "desktop", Platform: runtime.GOOS})
	if err != nil {
		return err
	}
	peerIdentity := services.Identity{ID: peerID, Name: peerName, Type: "desktop", Platform: runtime.GOOS}
	peerMetadata, metadataErr := writeIdentity(filepath.Join(root, "device-"+map[string]string{"a": "b", "b": "a"}[label]), peerIdentity)
	if metadataErr == nil {
		peerIdentity = peerMetadata
	}
	peers, _ := json.Marshal([]models.Device{localPeer(peerIdentity, *peerPort)})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	command := serverCommand(ctx, binary, *port, *listenPeerPort, dataDir, string(peers))
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	fmt.Printf("Starting %s at http://127.0.0.1:%d with data in %s\n", selfName, *port, dataDir)
	return command.Run()
}

func stopCommands(commands []*exec.Cmd) {
	for _, command := range commands {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
	}
}

func writeIdentity(dataDir string, identity services.Identity) (services.Identity, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return services.Identity{}, err
	}
	contents, _ := json.MarshalIndent(identity, "", "  ")
	if err := os.WriteFile(filepath.Join(dataDir, "identity.json"), append(contents, '\n'), 0o600); err != nil {
		return services.Identity{}, err
	}
	return services.NewFileIdentityStore(filepath.Join(dataDir, "identity.json")).LoadOrCreate()
}

func localPeer(identity services.Identity, port int) models.Device {
	return models.Device{ID: identity.ID, Name: identity.Name, Type: identity.Type, Platform: identity.Platform, LocalIP: "127.0.0.1", Port: port, AppVersion: "dev-local", LastSeen: time.Now().UTC(), Online: true, ConnectionState: models.ConnectionOnline, AvailableStorage: 1 << 40, TransferCapability: true, SupportedProtocolVersion: transfer.ProtocolVersion, MaximumChunkSize: transfer.MaximumChunkSize, CompressionSupport: true, IdentityHint: identity.ShortFingerprint(), PairingAvailable: true}
}

func devReset() error {
	root, err := devRoot()
	if err != nil {
		return err
	}
	if filepath.Base(root) != ".syncspace-dev" {
		return errors.New("refusing to reset an unexpected path")
	}
	if err = os.RemoveAll(root); err != nil {
		return err
	}
	fmt.Println("Reset", root)
	return nil
}

func seedFiles() (string, error) {
	root, err := devRoot()
	if err != nil {
		return "", err
	}
	seed := filepath.Join(root, "seed")
	if err = os.MkdirAll(filepath.Join(seed, "folder"), 0o700); err != nil {
		return "", err
	}
	files := map[string][]byte{
		filepath.Join(seed, "tiny.txt"):             []byte("hello from SyncSpace\n"),
		filepath.Join(seed, "folder", "alpha.txt"):  []byte(strings.Repeat("alpha\n", 200)),
		filepath.Join(seed, "folder", "binary.bin"): bytes.Repeat([]byte{0x53, 0x59, 0x4e, 0x43}, 256*1024),
	}
	for path, contents := range files {
		if err = os.WriteFile(path, contents, 0o600); err != nil {
			return "", err
		}
	}
	fmt.Println("Seeded", seed)
	return seed, nil
}

func doctor(args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	base := flags.String("url", "http://127.0.0.1:8384", "backend URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	var health map[string]any
	if err := jsonRequest(http.MethodGet, *base+"/api/v1/health", nil, &health); err != nil {
		return err
	}
	encoded, _ := json.MarshalIndent(health, "", "  ")
	fmt.Println(string(encoded))
	return nil
}

func simulateDevice(args []string) error {
	flags := flag.NewFlagSet("dev simulate-device", flag.ContinueOnError)
	base := flags.String("url", "http://127.0.0.1:8384", "backend URL")
	scenario := flags.String("scenario", "flaky", "online, offline, trusted, untrusted, slow, flaky, rejected, interrupted, disk_full, or checksum_failure")
	name := flags.String("name", "", "device name")
	trusted := flags.Bool("trusted", false, "trust the simulated peer locally")
	if err := flags.Parse(args); err != nil {
		return err
	}
	body := map[string]any{"name": *name, "scenario": *scenario, "trusted": *trusted}
	var result map[string]any
	if err := jsonRequest(http.MethodPost, *base+"/dev/simulated-devices", body, &result); err != nil {
		return err
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(encoded))
	return nil
}

func testTransfer(args []string) error {
	flags := flag.NewFlagSet("test-transfer", flag.ContinueOnError)
	source := flags.String("source", "http://127.0.0.1:8384", "source backend")
	destination := flags.String("destination", "http://127.0.0.1:8385", "destination backend")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := devRoot()
	if err != nil {
		return err
	}
	seed, err := seedFiles()
	if err != nil {
		return err
	}
	sourcePath := filepath.Join(seed, "tiny.txt")
	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	trustedA, err := isTrusted(*source, deviceBID)
	if err != nil || !trustedA {
		return errors.New("devices are not mutually paired; start them with `syncspace dev start` first")
	}
	trustedB, err := isTrusted(*destination, deviceAID)
	if err != nil || !trustedB {
		return errors.New("devices are not mutually paired; start them with `syncspace dev start` first")
	}
	var stage struct {
		ID string `json:"id"`
	}
	if err = jsonRequest(http.MethodPost, *source+"/api/v1/transfers/staging", map[string]any{}, &stage); err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPut, *source+"/api/v1/transfers/staging/"+url.PathEscape(stage.ID)+"/files?path=tiny.txt", bytes.NewReader(contents))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if response.StatusCode/100 != 2 {
		return fmt.Errorf("stage upload returned %s", response.Status)
	}
	var queued transfer.Transfer
	if err = jsonRequest(http.MethodPost, *source+"/api/v1/transfers/staging/"+url.PathEscape(stage.ID)+"/queue", map[string]any{"deviceId": deviceBID, "roots": []string{"tiny.txt"}, "conflictPolicy": "overwrite"}, &queued); err != nil {
		return err
	}
	received := filepath.Join(root, "device-b", "received")
	if err = os.MkdirAll(received, 0o700); err != nil {
		return err
	}
	deadline := time.Now().Add(45 * time.Second)
	accepted := false
	for time.Now().Before(deadline) {
		var incoming []transfer.Transfer
		if err = jsonRequest(http.MethodGet, *destination+"/api/v1/transfers", nil, &incoming); err == nil {
			for _, item := range incoming {
				if item.ID == queued.ID && !accepted {
					var acceptedTransfer transfer.Transfer
					err = jsonRequest(http.MethodPost, *destination+"/api/v1/transfers/"+item.ID+"/accept", map[string]any{"destinationPath": received, "conflictPolicy": "overwrite"}, &acceptedTransfer)
					if err == nil {
						accepted = true
					}
				}
			}
		}
		var current transfer.Transfer
		if err = jsonRequest(http.MethodGet, *source+"/api/v1/transfers/"+queued.ID, nil, &current); err == nil {
			if current.Status == transfer.StatusCompleted {
				if current.Size <= 0 || current.Progress != current.Size {
					return fmt.Errorf("completed transfer progress is %d of %d", current.Progress, current.Size)
				}
				target := filepath.Join(received, "tiny.txt")
				actual, readErr := os.ReadFile(target)
				if readErr != nil {
					return readErr
				}
				if sha(actual) != sha(contents) {
					return errors.New("destination checksum does not match source")
				}
				if err = verifyCompletedHistory(*source, queued.ID); err != nil {
					return fmt.Errorf("source history: %w", err)
				}
				if err = verifyCompletedHistory(*destination, queued.ID); err != nil {
					return fmt.Errorf("destination history: %w", err)
				}
				fmt.Printf("Transfer %s completed: %s (%s)\n", queued.ID, target, sha(actual))
				return nil
			}
			if current.Status == transfer.StatusFailed {
				return fmt.Errorf("transfer failed: %s", current.Error)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("timed out waiting for transfer completion")
}

func verifyCompletedHistory(base, transferID string) error {
	var items []transfer.Transfer
	if err := jsonRequest(http.MethodGet, base+"/api/v1/transfers", nil, &items); err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == transferID && item.Status == transfer.StatusCompleted && item.Progress == item.Size {
			return nil
		}
	}
	return errors.New("completed transfer is missing from persistent history")
}

func exportDiagnostics(args []string) error {
	flags := flag.NewFlagSet("export-diagnostics", flag.ContinueOnError)
	base := flags.String("url", "http://127.0.0.1:8384", "backend URL")
	output := flags.String("output", "syncspace-diagnostics.zip", "output path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	response, err := http.Get(*base + "/api/v1/diagnostics/export")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return fmt.Errorf("export returned %s", response.Status)
	}
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if err = os.WriteFile(*output, contents, 0o600); err != nil {
		return err
	}
	fmt.Println("Wrote", *output)
	return nil
}

func isTrusted(base, deviceID string) (bool, error) {
	var trusted []struct {
		DeviceID string `json:"deviceId"`
	}
	if err := jsonRequest(http.MethodGet, base+"/api/v1/pairing/trusted-devices", nil, &trusted); err != nil {
		return false, err
	}
	for _, item := range trusted {
		if item.DeviceID == deviceID {
			return true, nil
		}
	}
	return false, nil
}

func ensureMutualTrust(baseA, baseB string) error {
	trustedA, err := isTrusted(baseA, deviceBID)
	if err != nil {
		return err
	}
	trustedB, err := isTrusted(baseB, deviceAID)
	if err != nil {
		return err
	}
	if trustedA && trustedB {
		return nil
	}
	if trustedA {
		if err := jsonRequest(http.MethodDelete, baseA+"/api/v1/pairing/trusted-devices/"+deviceBID, nil, nil); err != nil {
			return err
		}
	}
	if trustedB {
		if err := jsonRequest(http.MethodDelete, baseB+"/api/v1/pairing/trusted-devices/"+deviceAID, nil, nil); err != nil {
			return err
		}
	}

	var initiated pairing.Request
	if err := jsonRequest(http.MethodPost, baseA+"/api/v1/pairing/request", map[string]string{"deviceId": deviceBID}, &initiated); err != nil {
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	var incoming pairing.Request
	for time.Now().Before(deadline) {
		var requests []pairing.Request
		if err := jsonRequest(http.MethodGet, baseB+"/api/v1/pairing/requests", nil, &requests); err == nil {
			for _, request := range requests {
				if request.RequestID == initiated.RequestID {
					incoming = request
					break
				}
			}
		}
		if incoming.RequestID != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if incoming.RequestID == "" {
		return errors.New("receiving device did not surface the pairing request")
	}
	if incoming.VerificationCode == "" || incoming.VerificationCode != initiated.VerificationCode {
		return errors.New("pairing verification codes did not match")
	}
	fmt.Printf("Verified development pairing code %s on both isolated devices.\n", initiated.VerificationCode)
	var first pairing.Decision
	if err := jsonRequest(http.MethodPost, baseA+"/api/v1/pairing/accept", map[string]string{"requestId": initiated.RequestID}, &first); err != nil {
		return err
	}
	var second pairing.Decision
	if err := jsonRequest(http.MethodPost, baseB+"/api/v1/pairing/accept", map[string]string{"requestId": initiated.RequestID}, &second); err != nil {
		return err
	}
	for time.Now().Before(deadline) {
		var decision pairing.Decision
		if err := jsonRequest(http.MethodGet, baseA+"/api/v1/pairing/requests/"+initiated.RequestID, nil, &decision); err == nil && decision.TrustedDevice != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("initiating device did not persist paired trust")
}

func waitHealthy(ctx context.Context, base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		response, err := http.Get(base + "/api/v1/health")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return errors.New("health check timed out")
}

func waitFrontend(base string) error {
	response, err := http.Get(base + "/")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("frontend returned %s", response.Status)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if !bytes.Contains(contents, []byte(`id="root"`)) {
		return errors.New("embedded React entrypoint is missing")
	}
	return nil
}

func jsonRequest(method, endpoint string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("%s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	if output == nil {
		_, err = io.Copy(io.Discard, response.Body)
		return err
	}
	return json.NewDecoder(response.Body).Decode(output)
}
func sha(contents []byte) string { sum := sha256.Sum256(contents); return hex.EncodeToString(sum[:]) }
