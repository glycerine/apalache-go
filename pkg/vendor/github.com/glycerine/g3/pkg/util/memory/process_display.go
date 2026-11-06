package memory

// Source: src/util/memory_manager.cpp:173-191; src/shell/z3_log_frontend.cpp:37.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

// DisplayMaxUsage writes the retained shell's process peak-memory report.
// The source default stream uses six significant decimal digits and writes
// the label, value, and suffix as separate insertions.
func (p *ProcessAccounting) DisplayMaxUsage(out io.Writer) error {
	if !p.Valid() {
		return ErrRuntimeOwner
	}
	if out == nil {
		return arena.ErrConfig
	}
	peak := p.MaximumUsed()
	value := strconv.FormatFloat(float64(peak)/float64(1024*1024), 'g', 6, 64)
	for _, part := range []string{"max. heap size:     ", value, " Mbytes\n"} {
		n, e := out.Write([]byte(part))
		if e != nil {
			return e
		}
		if n != len(part) {
			return io.ErrShortWrite
		}
	}
	return nil
}
