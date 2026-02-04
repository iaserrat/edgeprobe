package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	mu     sync.Mutex
	writer io.WriteCloser
}

type Config struct {
	Dir      string
	MaxMB    int
	MaxFiles int
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

	return &Logger{writer: lj}, nil
}

func (l *Logger) Close() error {
	if l == nil || l.writer == nil {
		return nil
	}

	return l.writer.Close()
}

func (l *Logger) Write(record any) error {
	if l == nil || l.writer == nil {
		return fmt.Errorf("logger not initialized")
	}

	if v, ok := record.(interface{ SetTimestamp(time.Time) }); ok {
		v.SetTimestamp(time.Now().UTC())
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
