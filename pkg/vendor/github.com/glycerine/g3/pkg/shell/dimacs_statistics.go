package shell

// Source: src/shell/dimacs_frontend.cpp:27-37.
import (
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/statistics"
	"time"
)

func (f *DIMACSFrontend) writeStatistics(s *sat.Solver) error {
	st := statistics.New()
	if err := s.CollectStatistics(st); err != nil {
		return err
	}
	st.UpdateDouble("total time", float64(time.Since(f.start).Milliseconds()/10)/100)
	return st.DisplaySMT2(f.output)
}
