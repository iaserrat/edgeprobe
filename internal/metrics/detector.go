package metrics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	lossThresholdPct      = 5.0
	rttP95ThresholdMs     = 200.0
	consecutiveFailThresh = 3
)

type EventType string

const (
	EventOutageStart   EventType = "outage_start"
	EventOutageEnd     EventType = "outage_end"
	EventOutageSummary EventType = "outage_summary"
)

type Event interface {
	Type() EventType
}

type OutageStart struct {
	Target              string
	OutageID            string
	Reason              string
	LossPct             float64
	RttP95Ms            float64
	ConsecutiveFailures int
}

func (o OutageStart) Type() EventType { return EventOutageStart }

type OutageEnd struct {
	Target              string
	OutageID            string
	Reason              string
	LossPct             float64
	RttP95Ms            float64
	ConsecutiveFailures int
}

func (o OutageEnd) Type() EventType { return EventOutageEnd }

type OutageSummary struct {
	Target             string
	OutageID           string
	StartTS            time.Time
	EndTS              time.Time
	DurationMs         int64
	LossPctMax         float64
	RttP95MaxMs        float64
	RttAvgMaxMs        float64
	ConsecutiveFailMax int
	PingSent           int
	PingRecv           int
	DNSErrors          int
	TracerouteCount    int
}

func (o OutageSummary) Type() EventType { return EventOutageSummary }

type Detector struct {
	window    time.Duration
	mu        sync.Mutex
	states    map[string]*targetState
	idCounter int64
}

type pingSample struct {
	ts  time.Time
	ok  bool
	rtt float64
}

type targetState struct {
	windowSamples []pingSample
	consecFail    int
	inOutage      bool
	outageID      string
	outageStart   time.Time
	clearSince    *time.Time

	lossPctMax      float64
	rttP95MaxMs     float64
	rttAvgMaxMs     float64
	consecFailMax   int
	pingSent        int
	pingRecv        int
	dnsErrors       int
	tracerouteCount int
}

func NewDetector(windowSecs int) *Detector {
	return &Detector{
		window: time.Duration(windowSecs) * time.Second,
		states: make(map[string]*targetState),
	}
}

func (d *Detector) ProcessPing(target string, ts time.Time, ok bool, rttMs float64) []Event {
	d.mu.Lock()
	defer d.mu.Unlock()

	state := d.stateFor(target)
	state.windowSamples = append(state.windowSamples, pingSample{ts: ts, ok: ok, rtt: rttMs})
	state.windowSamples = pruneWindow(state.windowSamples, ts, d.window)

	if ok {
		state.consecFail = 0
	} else {
		state.consecFail++
	}

	stats := computeStats(state.windowSamples)
	reason, outage := evaluateOutage(stats, state.consecFail)

	var events []Event

	if !state.inOutage && outage {
		state.inOutage = true
		state.outageID = d.nextOutageID(target, ts)
		state.outageStart = ts
		state.clearSince = nil

		state.lossPctMax = stats.lossPct
		state.rttP95MaxMs = stats.rttP95
		state.rttAvgMaxMs = stats.rttAvg
		state.consecFailMax = state.consecFail
		state.pingSent = 0
		state.pingRecv = 0
		state.dnsErrors = 0
		state.tracerouteCount = 0

		if ok {
			state.pingSent = 1
			state.pingRecv = 1
		} else {
			state.pingSent = 1
			state.pingRecv = 0
		}

		events = append(events, OutageStart{
			Target:              target,
			OutageID:            state.outageID,
			Reason:              reason,
			LossPct:             stats.lossPct,
			RttP95Ms:            stats.rttP95,
			ConsecutiveFailures: state.consecFail,
		})

		return events
	}

	if state.inOutage {
		if ok {
			state.pingSent++
			state.pingRecv++
		} else {
			state.pingSent++
		}
		if stats.lossPct > state.lossPctMax {
			state.lossPctMax = stats.lossPct
		}
		if stats.rttP95 > state.rttP95MaxMs {
			state.rttP95MaxMs = stats.rttP95
		}
		if stats.rttAvg > state.rttAvgMaxMs {
			state.rttAvgMaxMs = stats.rttAvg
		}
		if state.consecFail > state.consecFailMax {
			state.consecFailMax = state.consecFail
		}

		if outage {
			state.clearSince = nil
		} else {
			if state.clearSince == nil {
				t := ts
				state.clearSince = &t
			}
			if ts.Sub(*state.clearSince) >= d.window {
				endEvent := OutageEnd{
					Target:              target,
					OutageID:            state.outageID,
					Reason:              "cleared",
					LossPct:             stats.lossPct,
					RttP95Ms:            stats.rttP95,
					ConsecutiveFailures: state.consecFail,
				}
				summary := OutageSummary{
					Target:             target,
					OutageID:           state.outageID,
					StartTS:            state.outageStart,
					EndTS:              ts,
					DurationMs:         ts.Sub(state.outageStart).Milliseconds(),
					LossPctMax:         state.lossPctMax,
					RttP95MaxMs:        state.rttP95MaxMs,
					RttAvgMaxMs:        state.rttAvgMaxMs,
					ConsecutiveFailMax: state.consecFailMax,
					PingSent:           state.pingSent,
					PingRecv:           state.pingRecv,
					DNSErrors:          state.dnsErrors,
					TracerouteCount:    state.tracerouteCount,
				}

				state.inOutage = false
				state.outageID = ""
				state.outageStart = time.Time{}
				state.clearSince = nil

				events = append(events, endEvent, summary)
			}
		}
	}

	return events
}

