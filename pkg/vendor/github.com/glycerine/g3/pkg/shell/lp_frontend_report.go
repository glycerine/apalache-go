package shell

// Source: src/shell/lp_frontend.cpp:37-39,91-102.

import (
	"errors"
	"fmt"
	"io"

	lp "github.com/glycerine/g3/pkg/util/lp"
)

func (f *LPFrontend) cannotProcess(cause error) error {
	_, err := fmt.Fprintf(f.output, "cannot process %s\n", f.name)
	return errors.Join(cause, err)
}

func (f *LPFrontend) writeLPResult() error {
	status, err := lp.StatusString(f.solver.Status)
	if err != nil {
		return err
	}
	if _, err = io.WriteString(f.output, "status is "+status+"\n"); err != nil {
		return err
	}
	if f.solver.Status != lp.StatusOptimal {
		return nil
	}
	if f.min {
		if err = f.solver.FlipCosts(); err != nil {
			return err
		}
		f.min = false
	}
	return f.solver.PrintModel(f.output)
}
