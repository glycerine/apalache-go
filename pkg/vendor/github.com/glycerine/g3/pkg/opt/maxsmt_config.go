package opt

// Source: src/opt/opt_params.hpp values retained by MaxSMT engines.

type maxSMTConfig struct {
	engine               string
	satCoreEnabled       bool
	enableSLS            bool
	hillClimb            bool
	addUpperBoundBlock   bool
	maxNumCores          uint32
	maxCoreSize          uint32
	maximizeAssignment   bool
	maxCorrectionSetSize uint32
	pivotOnCorrectionSet bool
}
