package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iaserrat/edgeprobe/internal/config"
	"github.com/iaserrat/edgeprobe/internal/logging"
	"github.com/iaserrat/edgeprobe/internal/metrics"
	"github.com/iaserrat/edgeprobe/internal/probe"
	"github.com/iaserrat/edgeprobe/internal/traceroute"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "/etc/edgeprobe/config.toml", "Path to config file")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if err := run(*configPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}

	logger, err := newLogger(cfg)
	if err != nil {
		return err
	}
	defer logger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pingCh := make(chan probe.PingResult, 256)
	dnsCh := make(chan probe.DNSResult, 256)
	eventCh := make(chan metrics.Event, 256)
	errCh := make(chan error, 1)

	detector := metrics.NewDetector(cfg.Ping.WindowSecs)

	startPingWorkers(ctx, cfg, pingCh, errCh)
	startDNSWorker(ctx, cfg, dnsCh, errCh)
	startAggregator(ctx, detector, pingCh, dnsCh, eventCh)
	traceCh := startTracerouteWorker(ctx, cfg, logger, detector)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-sigCh:
			cancel()
			return nil
		case err := <-errCh:
			cancel()
			return err
		case e := <-eventCh:
			switch evt := e.(type) {
			case metrics.OutageStart:
				if err := logDegradation(logger, "degradation_start", evt.Target, evt.OutageID, evt.Reason, evt.LossPct, evt.RttP95Ms, evt.ConsecutiveFailures); err != nil {
					return err
				}
				traceCh <- traceRequest{target: evt.Target, outageID: evt.OutageID}
			case metrics.OutageEnd:
				if err := logDegradation(logger, "degradation_end", evt.Target, evt.OutageID, evt.Reason, evt.LossPct, evt.RttP95Ms, evt.ConsecutiveFailures); err != nil {
					return err
				}
			case metrics.OutageSummary:
				if err := logger.Emit(&logging.OutageSummary{
					BaseEvent: logging.BaseEvent{
						Type:     "outage_summary",
						Target:   evt.Target,
						OutageID: evt.OutageID,
					},
					StartTS:            evt.StartTS,
					EndTS:              evt.EndTS,
					DurationMs:         evt.DurationMs,
					LossPctMax:         evt.LossPctMax,
					RttP95MaxMs:        evt.RttP95MaxMs,
					RttAvgMaxMs:        evt.RttAvgMaxMs,
					ConsecutiveFailMax: evt.ConsecutiveFailMax,
					PingSent:           evt.PingSent,
					PingRecv:           evt.PingRecv,
					DNSErrors:          evt.DNSErrors,
					TracerouteCount:    evt.TracerouteCount,
				}); err != nil {
					return err
				}
			}
		}
	}
}

type traceRequest struct {
	target   string
	outageID string
}

func newLogger(cfg config.Config) (*logging.Logger, error) {
	hostID, err := os.Hostname()
	if err != nil || hostID == "" {
		hostID = "unknown"
	}
	return logging.New(logging.Config{
		Dir:         cfg.Logging.Dir,
		MaxMB:       cfg.Logging.MaxMB,
		MaxFiles:    cfg.Logging.MaxFiles,
		ToolName:    "edgeprobe",
		ToolVersion: version,
		HostID:      hostID,
	})
}

func startPingWorkers(ctx context.Context, cfg config.Config, pingCh chan<- probe.PingResult, errCh chan<- error) {
	pingCfg := probe.PingConfig{
		Interval: time.Duration(cfg.Ping.IntervalMS) * time.Millisecond,
		Timeout:  time.Duration(cfg.Ping.TimeoutMS) * time.Millisecond,
	}

	for _, t := range cfg.Targets {
		target := t.Host
		go func() {
			if err := probe.RunPing(ctx, target, pingCfg, pingCh); err != nil {
				errCh <- fmt.Errorf("ping %s: %w", target, err)
			}
		}()
	}
}

