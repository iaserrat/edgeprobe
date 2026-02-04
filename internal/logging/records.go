package logging

import "time"

type BaseRecord struct {
	Timestamp time.Time `json:"ts"`
	Type      string    `json:"type"`
	Target    string    `json:"target"`
	OutageID  string    `json:"outage_id"`
}

func (b *BaseRecord) SetTimestamp(ts time.Time) {
	b.Timestamp = ts
}

type DegradationRecord struct {
	BaseRecord
	Reason              string  `json:"reason"`
	LossPct             float64 `json:"loss_pct"`
	RttP95Ms            float64 `json:"rtt_p95_ms"`
	ConsecutiveFailures int     `json:"consecutive_failures"`
}

type OutageSummary struct {
	BaseRecord
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
	BaseRecord
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
	BaseRecord
	PrevPathHash string          `json:"prev_path_hash"`
	NewPathHash  string          `json:"new_path_hash"`
	PrevHops     []TracerouteHop `json:"prev_hops"`
	NewHops      []TracerouteHop `json:"new_hops"`
}
