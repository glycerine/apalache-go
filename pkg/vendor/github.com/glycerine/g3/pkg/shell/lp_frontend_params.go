package shell

// Source: src/shell/lp_frontend.cpp:59-65,81-89.

import (
	"strconv"

	lp "github.com/glycerine/g3/pkg/util/lp"
)

func (f *LPFrontend) globalUint(name string) (uint32, error) {
	value, err := f.globals.SourceGetValueName(name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseUint(value, 10, 32)
	return uint32(n), err
}

func (f *LPFrontend) configureSolver() error {
	minimum, err := f.params.Min()
	if err != nil {
		return err
	}
	f.min = minimum
	if minimum {
		if err = f.solver.FlipCosts(); err != nil {
			return err
		}
	}
	f.solver.Settings.SetResourceLimit(lpFrontendLimit{f.limit})
	f.solver.Settings.MessageOut = f.output
	f.solver.Settings.DebugOut = f.output
	freq, err := f.params.RepFreq()
	if err != nil {
		return err
	}
	f.solver.Settings.ReportFrequency = int32(freq)
	f.solver.Settings.PrintStatistics, err = f.params.PrintStats()
	if err != nil {
		return err
	}
	f.solver.Settings.SimplexStrategy = lp.StrategyLU
	return nil
}
