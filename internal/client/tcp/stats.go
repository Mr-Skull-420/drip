package tcp

import (
	"sync"
	"sync/atomic"
	"time"
)

// TrafficStats tracks traffic statistics for a tunnel connection
type TrafficStats struct {
	// Total bytes
	totalBytesIn  int64
	totalBytesOut int64

	// Request counts
	totalRequests int64

	// For speed calculation
	lastBytesIn  int64
	lastBytesOut int64
	lastTime     time.Time
	speedMu      sync.Mutex

	// Current speed (bytes per second)
	speedIn  int64
	speedOut int64

	// Start time
	startTime time.Time
}

// NewTrafficStats creates a new traffic stats tracker
func NewTrafficStats() *TrafficStats {
	now := time.Now()
	return &TrafficStats{
		startTime: now,
		lastTime:  now,
	}
}

// AddBytesIn adds incoming bytes to the counter
func (s *TrafficStats) AddBytesIn(n int64) {
	atomic.AddInt64(&s.totalBytesIn, n)
}

// AddBytesOut adds outgoing bytes to the counter
func (s *TrafficStats) AddBytesOut(n int64) {
	atomic.AddInt64(&s.totalBytesOut, n)
}

// AddRequest increments the request counter
func (s *TrafficStats) AddRequest() {
	atomic.AddInt64(&s.totalRequests, 1)
}

// GetTotalBytesIn returns total incoming bytes
func (s *TrafficStats) GetTotalBytesIn() int64 {
	return atomic.LoadInt64(&s.totalBytesIn)
}

// GetTotalBytesOut returns total outgoing bytes
func (s *TrafficStats) GetTotalBytesOut() int64 {
	return atomic.LoadInt64(&s.totalBytesOut)
}

// GetTotalRequests returns total request count
func (s *TrafficStats) GetTotalRequests() int64 {
	return atomic.LoadInt64(&s.totalRequests)
}

// GetTotalBytes returns total bytes (in + out)
func (s *TrafficStats) GetTotalBytes() int64 {
	return s.GetTotalBytesIn() + s.GetTotalBytesOut()
}

// UpdateSpeed calculates current transfer speed
// Should be called periodically (e.g., every second)
func (s *TrafficStats) UpdateSpeed() {
	s.speedMu.Lock()
	defer s.speedMu.Unlock()

	now := time.Now()
	elapsed := now.Sub(s.lastTime).Seconds()
	if elapsed < 0.1 {
		return // Avoid division by zero or too frequent updates
	}

	currentIn := atomic.LoadInt64(&s.totalBytesIn)
	currentOut := atomic.LoadInt64(&s.totalBytesOut)

	deltaIn := currentIn - s.lastBytesIn
	deltaOut := currentOut - s.lastBytesOut

	s.speedIn = int64(float64(deltaIn) / elapsed)
	s.speedOut = int64(float64(deltaOut) / elapsed)

	s.lastBytesIn = currentIn
	s.lastBytesOut = currentOut
	s.lastTime = now
}

// GetSpeedIn returns current incoming speed in bytes per second
func (s *TrafficStats) GetSpeedIn() int64 {
	s.speedMu.Lock()
	defer s.speedMu.Unlock()
	return s.speedIn
}

// GetSpeedOut returns current outgoing speed in bytes per second
func (s *TrafficStats) GetSpeedOut() int64 {
	s.speedMu.Lock()
	defer s.speedMu.Unlock()
	return s.speedOut
}

// GetUptime returns how long the connection has been active
func (s *TrafficStats) GetUptime() time.Duration {
	return time.Since(s.startTime)
}

// Snapshot returns a snapshot of all stats
type StatsSnapshot struct {
	TotalBytesIn  int64
	TotalBytesOut int64
	TotalBytes    int64
	TotalRequests int64
	SpeedIn       int64 // bytes per second
	SpeedOut      int64 // bytes per second
	Uptime        time.Duration
}

// GetSnapshot returns a snapshot of current stats
func (s *TrafficStats) GetSnapshot() StatsSnapshot {
	s.speedMu.Lock()
	speedIn := s.speedIn
	speedOut := s.speedOut
	s.speedMu.Unlock()

	totalIn := atomic.LoadInt64(&s.totalBytesIn)
	totalOut := atomic.LoadInt64(&s.totalBytesOut)

	return StatsSnapshot{
		TotalBytesIn:  totalIn,
		TotalBytesOut: totalOut,
		TotalBytes:    totalIn + totalOut,
		TotalRequests: atomic.LoadInt64(&s.totalRequests),
		SpeedIn:       speedIn,
		SpeedOut:      speedOut,
		Uptime:        time.Since(s.startTime),
	}
}

// FormatBytes formats bytes to human readable string
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return formatFloat(float64(bytes)/float64(GB)) + " GB"
	case bytes >= MB:
		return formatFloat(float64(bytes)/float64(MB)) + " MB"
	case bytes >= KB:
		return formatFloat(float64(bytes)/float64(KB)) + " KB"
	default:
		return formatInt(bytes) + " B"
	}
}

// FormatSpeed formats speed (bytes per second) to human readable string
func FormatSpeed(bytesPerSec int64) string {
	if bytesPerSec == 0 {
		return "0 B/s"
	}
	return FormatBytes(bytesPerSec) + "/s"
}

func formatFloat(f float64) string {
	if f >= 100 {
		return formatInt(int64(f))
	} else if f >= 10 {
		return formatOneDecimal(f)
	}
	return formatTwoDecimal(f)
}

func formatInt(i int64) string {
	return intToStr(i)
}

func formatOneDecimal(f float64) string {
	i := int64(f * 10)
	whole := i / 10
	frac := i % 10
	return intToStr(whole) + "." + intToStr(frac)
}

func formatTwoDecimal(f float64) string {
	i := int64(f * 100)
	whole := i / 100
	frac := i % 100
	if frac < 10 {
		return intToStr(whole) + ".0" + intToStr(frac)
	}
	return intToStr(whole) + "." + intToStr(frac)
}

func intToStr(i int64) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return "-" + intToStr(-i)
	}

	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
