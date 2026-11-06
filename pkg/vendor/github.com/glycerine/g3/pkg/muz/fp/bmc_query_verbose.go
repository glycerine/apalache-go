package fp

// Source: src/muz/bmc/dl_bmc_engine.cpp, bmc::query nonlinear dispatch.

import "github.com/glycerine/g3/pkg/util/diagnostic"

const nonlinearBMCWarning = "WARNING: non-linear BMC is highly inefficient\n"

func warnNonlinearBMC() {
	enabled, err := diagnostic.ProcessVerboseEnabled(0)
	if err != nil || !enabled {
		return
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return
	}
	// ostream insertion failure only marks the source stream bad; it does not
	// change the fixedpoint result.
	_ = out.AppendCString(nonlinearBMCWarning)
}
