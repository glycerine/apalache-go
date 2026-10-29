//go:build !linux

package stopwatch

// Source: src/util/stopwatch.h.

import (
	"time"
)

// Explicit portable profiling replacement: elapsed monotonic time rather than
// Windows QueryPerformanceCounter / macOS system_clock / POSIX CPU timespec.
// This adapter supplies diagnostics only, never solver resource decisions.
const ClockProfile = "Go monotonic elapsed time"

var clockEpoch = time.Now()

func platformNanoseconds() (uint64, error) { return uint64(time.Since(clockEpoch).Nanoseconds()), nil }
