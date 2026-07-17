package pairing

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/services"
)

type HTTPTransport struct{ timeout time.Duration }

func NewHTTPTransport() *HTTPTransport { return &HTTPTransport{timeout: 20 * time.Second} }

func (t *HTTPTransport) Begin(ctx context.Context, peer models.Device, request BeginRequest) (BeginResponse, error) {
	var response BeginResponse
	err := t.request(ctx, peer, "/v1/pairing/requests", request, &response)
	return response, err
}

func (t *HTTPTransport) Proof(ctx context.Context, peer models.Device, proof Proof) (PeerDecision, error) {
	var response PeerDecision
	err := t.request(ctx, peer, "/v1/pairing/proof", proof, &response)
	return response, err
}

func (t *HTTPTransport) request(ctx context.Context, peer models.Device, path string, input, output any) error {
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	endpoint := "https://" + net.JoinHostPort(peer.LocalIP, strconv.Itoa(peer.Port)) + path
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: t.timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("pairing redirects are not allowed") }, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, InsecureSkipVerify: true, VerifyConnection: func(state tls.ConnectionState) error {
		if len(state.PeerCertificates) != 1 {
			return errors.New("pairing peer did not present exactly one certificate")
		}
		publicKey, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
		if !ok || shortFingerprint(services.IdentityFingerprint(publicKey)) != peer.IdentityHint {
			return errors.New("pairing TLS identity does not match discovery")
		}
		return nil
	}}}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("peer returned %s: %s", response.Status, string(body))
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(output)
}
