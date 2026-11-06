package opt

// Source: src/opt/maxres.cpp maxres::strategy_t and constructor trace id.

type maxResStrategy uint8

const (
	maxResPrimal maxResStrategy = iota
	maxResPrimalDual
)

func maxResStrategyFromConfig(config maxSMTConfig) maxResStrategy {
	if config.engine == "pd-maxres" {
		return maxResPrimalDual
	}
	return maxResPrimal
}

func (s maxResStrategy) name() string {
	if s == maxResPrimalDual {
		return "pd-maxres"
	}
	return "maxres"
}
