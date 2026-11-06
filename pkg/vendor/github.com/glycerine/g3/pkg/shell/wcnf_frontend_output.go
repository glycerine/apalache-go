package shell

import (
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (f *WCNFFrontend) WriteResult() error {
	if f == nil || !f.finished || f.output == nil {
		return ErrWCNFFrontend
	}
	s := "unknown"
	if f.status == lbool.True {
		s = "sat"
	} else if f.status == lbool.False {
		s = "unsat"
	}
	if _, e := fmt.Fprintln(f.output, s); e != nil {
		return e
	}
	if f.options.DisplayStatistics {
		if e := f.context.WriteStatisticsWithTotalTime(f.output, f.elapsed.Seconds()); e != nil {
			return e
		}
	}
	valuesAvailable := f.status == lbool.True
	if f.status == lbool.Undef && f.summary.SoftClauses > 0 {
		_, err := f.context.ObjectiveBound(0, true)
		valuesAvailable = err == nil
	}
	if valuesAvailable && f.summary.SoftClauses > 0 {
		if e := f.context.WriteOptimizationFrontendValues(f.output); e != nil {
			return e
		}
	}
	if !valuesAvailable && f.summary.SoftClauses > 0 {
		var upper big.Int
		for _, clause := range f.document.Clauses {
			if clause.Weight < f.document.TopWeight {
				upper.Add(&upper, new(big.Int).SetUint64(clause.Weight))
			}
		}
		if upper.Sign() == 0 {
			if _, e := fmt.Fprintln(f.output, "   0"); e != nil {
				return e
			}
		} else if _, e := fmt.Fprintf(f.output, "  [0:%s]\n", upper.String()); e != nil {
			return e
		}
	}
	return nil
}

func (f *WCNFFrontend) WriteProgress() error {
	if f == nil || !f.finished || f.output == nil {
		return ErrWCNFFrontend
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
