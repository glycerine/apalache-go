package lp

// Source: src/test/lp.cpp:3080-3240. Retained test_lp_local preparation.

import "io"

// LPDriverPreparation records the source-selected call, before executing it.
// Arguments and Settings remain available on report/conversion failure. This
// stage does not execute tests or open input files. Action names are the actual
// source functions; an empty action is the ordinary final return.
type LPDriverPreparation struct {
	Arguments                               *DriverArguments
	Settings                                *Settings
	Action, File                            string
	ExitCode                                uint32
	MaxIterations, TimeLimit                uint32
	Minimum, Exact, Dual, CompareWithPrimal bool
}

func PrepareLPDriver(argv []string, out io.Writer) (*LPDriverPreparation, error) {
	p := &LPDriverPreparation{}
	out = driverReportOutput(out)
	// The source's resized, unused vector<mpq> has no retained consumer.
	if err := writeConstraintText(out, "resize\n"); err != nil {
		return p, err
	}
	p.Arguments = NewDriverArguments(argv)
	a := p.Arguments
	if !a.Parse() {
		if err := writeDriverLine(out, a.ErrorMessage); err != nil {
			return p, err
		}
		if err := writeConstraintText(out, a.UsageString()); err != nil {
			return p, err
		}
		p.ExitCode = 1
		return p, nil // finalize itself has an empty body
	}
	if err := a.Print(out); err != nil {
		return p, err
	}
	for _, branch := range []struct{ flag, action string }{
		{"--test_mpq", "test_rationals"},
		{"--test_mpq_np", "test_rationals_no_numeric_pairs"},
		{"--test_mpq_np_plus", "test_rationals_no_numeric_pairs_plus"},
		{"--test_int_set", "test_int_set"},
		{"--bp", "test_bound_propagation"},
	} {
		if a.OptionIsUsed(branch.flag) {
			p.Action = branch.action
			return p, nil
		}
	}
	if p.File = a.OptionValue("--checklu"); p.File != "" {
		p.Action = "check_lu_from_file"
		return p, nil
	}
	if sourceDebugEnabled && a.OptionIsUsed("--test_swaps") {
		p.Action = "test_swaps"
		return p, nil
	}
	if a.OptionIsUsed("--test_perm") {
		p.Action = "test_permutations"
		return p, nil
	}
	if a.OptionIsUsed("--test_file_directory") {
		p.Action, p.File = "test_files_from_directory", a.OptionValue("--test_file_directory")
		return p, nil // selected source function has a completely commented body
	}
	if p.File = a.OptionValue("--filelist"); p.File != "" {
		p.Action = "filelist"
		return p, nil
	}
	if a.OptionIsUsed("-tbq") {
		p.Action = "test_binary_priority_queue"
		return p, nil
	}
	if sourceDebugEnabled {
		var err error
		p.Settings, err = NewSettings()
		if err != nil {
			return p, err
		}
		if err = ApplyDriverSettings(a, p.Settings, out); err != nil {
			return p, err
		}
		for _, branch := range []struct{ flag, action string }{
			{"--test_lu", "test_lu"}, {"--test_small_lu", "test_small_lu"},
			{"--lar", "test_lar_solver"}, {"--test_larger_lu", "test_larger_lu"},
			{"--test_larger_lu_with_holes", "test_larger_lu_with_holes"},
		} {
			if a.OptionIsUsed(branch.flag) {
				if branch.flag == "--lar" {
					if err = writeDriverLine(out, "calling test_lar_solver"); err != nil {
						return p, err
					}
				}
				p.Action = branch.action
				return p, nil
			}
		}
	}
	for _, branch := range []struct{ flag, action string }{
		{"--eti", "test_evidence_for_total_inf_simple"}, {"--test_lp_0", "test_lp_0"},
		{"--smap", "test_stacked"}, {"--term", "test_term"},
	} {
		if a.OptionIsUsed(branch.flag) {
			p.Action = branch.action
			return p, nil
		}
	}
	var err error
	p.TimeLimit, p.MaxIterations, err = a.TimeAndIterationLimits()
	if err != nil {
		return p, err
	}
	p.Dual, p.Exact = a.OptionIsUsed("--dual"), a.OptionIsUsed("--mpq")
	if p.File = a.OptionValue("--file"); p.File != "" {
		p.Minimum, p.CompareWithPrimal = a.OptionIsUsed("--min"), a.OptionIsUsed("--compare_with_primal")
		p.Action = "solve_mps"
		return p, nil
	}
	if a.OptionIsUsed("--solve_some_mps") {
		p.Action = "solve_some_mps"
	}
	return p, nil
}
