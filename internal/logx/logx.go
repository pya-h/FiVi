package logx

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

var (
	mu sync.Mutex
	l  = log.New(os.Stdout, "", 0)
)

func Infof(format string, args ...any) {
	print("INFO", format, args...)
}

func Warnf(format string, args ...any) {
	print("WARN", format, args...)
}

func Errorf(format string, args ...any) {
	print("ERROR", format, args...)
}

func SetOutput(w io.Writer) func() {
	if w == nil {
		w = io.Discard
	}

	mu.Lock()
	prev := l.Writer()
	l.SetOutput(w)
	mu.Unlock()

	return func() {
		mu.Lock()
		defer mu.Unlock()
		l.SetOutput(prev)
	}
}

func print(level, format string, args ...any) {
	mu.Lock()
	defer mu.Unlock()
	l.Printf("%s [%s] %s", time.Now().Format("2006-01-02 15:04:05"), level, fmt.Sprintf(format, args...))
}
