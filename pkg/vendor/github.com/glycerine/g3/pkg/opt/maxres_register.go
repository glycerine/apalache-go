package opt

// Source: src/opt/maxsmt.cpp maxsat_engine dispatch default/maxres branch.

func init() {
	maxSMTFallbackFactory = newMaxResFallback
}

func newMaxResFallback(s *ArithmeticSolver, objective *arithmeticObjective, index int, config maxSMTConfig) (maxSMTEngine, error) {
	// Keep this packet dependency-closed: it owns only the source MaxRes and
	// primal-dual MaxRes selections. WMax and other engines remain with the
	// existing generic path until their own retained prerequisites are ported.
	switch config.engine {
	case "", "maxres", "pd-maxres":
		return newMaxResEngine(s, objective, index, config)
	default:
		return nil, nil
	}
}
