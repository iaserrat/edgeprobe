package probe

import (
	"context"
	"fmt"
	"time"

	"github.com/miekg/dns"
)

type DNSConfig struct {
	Interval  time.Duration
	Timeout   time.Duration
	Queries   []string
	Resolvers []string
}

func RunDNS(ctx context.Context, cfg DNSConfig, out chan<- DNSResult) error {
	client := &dns.Client{Timeout: cfg.Timeout}
	if len(cfg.Queries) == 0 || len(cfg.Resolvers) == 0 {
		return fmt.Errorf("dns queries or resolvers empty")
	}

	idx := 0
	resIdx := 0
	next := time.Now()

	for {
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}

		query := cfg.Queries[idx%len(cfg.Queries)]
		resolver := cfg.Resolvers[resIdx%len(cfg.Resolvers)]
		idx++
		resIdx++

		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn(query), dns.TypeA)

		_, _, err := client.Exchange(msg, resolver)
		ok := err == nil
		out <- DNSResult{Time: time.Now().UTC(), OK: ok}

		next = next.Add(cfg.Interval)
	}
}
