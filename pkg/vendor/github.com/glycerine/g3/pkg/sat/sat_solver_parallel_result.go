package sat

// Source: src/sat/sat_solver.cpp:844-856,882-884.

import "github.com/glycerine/g3/pkg/util/lbool"

func (p *satParallelPortfolio) adopt() error {
	if p.finished < 0 || p.finished >= len(p.workers) {
		return nil
	}
	winner := p.workers[p.finished]
	switch p.result {
	case lbool.True:
		if err := p.owner.setModel(&winner.model); err != nil {
			return err
		}
	case lbool.False:
		if err := p.owner.conflict.core.Shrink(0); err != nil {
			return err
		}
		if err := satMUSAppend(&p.owner.conflict.core, &winner.conflict.core); err != nil {
			return err
		}
	}
	p.owner.copyParallelMainStatistics(winner)
	return nil
}

func (s *Solver) copyParallelMainStatistics(src *Solver) {
	s.Variables.mkVarCount = src.Variables.mkVarCount
	s.Lifecycle.mkBinary = src.Lifecycle.mkBinary
	s.Lifecycle.mkTernary = src.Lifecycle.mkTernary
	s.Lifecycle.mkNary = src.Lifecycle.mkNary
	s.gc.gcClauseStat = src.gc.gcClauseStat
	s.Lifecycle.deleted = src.Lifecycle.deleted
	s.conflict.conflictStat = src.conflict.conflictStat
	s.propagation.clausePropagations = src.propagation.clausePropagations
	s.decision.decisions = src.decision.decisions
	s.propagation.binaryPropagations = src.propagation.binaryPropagations
	s.propagation.ternaryPropagations = src.propagation.ternaryPropagations
	s.gc.restartStat = src.gc.restartStat
	s.analysis.minimizedLits = src.analysis.minimizedLits
	s.dynamic.eliminated = src.dynamic.eliminated
	s.blockedCorrectionSets = src.blockedCorrectionSets
}
