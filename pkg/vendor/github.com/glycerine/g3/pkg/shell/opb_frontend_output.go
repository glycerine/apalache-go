package shell

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *OPBFrontend) WriteResult() error {
	if f == nil || !f.finished || f.output == nil {
		return ErrOPBFrontend
	}
	status := "unknown"
	if f.status == lbool.True {
		status = "sat"
	} else if f.status == lbool.False {
		status = "unsat"
	}
	if _, err := fmt.Fprintln(f.output, status); err != nil {
		return err
	}
	if f.options.DisplayStatistics {
		if err := f.context.WriteStatisticsWithTotalTime(f.output, f.elapsed.Seconds()); err != nil {
			return err
		}
	}
	valuesAvailable := f.status == lbool.True
	if f.status == lbool.Undef && f.summary.Objectives != 0 {
		valuesAvailable = true
		for i := 0; i < f.summary.Objectives; i++ {
			if _, err := f.context.ObjectiveBound(uint32(i), true); err != nil {
				valuesAvailable = false
				break
			}
		}
	}
	if valuesAvailable && f.summary.Objectives != 0 {
		if err := f.context.WriteOptimizationFrontendValues(f.output); err != nil {
			return err
		}
	}
	if !valuesAvailable {
		for i := 0; i < f.summary.Objectives; i++ {
			if _, err := fmt.Fprintln(f.output, "  [(* (- 1) oo):oo]"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *OPBFrontend) WriteProgress() error {
	if f == nil || !f.finished || f.output == nil {
		return ErrOPBFrontend
	}
	if !f.initialized {
		return nil
	}
	if f.options.DisplayStatistics {
		if err := f.context.WriteStatisticsWithTotalTime(f.output, f.elapsed.Seconds()); err != nil {
			return err
		}
	}
	return f.context.WriteOptimizationFrontendValues(f.output)
}
