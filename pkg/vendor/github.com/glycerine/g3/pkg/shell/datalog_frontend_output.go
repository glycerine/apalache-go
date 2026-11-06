package shell

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/lbool"
)

// WriteAnswers is an explicit Go inspection adapter. Native outputtuples
// rendering remains separate from this copied predicate-status view.
func (f *DatalogFrontend) WriteAnswers(out io.Writer) error {
	if f == nil || !f.saturated || out == nil {
		return ErrDatalogFrontend
	}
	for _, result := range f.results {
		status := "unknown"
		if result.Status == lbool.True {
			status = "sat"
		}
		if result.Status == lbool.False {
			status = "unsat"
		}
		if _, err := fmt.Fprintf(out, "%s: %s\n", result.Name, status); err != nil {
			return err
		}
	}
	return nil
}