func startDNSWorker(ctx context.Context, cfg config.Config, dnsCh chan<- probe.DNSResult, errCh chan<- error) {
	dnsCfg := probe.DNSConfig{
		Interval:  time.Duration(cfg.DNS.IntervalMS) * time.Millisecond,
		Timeout:   time.Duration(cfg.DNS.TimeoutMS) * time.Millisecond,
		Queries:   cfg.DNS.Queries,
		Resolvers: cfg.DNS.Resolvers,
	}
	go func() {
		if err := probe.RunDNS(ctx, dnsCfg, dnsCh); err != nil {
			errCh <- fmt.Errorf("dns: %w", err)
		}
	}()
}

func startAggregator(ctx context.Context, detector *metrics.Detector, pingCh <-chan probe.PingResult, dnsCh <-chan probe.DNSResult, eventCh chan<- metrics.Event) {
	go func() {
		for {
			select {
			case p := <-pingCh:
				events := detector.ProcessPing(p.Target, p.Time, p.OK, p.RTTMs)
				for _, e := range events {
					eventCh <- e
				}
			case d := <-dnsCh:
				detector.ProcessDNS(d.Time, d.OK)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func startTracerouteWorker(ctx context.Context, cfg config.Config, logger *logging.Logger, detector *metrics.Detector) chan<- traceRequest {
	reqCh := make(chan traceRequest, 64)
	trCfg := traceroute.Config{
		MaxHops: cfg.Traceroute.MaxHops,
		Timeout: time.Duration(cfg.Traceroute.TimeoutMS) * time.Millisecond,
	}
	cooldown := time.Duration(cfg.Traceroute.CooldownSecs) * time.Second
	traceTimeout := time.Duration(cfg.Traceroute.MaxHops)*trCfg.Timeout + 2*time.Second

	go func() {
		lastTrace := make(map[string]time.Time)
		lastPath := make(map[string]string)
		lastHops := make(map[string][]logging.TracerouteHop)

		for {
			select {
			case <-ctx.Done():
				return
			case req := <-reqCh:
				if time.Since(lastTrace[req.target]) < cooldown {
					continue
				}
				lastTrace[req.target] = time.Now()

				trCtx, cancelTrace := context.WithTimeout(ctx, traceTimeout)
				res := traceroute.Run(trCtx, req.target, trCfg)
				cancelTrace()

				detector.RecordTraceroute(req.target, req.outageID)

				hops := toLogHops(res.Hops)
				_ = logger.Emit(&logging.TracerouteResult{
					BaseEvent: logging.BaseEvent{
						Type:     "traceroute_result",
						Target:   req.target,
						OutageID: req.outageID,
					},
					Hops:     hops,
					PathHash: res.PathHash,
					Err:      res.Err,
				})

				if res.Err == "" && res.PathHash != "" {
					prev := lastPath[req.target]
					if prev != "" && prev != res.PathHash {
						_ = logger.Emit(&logging.PathChange{
							BaseEvent: logging.BaseEvent{
								Type:     "path_change",
								Target:   req.target,
								OutageID: req.outageID,
							},
							PrevPathHash: prev,
							NewPathHash:  res.PathHash,
							PrevHops:     lastHops[req.target],
							NewHops:      hops,
						})
					}

					lastPath[req.target] = res.PathHash
					lastHops[req.target] = hops
				}
			}
		}
	}()

	return reqCh
}

func toLogHops(hops []traceroute.Hop) []logging.TracerouteHop {
	out := make([]logging.TracerouteHop, 0, len(hops))
	for _, h := range hops {
		var rtt *float64
		if h.IP != "" {
			val := h.RttMs
			rtt = &val
		}
		out = append(out, logging.TracerouteHop{TTL: h.TTL, IP: h.IP, RttMs: rtt})
	}
	return out
}

func logDegradation(logger *logging.Logger, recordType string, target string, outageID string, reason string, lossPct float64, rttP95 float64, consecutiveFailures int) error {
	return logger.Emit(&logging.DegradationRecord{
		BaseEvent: logging.BaseEvent{
			Type:     recordType,
			Target:   target,
			OutageID: outageID,
		},
		Reason:              reason,
		LossPct:             lossPct,
		RttP95Ms:            rttP95,
		ConsecutiveFailures: consecutiveFailures,
	})
}
