package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"drip/internal/client/cli"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	// Performance optimizations
	setupPerformanceOptimizations()

	cli.SetVersion(Version, GitCommit, BuildTime)

	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// setupPerformanceOptimizations configures runtime settings for optimal performance
func setupPerformanceOptimizations() {
	// Set GOMAXPROCS to use all available CPU cores
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)

	// Reduce GC frequency for high-throughput scenarios
	// Default is 100, setting to 200 reduces GC overhead at cost of more memory
	// This is beneficial since we now use buffer pools (less garbage)
	debug.SetGCPercent(200)

	// Set memory limit to prevent OOM (adjust based on your server)
	// This is a soft limit - Go will try to stay under this
	debug.SetMemoryLimit(8 * 1024 * 1024 * 1024) // 8GB limit
}
