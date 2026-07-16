package memstats

import (
	"fmt"
	"log/slog"
	"runtime"
	"time"
)

// Log writes a single-line memory summary to the application log.
func Log(label string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	slog.Info(label,
		"alloc_mb", fmt.Sprintf("%.2f", float64(m.Alloc)/1024/1024),
		"heap_mb", fmt.Sprintf("%.2f", float64(m.HeapAlloc)/1024/1024),
		"sys_mb", fmt.Sprintf("%.2f", float64(m.Sys)/1024/1024),
		"total_alloc_mb", fmt.Sprintf("%.2f", float64(m.TotalAlloc)/1024/1024),
		"num_gc", m.NumGC,
	)
}

// BytesToMB converts bytes to megabytes string with two decimals.
func BytesToMB(b uint64) string {
	return fmt.Sprintf("%.2f", float64(b)/1024/1024)
}

// LogEvery logs memory at the given interval until the program exits.
// ponytail: simple background ticker; no shutdown coordination needed for a log-only goroutine.
func LogEvery(d time.Duration) {
	t := time.NewTicker(d)
	defer t.Stop()
	for range t.C {
		Log("memory idle tick")
	}
}
