package opt

// Source: src/opt/maxsmt.h maxsmt_solver and src/opt/maxsmt.cpp engine
// dispatch retained by ArithmeticSolver.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
)

type maxSMTEngine interface {
	optimize(*ArithmeticResult) (lbool.LBool, bool, error)
	statistics() MaxSMTStatistics
	close() error
}

func newMaxSMTEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int) (maxSMTEngine, error) {
	// A formula normalization pass can reduce every soft term to a Boolean
	// constant. The weighted set then contains only its exact offset, so no
	// MaxSMT search engine should be constructed.
	if objective == nil || len(objective.softTerms) == 0 {
		return nil, nil
	}
	config, err := loadMaxSMTConfig(s)
	if err != nil {
		return nil, err
	}
	return selectMaxSMTEngine(s, objective, index, config)
}
