package smtcore

// Source: src/smt/smt_context.h inc_bvar_activity(),
// decay_bvar_activity(), and src/smt/smt_context.cpp rescale_bool_var_activity().

import (
	"github.com/glycerine/g3/pkg/ast"
)

const (
	smtBooleanActivityLimit        = 1e100
	smtBooleanInverseActivityLimit = 1e-100
)

func (s *SMTInternalizerState) incrementBooleanActivity(variable int32) error {
	if variable < SMTFirstBoolVar || int(variable) >= len(s.activity) {
		return ast.ErrASTDomain
	}
	s.activity[variable] += s.activityIncrement
	if s.activity[variable] > smtBooleanActivityLimit {
		for index := range s.activity {
			s.activity[index] *= smtBooleanInverseActivityLimit
		}
		s.activityIncrement *= smtBooleanInverseActivityLimit
	}
	if s.basicActivity != nil && s.basicActivity.Contains(variable) {
		if err := s.basicActivity.Decreased(variable); err != nil {
			return err
		}
	}
	// dact calls the base update, then repeats the primary Decreased call.
	if s.delayedActivity != nil {
		if s.basicActivity.Contains(variable) {
			if err := s.basicActivity.Decreased(variable); err != nil {
				return err
			}
		}
		if s.delayedActivity.Contains(variable) {
			if err := s.delayedActivity.Decreased(variable); err != nil {
				return err
			}
		}
	}
	if s.theoryAwareBranching != nil {
		return s.theoryAwareBranching.activityIncreased(variable)
	}
	return nil
}

func (s *SMTInternalizerState) decayBooleanActivity() {
	s.activityIncrement *= s.activityInverseDecay
}

func (s *SMTInternalizerState) configureBooleanHeuristics(params SMTParamsState) error {
	return s.configureBooleanHeuristicsForQueue(params, effectiveSMTCaseSplitStrategy(params), true)
}

func (s *SMTInternalizerState) configureBooleanHeuristicsForQueue(params SMTParamsState, strategy uint32, reseed bool) error {
	if params.RandomInitialActivity > 2 || strategy > 6 || params.InvDecay <= 0 {
		return ast.ErrASTDomain
	}
	if reseed {
		s.random.SetSeed(params.RandomSeed)
	}
	s.randomInitialActivity = params.RandomInitialActivity
	s.caseSplitStrategy = strategy
	s.theoryCaseSplit = params.TheoryCaseSplit
	s.activityInverseDecay = params.InvDecay
	switch strategy {
	case 0:
		return s.ensureBasicActivityQueue()
	case 1:
		return s.ensureDelayedActivityQueue()
	case 2:
		return s.ensureCachedActivityQueue()
	case 4:
		return s.ensureRelevanceActivityQueue()
	case 5:
		return s.ensureRelevanceGoalQueue()
	case 6:
		return s.ensureTheoryAwareBranching()
	}
	return nil
}

// Source cact_case_split_queue: map values overwrite, but every positive
// search-time deletion pushes another owning reference into cache_domain.
func (s *SMTInternalizerState) cacheBooleanActivity(expression ast.ASTHandle, activity float64) error {
	if s.activityCacheRetiring {
		return ast.ErrASTPending
	}
	if err := s.manager.IncRef(expression); err != nil {
		return err
	}
	s.activityCache[expression] = activity
	s.activityCacheDomain = append(s.activityCacheDomain, expression)
	return nil
}

func (s *SMTInternalizerState) releaseActivityCacheDomain() error {
	s.activityCacheRetiring = true
	for s.activityCacheDrop < len(s.activityCacheDomain) {
		consumed, err := s.manager.DecRefConsumed(s.activityCacheDomain[s.activityCacheDrop])
		if consumed {
			s.activityCacheDomain[s.activityCacheDrop] = ast.ASTHandle{}
			s.activityCacheDrop++
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	s.activityCacheDomain = s.activityCacheDomain[:0]
	s.activityCacheDrop = 0
	s.activityCacheRetiring = false
	return nil
}

func (s *SMTInternalizerState) releaseBooleanActivityCache() error {
	// Native clears the nonowning map before retiring domain references.
	s.activityCache = nil
	if err := s.releaseActivityCacheDomain(); err != nil {
		return err
	}
	s.activityCacheDomain = nil
	return nil
}

// Source init_search_eh resets both map and ordered reference domain. The
// cache connects deletions/reinternalizations within a search, not searches.
func (s *SMTInternalizerState) resetBooleanActivityCache() error {
	s.activityCache = make(map[ast.ASTHandle]float64)
	return s.releaseActivityCacheDomain()
}
