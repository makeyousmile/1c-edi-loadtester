package tester

import (
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"loadtester/internal/mock1c"
)

func TestRunner(t *testing.T) {
	xmlData, err := os.ReadFile("../../blrwbl 517_001-4810413900004-260616000002408.xml")
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}

	mock := mock1c.NewMockServer()
	mock.SimulatedLatencyMs = 2
	ts := httptest.NewServer(mock)
	defer ts.Close()

	runner := NewRunner(string(xmlData))
	cfg := TestConfig{
		EndpointURL: ts.URL,
		Count:       10,
		Concurrency: 3,
		GLN:         "4810413900004",
		Suffix:      "001",
		TimeoutSec:  5,
	}

	err = runner.Start(cfg)
	if err != nil {
		t.Fatalf("runner.Start error: %v", err)
	}

	// Wait for finish
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !runner.IsRunning() {
			break
		}
	}

	if runner.IsRunning() {
		t.Errorf("runner is still running after timeout")
	}

	stats := runner.GetStats()
	if stats.TotalSent != 10 {
		t.Errorf("expected 10 sent, got %d", stats.TotalSent)
	}
	if stats.Success201 != 10 {
		t.Errorf("expected 10 201s, got %d", stats.Success201)
	}
	if len(stats.RecentLogs) != 10 {
		t.Errorf("expected 10 logs, got %d", len(stats.RecentLogs))
	}
	if stats.RecentLogs[0].Operation != 5 {
		t.Errorf("expected operation 5, got %d", stats.RecentLogs[0].Operation)
	}
}
