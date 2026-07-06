package devsim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/louisboii747/syncspace/backend/internal/models"
	"github.com/louisboii747/syncspace/backend/internal/transfer"
)

type simulatorDirectory struct{}

func (simulatorDirectory) Devices() []models.Device { return nil }
func (simulatorDirectory) Self() models.Device      { return models.Device{} }
func (simulatorDirectory) Refresh()                 {}

func TestSimulatorExposesEveryDevelopmentScenario(t *testing.T) {
	service, err := New(Config{
		Enabled: true,
		Base:    simulatorDirectory{},
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	scenarios := []Scenario{
		ScenarioOnline, ScenarioOffline, ScenarioTrusted, ScenarioUntrusted,
		ScenarioSlow, ScenarioFlaky, ScenarioRejected, ScenarioInterrupted,
		ScenarioDiskFull, ScenarioChecksumFailure,
	}
	for _, scenario := range scenarios {
		peer, err := service.Simulate(context.Background(), string(scenario), scenario)
		if err != nil {
			t.Fatalf("scenario %s: %v", scenario, err)
		}
		if peer.Scenario != scenario {
			t.Fatalf("scenario=%s peer=%#v", scenario, peer)
		}
		if scenario == ScenarioOffline && (peer.Device.Online || peer.Device.TransferCapability) {
			t.Fatalf("offline peer=%#v", peer)
		}
	}
	if len(service.List()) != len(scenarios) {
		t.Fatalf("peers=%d", len(service.List()))
	}
}

func TestSimulatorFaultsExerciseTheWireProtocol(t *testing.T) {
	tests := []struct {
		scenario                                           Scenario
		offerStatus, firstChunk, secondChunk, completeStatus int
	}{
		{ScenarioOnline, http.StatusCreated, http.StatusNoContent, http.StatusNoContent, http.StatusOK},
		{ScenarioFlaky, http.StatusCreated, http.StatusServiceUnavailable, http.StatusNoContent, http.StatusOK},
		{ScenarioInterrupted, http.StatusCreated, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK},
		{ScenarioDiskFull, http.StatusCreated, http.StatusInsufficientStorage, http.StatusInsufficientStorage, http.StatusOK},
		{ScenarioChecksumFailure, http.StatusCreated, http.StatusNoContent, http.StatusNoContent, http.StatusUnprocessableEntity},
		{ScenarioRejected, http.StatusConflict, 0, 0, 0},
	}
	for _, test := range tests {
		t.Run(string(test.scenario), func(t *testing.T) {
			service, err := New(Config{Enabled: true, Base: simulatorDirectory{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(service.Close)
			peer, err := service.Simulate(context.Background(), "fault", test.scenario)
			if err != nil {
				t.Fatal(err)
			}
			base := fmt.Sprintf("http://127.0.0.1:%d", peer.Device.Port)
			offer := transfer.Offer{
				TransferID: "transfer-1", DeviceID: "sender", DeviceName: "Sender",
				Filename: "demo.txt", Size: 4,
				Files: []transfer.File{{ID: "file-1", RelativePath: "demo.txt", Size: 4}},
			}
			body, err := json.Marshal(offer)
			if err != nil {
				t.Fatal(err)
			}
			response, err := http.Post(base+"/v1/transfers/offers", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != test.offerStatus {
				t.Fatalf("offer status=%d", response.StatusCode)
			}
			if test.offerStatus != http.StatusCreated {
				return
			}
			put := func() int {
				request, err := http.NewRequest(http.MethodPut, base+"/v1/transfers/transfer-1/files/file-1/chunks/0", bytes.NewReader([]byte("data")))
				if err != nil {
					t.Fatal(err)
				}
				result, err := http.DefaultClient.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				_ = result.Body.Close()
				return result.StatusCode
			}
			if status := put(); status != test.firstChunk {
				t.Fatalf("first chunk=%d", status)
			}
			if status := put(); status != test.secondChunk {
				t.Fatalf("second chunk=%d", status)
			}
			response, err = http.Post(base+"/v1/transfers/transfer-1/complete", "application/json", nil)
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != test.completeStatus {
				t.Fatalf("complete status=%d", response.StatusCode)
			}
		})
	}
}
