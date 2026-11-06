package lp

// Source: src/test/lp.cpp:1080-1147,2098-2112; lp_solver.h:158-164.

import (
	"fmt"
	"io"
)

// ApplyDriverSettings preserves the LP driver's assignment and reporting order.
// In particular, diagnostic writes and endl flushes precede their assignments.
// A nil output suppresses reports; the native command-line caller uses stdout.
func ApplyDriverSettings(args *DriverArguments, settings *Settings, out io.Writer) error {
	if args == nil || settings == nil {
		return ErrBounds
	}
	settings.SimplexStrategy = StrategyLU
	n, present, err := args.uintValue("--rep_frq")
	if err != nil {
		return err
	}
	if present {
		settings.ReportFrequency = int32(n)
	} else if args.OptionIsUsed("--mpq") {
		settings.ReportFrequency = 80
	} else {
		settings.ReportFrequency = 1000
	}
	settings.PrintStatistics = true
	if n, present, err = args.uintValue("--percent_for_enter"); err != nil {
		return err
	} else if present {
		settings.PercentOfEnteringToCheck = n
	}
	if n, present, err = args.uintValue("--partial_pivot"); err != nil {
		return err
	} else if present {
		if err = writeDriverLine(out, fmt.Sprintf("setting partial pivot constant to %d", n)); err != nil {
			return err
		}
		settings.PartialPivotingConstant = int32(n)
	}
	if n, present, err = args.uintValue("--density"); err != nil {
		return err
	} else if present {
		density := float64(n) / 100
		if err = writeDriverLine(out, "setting density to "+floatLPNumberText(density)); err != nil {
			return err
		}
		settings.DensityThreshold = density
	}
	if n, present, err = args.uintValue("--maxng"); err != nil {
		return err
	} else if present {
		settings.MaxIterationsWithoutImprovements = n
	}
	d, present, err := args.doubleValue("--harris_toler")
	if err != nil {
		return err
	}
	if present {
		if err = writeDriverLine(out, "setting harris_feasibility_tolerance to "+floatLPNumberText(d)); err != nil {
			return err
		}
		settings.HarrisFeasibilityTolerance = d
	}
	if n, present, err = args.uintValue("--random_seed"); err != nil {
		return err
	} else if present {
		settings.RandomSeed(n)
	}
	if n, present, err = args.uintValue("--simplex_strategy"); err != nil {
		return err
	} else if present {
		settings.SimplexStrategy = SimplexStrategy(n)
	}
	return nil
}

// SetupFromDriver changes limits before optional objective negation, then applies
// driver settings. No parent numerical capture encloses the owning FlipCosts
// child operation; it retains its own exact failure prefixes and cleanup.
func (m *RationalLPSolver) SetupFromDriver(maxIterations, timeLimit uint32, minimum bool, args *DriverArguments, out io.Writer) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.Settings == nil {
		return ErrBounds
	}
	if maxIterations > 0 {
		m.Settings.MaxTotalIterations = maxIterations
	}
	if timeLimit > 0 {
		m.Settings.TimeLimit = float64(timeLimit)
	}
	if minimum {
		if err = m.FlipCosts(); err != nil {
			return err
		}
	}
	return ApplyDriverSettings(args, m.Settings, out)
}

func (m *FloatLPSolver) SetupFromDriver(maxIterations, timeLimit uint32, minimum bool, args *DriverArguments, out io.Writer) error {
	if m.Settings == nil {
		return ErrBounds
	}
	if maxIterations > 0 {
		m.Settings.MaxTotalIterations = maxIterations
	}
	if timeLimit > 0 {
		m.Settings.TimeLimit = float64(timeLimit)
	}
	if minimum {
		if err := m.FlipCosts(); err != nil {
			return err
		}
	}
	return ApplyDriverSettings(args, m.Settings, out)
}
