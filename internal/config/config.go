package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Logging    LoggingConfig     `toml:"logging"`
	Ping       PingConfig        `toml:"ping"`
	DNS        DNSConfig         `toml:"dns"`
	Traceroute TracerouteConfig  `toml:"traceroute"`
	Targets    []TargetConfig    `toml:"targets"`
}

type LoggingConfig struct {
	Dir      string `toml:"dir"`
	MaxMB    int    `toml:"max_mb"`
	MaxFiles int    `toml:"max_files"`
}

type PingConfig struct {
	IntervalMS int `toml:"interval_ms"`
	TimeoutMS  int `toml:"timeout_ms"`
	WindowSecs int `toml:"window_secs"`
}

type DNSConfig struct {
	IntervalMS int      `toml:"interval_ms"`
	TimeoutMS  int      `toml:"timeout_ms"`
	Queries    []string `toml:"queries"`
	Resolvers  []string `toml:"resolvers"`
}

type TracerouteConfig struct {
	CooldownSecs int `toml:"cooldown_secs"`
	MaxHops      int `toml:"max_hops"`
	TimeoutMS    int `toml:"timeout_ms"`
}

type TargetConfig struct {
	Name string `toml:"name"`
	Host string `toml:"host"`
}

func Load(path string) (Config, error) {
	var cfg Config

	if _, err := os.Stat(path); err != nil {
		return cfg, fmt.Errorf("config file not found: %w", err)
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var errs []string

	if strings.TrimSpace(c.Logging.Dir) == "" {
		errs = append(errs, "logging.dir is required")
	}
	if c.Logging.MaxMB <= 0 {
		errs = append(errs, "logging.max_mb must be > 0")
	}
	if c.Logging.MaxFiles <= 0 {
		errs = append(errs, "logging.max_files must be > 0")
	}
	if c.Ping.IntervalMS <= 0 {
		errs = append(errs, "ping.interval_ms must be > 0")
	}
	if c.Ping.TimeoutMS <= 0 {
		errs = append(errs, "ping.timeout_ms must be > 0")
	}
	if c.Ping.WindowSecs <= 0 {
		errs = append(errs, "ping.window_secs must be > 0")
	}
	if c.DNS.IntervalMS <= 0 {
		errs = append(errs, "dns.interval_ms must be > 0")
	}
	if c.DNS.TimeoutMS <= 0 {
		errs = append(errs, "dns.timeout_ms must be > 0")
	}
	if len(c.DNS.Queries) == 0 {
		errs = append(errs, "dns.queries must not be empty")
	}
	if len(c.DNS.Resolvers) == 0 {
		errs = append(errs, "dns.resolvers must not be empty")
	}
	if c.Traceroute.CooldownSecs <= 0 {
		errs = append(errs, "traceroute.cooldown_secs must be > 0")
	}
	if c.Traceroute.MaxHops <= 0 {
		errs = append(errs, "traceroute.max_hops must be > 0")
	}
	if c.Traceroute.TimeoutMS <= 0 {
		errs = append(errs, "traceroute.timeout_ms must be > 0")
	}
	if len(c.Targets) == 0 {
		errs = append(errs, "targets must not be empty")
	}
	for i, t := range c.Targets {
		if strings.TrimSpace(t.Name) == "" {
			errs = append(errs, fmt.Sprintf("targets[%d].name is required", i))
		}
		if strings.TrimSpace(t.Host) == "" {
			errs = append(errs, fmt.Sprintf("targets[%d].host is required", i))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}
