package logging

import "time"

type BaseEvent struct {
	TSUTC         string `json:"ts_utc"`
	TSUnixMS      int64  `json:"ts_unix_ms"`
	Seq           uint64 `json:"seq"`
	Type          string `json:"type"`
	Target        string `json:"target"`
	OutageID      string `json:"outage_id"`
	SchemaVersion int    `json:"schema_version"`
	ToolName      string `json:"tool_name"`
	ToolVersion   string `json:"tool_version"`
	HostID        string `json:"host_id"`
	ClockSource   string `json:"clock_source"`
}

func (b *BaseEvent) Base() *BaseEvent {
	return b
}

type DegradationRecord struct {
	BaseEvent
	Reason              string  `json:"reason"`
	LossPct             float64 `json:"loss_pct"`
	RttP95Ms            float64 `json:"rtt_p95_ms"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
}

type OutageSummary struct {
	BaseEvent
	StartTS            time.Time `json:"start_ts"`
	EndTS              time.Time `json:"end_ts"`
	DurationMs         int64     `json:"duration_ms"`
	LossPctMax         float64   `json:"loss_pct_max"`
	RttP95MaxMs        float64   `json:"rtt_p95_max_ms"`
	RttAvgMaxMs        float64   `json:"rtt_avg_max_ms"`
	ConsecutiveFailMax int       `json:"consecutive_failures_max"`
	PingSent           int       `json:"ping_sent"`
	PingRecv           int       `json:"ping_recv"`
	DNSErrors          int       `json:"dns_errors"`
	TracerouteCount    int       `json:"traceroute_count"`
}

type TracerouteResult struct {
	BaseEvent
	Hops     []TracerouteHop `json:"hops"`
	PathHash string          `json:"path_hash"`
	Err      string          `json:"err,omitempty"`
}

type TracerouteHop struct {
	TTL   int      `json:"ttl"`
	IP    string   `json:"ip"`
	RttMs *float64 `json:"rtt_ms"`
}

type PathChange struct {
	BaseEvent
	PrevPathHash string          `json:"prev_path_hash"`
	NewPathHash  string          `json:"new_path_hash"`
	PrevHops     []TracerouteHop `json:"prev_hops"`
	NewHops      []TracerouteHop `json:"new_hops"`
}
