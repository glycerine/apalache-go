package sat

// Source: src/sat/sat_simplifier.cpp:769-770; retained util/trace site.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceDuplicateBinary(watchIndex uint32, other Literal) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("subsumption", "elim_dup_bins", "src/sat/sat_simplifier.cpp", 769,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, "eliminating: "+Literal(watchIndex).Negated().String()+" "+other.String()+"\n")
			return err
		})
}
