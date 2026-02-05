package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEmitPopulatesBaseFields(t *testing.T) {
	dir := t.TempDir()
	logger, err := New(Config{
		Dir:         dir,
		MaxMB:       1,
		MaxFiles:    1,
		ToolName:    "edgeprobe",
		ToolVersion: "test",
		HostID:      "host-1",
	})
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	defer logger.Close()

	events := []Emittable{
		&DegradationRecord{
			BaseEvent: BaseEvent{
				Type:     "degradation_start",
				Target:   "example.com",
				OutageID: "example.com-123-000001",
			},
			Reason:              "loss_pct",
			LossPct:             50,
			RttP95Ms:            300,
			ConsecutiveFailures: 4,
		},
		&OutageSummary{
			BaseEvent: BaseEvent{
				Type:     "outage_summary",
				Target:   "example.com",
				OutageID: "example.com-123-000001",
			},
			StartTS:            time.Unix(1, 0).UTC(),
			EndTS:              time.Unix(2, 0).UTC(),
			DurationMs:         1000,
			LossPctMax:         50,
			RttP95MaxMs:        300,
			RttAvgMaxMs:        150,
			ConsecutiveFailMax: 4,
			PingSent:           10,
			PingRecv:           2,
			DNSErrors:          1,
			TracerouteCount:    1,
		},
		&TracerouteResult{
			BaseEvent: BaseEvent{
				Type:     "traceroute_result",
				Target:   "example.com",
				OutageID: "example.com-123-000001",
			},
			Hops:     []TracerouteHop{{TTL: 1, IP: "1.1.1.1"}},
			PathHash: "hash",
		},
		&PathChange{
			BaseEvent: BaseEvent{
				Type:     "path_change",
				Target:   "example.com",
				OutageID: "example.com-123-000001",
			},
			PrevPathHash: "prev",
			NewPathHash:  "new",
			PrevHops:     []TracerouteHop{{TTL: 1, IP: "1.1.1.1"}},
			NewHops:      []TracerouteHop{{TTL: 1, IP: "1.1.1.2"}},
		},
	}

	for _, evt := range events {
		if err := logger.Emit(evt); err != nil {
			t.Fatalf("emit: %v", err)
		}
	}

	logPath := filepath.Join(dir, "edgeprobe.jsonl")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != len(events) {
		t.Fatalf("expected %d log lines, got %d", len(events), len(lines))
	}

	for _, line := range lines {
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("unmarshal log line: %v", err)
		}

		tsUTC, ok := payload["ts_utc"].(string)
		if !ok || tsUTC == "" || tsUTC == "0001-01-01T00:00:00Z" {
			t.Fatalf("invalid ts_utc: %v", payload["ts_utc"])
		}
		if _, err := time.Parse(time.RFC3339Nano, tsUTC); err != nil {
			t.Fatalf("ts_utc not RFC3339Nano: %v", err)
		}

		tsUnix, ok := payload["ts_unix_ms"].(float64)
		if !ok || tsUnix == 0 {
			t.Fatalf("invalid ts_unix_ms: %v", payload["ts_unix_ms"])
		}

		if _, ok := payload["seq"].(float64); !ok {
			t.Fatalf("missing seq")
		}
		if payload["type"] == "" || payload["target"] == "" || payload["outage_id"] == "" {
			t.Fatalf("missing required identifiers: %#v", payload)
		}
		if payload["schema_version"] != float64(2) {
			t.Fatalf("expected schema_version 2, got %v", payload["schema_version"])
		}
		if payload["tool_name"] != "edgeprobe" {
			t.Fatalf("expected tool_name edgeprobe, got %v", payload["tool_name"])
		}
		if payload["tool_version"] != "test" {
			t.Fatalf("expected tool_version test, got %v", payload["tool_version"])
		}
		if payload["host_id"] != "host-1" {
			t.Fatalf("expected host_id host-1, got %v", payload["host_id"])
		}
		if payload["clock_source"] != "system" {
			t.Fatalf("expected clock_source system, got %v", payload["clock_source"])
		}
	}
}
