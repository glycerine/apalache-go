//go:build g3trace

package diagnostic

import "os"

// Source: src/util/trace.cpp:22-24. The process-wide ofstream is constructed
// when the trace build loads, before any tag is enabled or record is emitted.
func newProcessTraceTags() *TraceTags { return newProcessTraceTagsAt(".z3-trace") }

func newProcessTraceTagsAt(path string) *TraceTags {
	t := NewTraceTags()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err == nil {
		t.writer = f
	}
	t.openErr = err
	return t
}
