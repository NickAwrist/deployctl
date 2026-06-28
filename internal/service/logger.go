package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"deployctl/internal"
)

type Logger struct {
	path string
	mu   sync.Mutex
}

func NewDaemonLogger() *Logger {
	return &Logger{path: internal.GetDaemonLogPath()}
}

func (l *Logger) Printf(format string, args ...any) {
	if l == nil || l.path == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		return
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer file.Close()

	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(file, "%s %s\n", time.Now().Format(time.RFC3339), message)
}
