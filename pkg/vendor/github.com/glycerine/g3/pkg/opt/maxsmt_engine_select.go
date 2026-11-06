package opt

// Source: src/opt/maxsmt.cpp maxsat_engine dispatch.

type maxSMTEngineFactory func(*ArithmeticSolver, *arithmeticObjective, int, maxSMTConfig) (maxSMTEngine, error)

var maxSMTFallbackFactory maxSMTEngineFactory

func sourceMaxSMTEngine(name string) string {
	switch name {
	case "", "maxres", "pd-maxres", "wmax", "sortmax":
		return name
	default:
		// maxsmt::operator() warns and constructs MaxRes for unknown names.
		return "maxres"
	}
}

func selectMaxSMTEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int, config maxSMTConfig) (maxSMTEngine, error) {
	config.engine = sourceMaxSMTEngine(config.engine)
	if config.engine == "wmax" {
		return newWMaxEngine(s, objective, index)
	}
	if config.engine == "sortmax" {
		engine, usable, err := newSortMaxEngine(s, objective, index)
		if err != nil || usable {
			return engine, err
		}
	}
	if maxSMTFallbackFactory != nil {
		return maxSMTFallbackFactory(s, objective, index, config)
	}
	return nil, nil
}
