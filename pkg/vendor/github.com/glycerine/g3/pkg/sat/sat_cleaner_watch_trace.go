package sat

// Source: src/sat/sat_cleaner.cpp:48,61; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *satCleaner) traceProcessingWatchList(index uint32) {
	tags := c.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("cleanup_bug", "cleanup_watches", "src/sat/sat_cleaner.cpp", 48,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, "processing wlist of "+Literal(index).String()+"\n")
			return err
		})
}

func (c *satCleaner) traceKeepingBinaryWatch(index uint32, other Literal) {
	tags := c.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("cleanup_bug", "cleanup_watches", "src/sat/sat_cleaner.cpp", 61,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, "keeping: "+Literal(index).Negated().String()+" "+other.String()+"\n")
			return err
		})
}
