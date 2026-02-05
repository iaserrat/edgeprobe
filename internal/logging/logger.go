package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	mu          sync.Mutex
	writer      io.WriteCloser
	seq         uint64
	toolName    string
	toolVersion string
	hostID      string
}

type Config struct {
	Dir         string
	MaxMB       int
	MaxFiles    int
	ToolName    string
	ToolVersion string
	HostID      string
}

func New(cfg Config) (*Logger, error) {
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	logPath := filepath.Join(cfg.Dir, "edgeprobe.jsonl")
	lj := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    cfg.MaxMB,
		MaxBackups: cfg.MaxFiles,
		Compress:   false,
	}

	return &Logger{
		writer:      lj,
		toolName:    cfg.ToolName,
		toolVersion: cfg.ToolVersion,
		hostID:      cfg.HostID,
	}, nil
}

func (l *Logger) Close() error {
	if l == nil || l.writer == nil {
		return nil
	}

	return l.writer.Close()
}

type Emittable interface {
	Base() *BaseEvent
}

func (l *Logger) Emit(record Emittable) error {
	if l == nil || l.writer == nil {
		return fmt.Errorf("logger not initialized")
	}
	if record == nil {
		return fmt.Errorf("log record is nil")
	}

	base := record.Base()
	if base == nil {
		return fmt.Errorf("log record missing base event")
	}

	now := time.Now().UTC()
	base.TSUTC = now.Format(time.RFC3339Nano)
	base.TSUnixMS = now.UnixMilli()
	base.Seq = atomic.AddUint64(&l.seq, 1)
	base.ClockSource = "system"
	base.SchemaVersion = 2
	if base.ToolName == "" {
		base.ToolName = l.toolName
	}
	if base.ToolVersion == "" {
		base.ToolVersion = l.toolVersion
	}
	if base.HostID == "" {
		base.HostID = l.hostID
	}

	if err := validateBase(base); err != nil {
		return err
	}

	b, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal log record: %w", err)
	}

	b = append(b, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	_, err = l.writer.Write(b)
	return err
}

func (l *Logger) Write(record Emittable) error {
	return l.Emit(record)
}

func validateBase(base *BaseEvent) error {
	if base.TSUTC == "" || base.TSUnixMS == 0 {
		return fmt.Errorf("invalid timestamps on log record")
	}
	if base.Type == "" {
		return fmt.Errorf("log record missing type")
	}
	if base.Target == "" {
		return fmt.Errorf("log record missing target")
	}
	if base.OutageID == "" {
		return fmt.Errorf("log record missing outage_id")
	}
	if base.ToolName == "" {
		return fmt.Errorf("log record missing tool_name")
	}
	if base.ToolVersion == "" {
		return fmt.Errorf("log record missing tool_version")
	}
	if base.HostID == "" {
		return fmt.Errorf("log record missing host_id")
	}
	if base.SchemaVersion != 2 {
		return fmt.Errorf("log record schema_version must be 2")
	}
	if base.ClockSource != "system" {
		return fmt.Errorf("log record clock_source must be system")
	}

	return nil
}
