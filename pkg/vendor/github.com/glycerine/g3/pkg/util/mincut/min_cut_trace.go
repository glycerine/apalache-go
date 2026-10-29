package mincut

// Source: src/util/min_cut.cpp:35, after edge publication.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *MinCut) traceAddEdge(from, to uint32) {
	tags := m.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("spacer.mincut", "add_edge", "src/util/min_cut.cpp", 35,
		func() bool { return true }, func(w io.Writer) error {
			for _, part := range [...]string{"adding edge (", strconv.FormatUint(uint64(from), 10), ",", strconv.FormatUint(uint64(to), 10), ")\n"} {
				n, err := io.WriteString(w, part)
				if err != nil {
					return err
				}
				if n != len(part) {
					return io.ErrShortWrite
				}
			}
			return nil
		})
}
