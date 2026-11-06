package sat

// Source: src/sat/sat_solver.cpp:805-835.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/control"
)

func newSATParallelPortfolio(s *Solver, lanes uint32) (_ *satParallelPortfolio, err error) {
	if err = s.open(); err != nil {
		return nil, err
	}
	if lanes < 2 {
		return nil, ErrSATOwner
	}
	p := &satParallelPortfolio{owner: s, exchange: NewParallelUnitExchange(), finished: -1}
	defer func() {
		if err != nil {
			err = errors.Join(err, p.close())
		}
	}()
	phaseName := "phase"
	savedPhase, err := s.context.GetSymbolName(&s.params, &phaseName, s.config.cached[2])
	if err != nil {
		return nil, err
	}
	defer func() {
		e := s.context.SetSymbolName(&s.params, &phaseName, savedPhase)
		if e != nil {
			err = errors.Join(err, e)
		}
	}()
	randomSeedName := "random_seed"
	extra := lanes - 1
	for i := uint32(0); i < extra; i++ {
		if err = s.context.SetUIntName(&s.params, &randomSeedName, s.decision.rng.Next()); err != nil {
			return nil, err
		}
		if i == 1+lanes/2 {
			if err = s.context.SetSymbolName(&s.params, &phaseName, s.config.cached[3]); err != nil {
				return nil, err
			}
		}
		limit, e := control.NewReslimit(s.budget)
		if e != nil {
			return nil, e
		}
		p.limits = append(p.limits, limit)
		worker, e := NewSATSolver(SolverConfig{
			TraceTags: s.traceTags, ParamsContext: s.context, Params: s.params,
			RLimit: limit, Budget: s.budget, Runtime: s.runtime,
		})
		if e != nil {
			return nil, e
		}
		p.workers = append(p.workers, worker)
		if e = worker.copyFrom(s); e != nil {
			return nil, e
		}
		if e = worker.SetParallelExchange(p.exchange); e != nil {
			return nil, e
		}
		if e = s.Limit.PushChild(limit); e != nil {
			return nil, e
		}
		p.registered++
	}
	if err = s.SetParallelExchange(p.exchange); err != nil {
		return nil, err
	}
	p.ownerBound = true
	return p, nil
}
