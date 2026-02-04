package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type PingConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

func RunPing(ctx context.Context, target string, cfg PingConfig, out chan<- PingResult) error {
	ipAddr, err := net.ResolveIPAddr("ip4", target)
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("icmp listen requires root or CAP_NET_RAW: %w", err)
		}
		return fmt.Errorf("icmp listen: %w", err)
	}
	defer conn.Close()

	id := os.Getpid() & 0xffff
	seq := 0
	payload := []byte("edgeprobe")
	next := time.Now()

	for {
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}

		seq++
		msg := icmp.Message{
			Type: ipv4.ICMPTypeEcho,
			Code: 0,
			Body: &icmp.Echo{
				ID: id,
				Seq: seq,
				Data: payload,
			},
		}

		b, err := msg.Marshal(nil)
		if err != nil {
			return fmt.Errorf("icmp marshal: %w", err)
		}

		start := time.Now()
		if _, err := conn.WriteTo(b, ipAddr); err != nil {
			out <- PingResult{Target: target, Time: time.Now().UTC(), OK: false}
			next = next.Add(cfg.Interval)
			continue
		}

		_ = conn.SetReadDeadline(time.Now().Add(cfg.Timeout))
		buf := make([]byte, 1500)
		n, _, err := conn.ReadFrom(buf)
		elapsed := time.Since(start)

		if err != nil {
			out <- PingResult{Target: target, Time: time.Now().UTC(), OK: false}
			next = next.Add(cfg.Interval)
			continue
		}

		recv, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), buf[:n])
		if err != nil {
			out <- PingResult{Target: target, Time: time.Now().UTC(), OK: false}
			next = next.Add(cfg.Interval)
			continue
		}

		if recv.Type == ipv4.ICMPTypeEchoReply {
			if echo, ok := recv.Body.(*icmp.Echo); ok && echo.ID == id {
				out <- PingResult{Target: target, Time: time.Now().UTC(), OK: true, RTTMs: float64(elapsed.Milliseconds())}
			} else {
				out <- PingResult{Target: target, Time: time.Now().UTC(), OK: false}
			}
		} else {
			out <- PingResult{Target: target, Time: time.Now().UTC(), OK: false}
		}

		next = next.Add(cfg.Interval)
	}
}
