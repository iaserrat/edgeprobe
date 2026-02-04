package traceroute

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	MaxHops int
	Timeout time.Duration
}

type Hop struct {
	TTL   int
	IP    string
	RttMs float64
}

type Result struct {
	Hops     []Hop
	PathHash string
	Err      string
}

var hopLine = regexp.MustCompile(`^\s*(\d+)\s+(.+)$`)

func Run(ctx context.Context, target string, cfg Config) Result {
	args := []string{"-n", "-m", strconv.Itoa(cfg.MaxHops), "-w", fmt.Sprintf("%.0f", cfg.Timeout.Seconds()), target}
	cmd := exec.CommandContext(ctx, "traceroute", args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		res := Result{Err: err.Error()}
		if len(out) == 0 {
			return res
		}
		hops := parseOutput(string(out))
		res.Hops = hops
		res.PathHash = hashPath(hops)
		return res
	}

	hops := parseOutput(string(out))
	return Result{Hops: hops, PathHash: hashPath(hops)}
}

func parseOutput(out string) []Hop {
	scanner := bufio.NewScanner(strings.NewReader(out))
	var hops []Hop

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "traceroute") {
			continue
		}

		matches := hopLine.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}

		ttl, _ := strconv.Atoi(matches[1])
		rest := matches[2]
		ip, rtt := parseHop(rest)

		hops = append(hops, Hop{TTL: ttl, IP: ip, RttMs: rtt})
	}

	return hops
}

func parseHop(rest string) (string, float64) {
	if strings.Contains(rest, "*") {
		return "", 0
	}

	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return "", 0
	}

	ip := fields[0]
	var rtt float64
	for i := 1; i < len(fields); i++ {
		if fields[i] == "ms" && i > 0 {
			val, _ := strconv.ParseFloat(fields[i-1], 64)
			rtt = val
			break
		}
	}

	return ip, rtt
}

func hashPath(hops []Hop) string {
	var sb strings.Builder
	for _, h := range hops {
		sb.WriteString(fmt.Sprintf("%d:%s|", h.TTL, h.IP))
	}

	hash := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(hash[:])
}
