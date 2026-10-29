package smtcore

// Source: src/smt/smt_context.cpp init_search(), inc_limits(), restart(), and
// the restart exit in bounded_search().

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/luby"
)

type smtRestartState struct {
	conflicts, threshold, outerThreshold, lubyIndex uint32
}

func newSMTRestartState(params SMTParamsState) smtRestartState {
	return smtRestartState{threshold: params.RestartInitial, outerThreshold: params.RestartInitial, lubyIndex: 1}
}

func smtRestartUnsigned(value float64) (uint32, error) {
	value = math.Trunc(value)
	if math.IsNaN(value) || value < 0 || value > float64(^uint32(0)) {
		return 0, ast.ErrASTDomain
	}
	return uint32(value), nil
}

func (r *smtRestartState) due(scopeLevel, baseLevel uint32) bool {
	return r.conflicts > r.threshold && scopeLevel-baseLevel > 2
}

func (r *smtRestartState) advance(params SMTParamsState) error {
	if r.conflicts >= r.threshold {
		switch params.RestartStrategy {
		case 0:
			value, err := smtRestartUnsigned(float64(r.threshold) * params.RestartFactor)
			if err != nil {
				return err
			}
			r.threshold = value
		case 1:
			value, err := smtRestartUnsigned(float64(r.threshold) * params.RestartFactor)
			if err != nil {
				return err
			}
			r.threshold = value
			if r.threshold > r.outerThreshold {
				r.threshold = params.RestartInitial
				r.outerThreshold, err = smtRestartUnsigned(float64(r.outerThreshold) * params.RestartFactor)
				if err != nil {
					return err
				}
			}
		case 2:
			r.lubyIndex++
			value, err := luby.Value(r.lubyIndex)
			if err != nil {
				return err
			}
			r.threshold = value * params.RestartInitial
		case 3:
		case 4:
			value, err := smtRestartUnsigned(float64(r.threshold) + params.RestartFactor)
			if err != nil {
				return err
			}
			r.threshold = value
		}
	}
	r.conflicts = 0
	return nil
}

func (k *SMTKernel) restartSearch(baseLevel uint32, state *smtRestartState) error {
	return k.restartSearchAtBase(baseLevel, state, false)
}

func (k *SMTKernel) restartSearchAtBase(baseLevel uint32, state *smtRestartState, force bool) error {
	if state == nil {
		return ast.ErrASTHandle
	}
	if err := state.advance(k.smtState); err != nil {
		return err
	}
	if force || !k.smtState.RestartAdaptive || k.internalizer.searchAgility < k.smtState.RestartAgilityThreshold {
		k.internalizer.statistics.NumRestarts++
		if count := k.internalizer.scopeLevel - baseLevel; count != 0 {
			if err := k.internalizer.PopScopes(count); err != nil {
				return err
			}
		}
		if err := k.internalizer.RestartTheories(); err != nil {
			return err
		}
		if k.internalizer.Inconsistent() {
			return nil
		}
	}
	if k.smtState.SimplifyClauses && k.assumptions == nil {
		if err := k.internalizer.simplifyClausesAtBase(baseLevel); err != nil {
			return err
		}
	}
	if k.smtState.LemmaGcStrategy == 2 {
		if err := k.internalizer.garbageCollectLearnedClauses(); err != nil {
			return err
		}
		k.internalizer.conflictsSinceLemmaGC = 0
	}
	return nil
}
