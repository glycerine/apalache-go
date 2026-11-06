package sat

// Source: src/sat/sat_solver.cpp:805-896. The Go port remains deliberately
// single-goroutine. Running lanes in source index order selects one legal
// first-finisher ordering while preserving the source solver-copy, shared-unit,
// child-limit, result-transfer, and teardown boundaries.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type satParallelPortfolio struct {
	owner          *Solver
	exchange       *ParallelUnitExchange
	workers        []*Solver
	limits         []*control.Reslimit
	registered     uint32
	ownerBound     bool
	finished       int
	result         lbool.LBool
	firstLaneError error
}

func (s *Solver) checkParallelSpan(count uint32, span satSolverLiteralSpan, lanes uint32) (result lbool.LBool, err error) {
	p, err := newSATParallelPortfolio(s, lanes)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { err = errors.Join(err, p.close()) }()
	if err = p.run(count, span); err != nil {
		return lbool.Undef, err
	}
	if err = p.adopt(); err != nil {
		return lbool.Undef, err
	}
	return p.result, nil
}

func (p *satParallelPortfolio) run(count uint32, span satSolverLiteralSpan) error {
	p.finished = -1
	p.result = lbool.Undef
	lanes := len(p.workers) + 1
	for i := 0; i < lanes; i++ {
		s := p.owner
		if i < len(p.workers) {
			s = p.workers[i]
		}
		r, err := s.checkSpan(count, span)
		if err != nil {
			if i == 0 {
				p.firstLaneError = err
			}
			continue
		}
		if p.finished >= 0 {
			continue
		}
		p.finished, p.result = i, r
		for j, limit := range p.limits {
			if j != i {
				limit.Cancel()
			}
		}
	}
	if p.finished < 0 {
		if p.firstLaneError != nil {
			return p.firstLaneError
		}
		return ErrSATOwner
	}
	return nil
}
