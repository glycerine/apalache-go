package shell

// Source: src/shell/dimacs_frontend.cpp:49-59,126-137.
import (
	"fmt"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
	"io"
)

func (f *DIMACSFrontend) writeResult(s *sat.Solver, status lbool.LBool, tracking [][]sat.Literal) error {
	switch status {
	case lbool.True:
		if _, err := io.WriteString(f.output, "sat\n"); err != nil {
			return err
		}
		model, err := s.AppendModel(nil)
		if err != nil {
			return err
		}
		for i := 1; i < len(model); i++ {
			if model[i] == lbool.True {
				if _, err = fmt.Fprintf(f.output, "%d ", i); err != nil {
					return err
				}
			} else if model[i] == lbool.False {
				if _, err = fmt.Fprintf(f.output, "-%d ", i); err != nil {
					return err
				}
			}
		}
		_, err = io.WriteString(f.output, "\n")
		return err
	case lbool.False:
		if _, err := io.WriteString(f.output, "unsat\n"); err != nil {
			return err
		}
		if tracking != nil {
			return f.writeCore(s, tracking)
		}
		return nil
	default:
		_, err := io.WriteString(f.output, "unknown\n")
		return err
	}
}