func (d *Detector) ProcessDNS(ts time.Time, ok bool) {
	if ok {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, state := range d.states {
		if state.inOutage {
			state.dnsErrors++
		}
	}
}

func (d *Detector) RecordTraceroute(target string, outageID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	state := d.states[target]
	if state == nil {
		return
	}
	if state.inOutage && state.outageID == outageID {
		state.tracerouteCount++
	}
}

func (d *Detector) ActiveOutageID(target string) string {
	d.mu.Lock()
	defer d.mu.Unlock()

	state := d.states[target]
	if state == nil || !state.inOutage {
		return ""
	}

	return state.outageID
}

func (d *Detector) stateFor(target string) *targetState {
	state := d.states[target]
	if state == nil {
		state = &targetState{}
		d.states[target] = state
	}

	return state
}

func (d *Detector) nextOutageID(target string, ts time.Time) string {
	d.idCounter++
	return fmt.Sprintf("%s-%d-%06d", target, ts.UnixNano(), d.idCounter)
}

type windowStats struct {
	lossPct float64
	rttP95  float64
	rttAvg  float64
}

func computeStats(samples []pingSample) windowStats {
	if len(samples) == 0 {
		return windowStats{}
	}

	sent := len(samples)
	recv := 0
	var rtts []float64
	var rttSum float64

	for _, s := range samples {
		if s.ok {
			recv++
			rtts = append(rtts, s.rtt)
			rttSum += s.rtt
		}
	}

	lossPct := (1.0 - (float64(recv) / float64(sent))) * 100.0
	var rttP95 float64
	var rttAvg float64
	if len(rtts) > 0 {
		sort.Float64s(rtts)
		idx := int(float64(len(rtts)-1) * 0.95)
		rttP95 = rtts[idx]
		rttAvg = rttSum / float64(len(rtts))
	}

	return windowStats{lossPct: lossPct, rttP95: rttP95, rttAvg: rttAvg}
}

func evaluateOutage(stats windowStats, consecutiveFailures int) (string, bool) {
	var reasons []string
	if stats.lossPct >= lossThresholdPct {
		reasons = append(reasons, "loss_pct")
	}
	if stats.rttP95 >= rttP95ThresholdMs {
		reasons = append(reasons, "rtt_p95_ms")
	}
	if consecutiveFailures >= consecutiveFailThresh {
		reasons = append(reasons, "consecutive_failures")
	}
	if len(reasons) == 0 {
		return "", false
	}

	return strings.Join(reasons, ","), true
}

func pruneWindow(samples []pingSample, now time.Time, window time.Duration) []pingSample {
	cutoff := now.Add(-window)
	idx := 0
	for idx < len(samples) {
		if samples[idx].ts.After(cutoff) || samples[idx].ts.Equal(cutoff) {
			break
		}
		idx++
	}
	if idx == 0 {
		return samples
	}

	return append([]pingSample(nil), samples[idx:]...)
}
