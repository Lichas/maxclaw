package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Loggers struct {
	Gateway  *log.Logger
	Session  *log.Logger
	Tools    *log.Logger
	Channels *log.Logger
	Cron     *log.Logger
	Web      *log.Logger

	files []*os.File
}

var (
	once    sync.Once
	loggers *Loggers
	initErr error
)

// Init sets up ~/.maxclaw/logs files. Safe to call multiple times.
func Init(baseDir string) (*Loggers, error) {
	once.Do(func() {
		if baseDir == "" {
			initErr = fmt.Errorf("log base dir is empty")
			return
		}
		logDir := filepath.Join(baseDir, "logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			initErr = fmt.Errorf("failed to create log dir: %w", err)
			return
		}

		open := func(name string) (*log.Logger, *os.File, error) {
			path := filepath.Join(logDir, name)
			f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return nil, nil, err
			}
			l := log.New(f, "", log.LstdFlags|log.Lmicroseconds)
			return l, f, nil
		}

		l := &Loggers{}
		var files []*os.File

		var err error
		l.Gateway, files, err = attach(open, files, "gateway.log")
		if err != nil {
			initErr = err
			return
		}
		l.Session, files, err = attach(open, files, "session.log")
		if err != nil {
			initErr = err
			return
		}
		l.Tools, files, err = attach(open, files, "tools.log")
		if err != nil {
			initErr = err
			return
		}
		l.Channels, files, err = attach(open, files, "channels.log")
		if err != nil {
			initErr = err
			return
		}
		l.Cron, files, err = attach(open, files, "cron.log")
		if err != nil {
			initErr = err
			return
		}
		l.Web, files, err = attach(open, files, "webui.log")
		if err != nil {
			initErr = err
			return
		}

		l.files = files
		loggers = l

		l.Gateway.Printf("logging initialized at %s", logDir)
	})

	return loggers, initErr
}

func attach(open func(string) (*log.Logger, *os.File, error), files []*os.File, name string) (*log.Logger, []*os.File, error) {
	l, f, err := open(name)
	if err != nil {
		return nil, files, fmt.Errorf("open %s: %w", name, err)
	}
	return l, append(files, f), nil
}

// Get returns initialized loggers (may be nil if Init failed or not called).
func Get() *Loggers {
	return loggers
}

// Truncate keeps logs readable by capping long strings.
func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// Timestamp returns a stable, human-friendly timestamp.
func Timestamp() string {
	return time.Now().Format(time.RFC3339)
}
