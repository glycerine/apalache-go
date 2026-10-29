package sat

// Source: src/sat/sat_simplifier.cpp:700; retained util/trace site.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSimplifier) traceMarkNotLearnedBinary(other Literal, watch satWatched) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("mark_not_learned_bug", "subsume_with_binaries", "src/sat/sat_simplifier.cpp", 700,
		func() bool { return true }, func(w io.Writer) error {
			learned, err := watch.Learned()
			if err != nil {
				return err
			}
			flag := "0"
			if learned {
				flag = "1"
			}
			_, err = io.WriteString(w, "marking as not learned: "+other.String()+" "+flag+"\n")
			return err
		})
}
