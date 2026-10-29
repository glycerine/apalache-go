//go:build linux

package stopwatch

// Source: src/util/stopwatch.h.

import (
	"syscall"
)

// Process CPU time is obtained without unsafe or cgo through Getrusage's
// process-wide user+system totals. This replaces CLOCK_PROCESS_CPUTIME_ID with
// microsecond-resolution CPU readings, also used by LP resource time limits.
const ClockProfile = "process CPU, getrusage microsecond resolution"

func platformNanoseconds() (uint64, error) {
	var r syscall.Rusage
	if e := syscall.Getrusage(syscall.RUSAGE_SELF, &r); e != nil {
		return 0, e
	}
	return uint64(r.Utime.Sec+r.Stime.Sec)*1e9 + uint64(r.Utime.Usec+r.Stime.Usec)*1000, nil
}
