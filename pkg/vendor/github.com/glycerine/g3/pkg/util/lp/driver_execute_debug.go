//go:build g3debug

package lp

// Source: src/test/lp.cpp:3133-3138,3160-3194. Source debug-only branches.

type lpDriverDebug struct {
	Swaps    *SwapTestDriver
	LU       *LUTestDriver
	SmallLU  *SmallLUTestDriver
	LargerLU *LargerLUTestDriver
	HolesLU  *HolesLUTestDriver
	LARInput *LARInputDriver
}

func (d *LPTestDriver) runDebug() error {
	p := d.Preparation
	switch p.Action {
	case "test_swaps":
		d.Swaps = NewSwapTestDriver(d.config.Output)
		return d.Swaps.Run(d.random)
	case "test_lu":
		d.LU = NewLUTestDriver(p.Settings, d.config.Output)
		err := d.LU.Run(d.random)
		d.ExitCode = uint32(d.LU.ExitCode)
		return err
	case "test_small_lu":
		d.SmallLU = NewSmallLUTestDriver(p.Settings, d.config.Output)
		return d.SmallLU.Run()
	case "test_larger_lu":
		d.LargerLU = NewLargerLUTestDriver(p.Settings, d.config.Output, false)
		err := d.LargerLU.Run(d.random)
		d.ExitCode = uint32(d.LargerLU.ExitCode)
		return err
	case "test_larger_lu_with_holes":
		d.HolesLU = NewHolesLUTestDriver(p.Settings, d.config.Output)
		return d.HolesLU.Run()
	case "test_lar_solver":
		d.LARInput = NewLARInputDriver(p.Arguments, d.manager, d.config)
		return d.LARInput.Run()
	}
	return ErrBounds
}

func (d *LPTestDriver) closeDebug() error {
	if d.LARInput != nil {
		if err := d.LARInput.Close(); err != nil {
			return err
		}
	}
	if d.HolesLU != nil {
		if err := d.HolesLU.Close(); err != nil {
			return err
		}
	}
	if d.LargerLU != nil {
		if err := d.LargerLU.Close(); err != nil {
			return err
		}
	}
	if d.SmallLU != nil {
		if err := d.SmallLU.Close(); err != nil {
			return err
		}
	}
	if d.LU != nil {
		if err := d.LU.Close(); err != nil {
			return err
		}
	}
	if d.Swaps != nil {
		if err := d.Swaps.Close(); err != nil {
			return err
		}
	}
	return nil
}
