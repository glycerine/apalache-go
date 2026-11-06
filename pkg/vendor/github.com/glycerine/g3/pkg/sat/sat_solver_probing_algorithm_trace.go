package sat

// Source: src/sat/sat_probing.cpp:111; retained util/trace site.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (s *satSolverProbing) traceProcessing(v BoolVar) {
	tags := s.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("probing", "process_core", "src/sat/sat_probing.cpp", 111,
		func() bool { return true }, func(w io.Writer) error {
			counter := -int64(s.propagation().probingCounter)
			_, err := io.WriteString(w, "processing: "+strconv.FormatUint(uint64(v), 10)+" counter: "+strconv.FormatInt(counter, 10)+"\n")
			return err
		})
}
