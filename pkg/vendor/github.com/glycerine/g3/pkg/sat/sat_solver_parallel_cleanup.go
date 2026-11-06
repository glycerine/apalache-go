package sat

// Source: src/sat/sat_solver.cpp:873-888.

import "errors"

func (p *satParallelPortfolio) close() (err error) {
	if p == nil {
		return nil
	}
	if p.ownerBound {
		err = errors.Join(err, p.owner.SetParallelExchange(nil))
		p.ownerBound = false
	}
	for i := len(p.workers); i > 0; i-- {
		w := p.workers[i-1]
		if w != nil && w.conflict.parallelPresent {
			err = errors.Join(err, w.SetParallelExchange(nil))
		}
	}
	for p.registered > 0 {
		p.owner.Limit.PopChild()
		p.registered--
	}
	for i := len(p.workers); i > 0; i-- {
		if p.workers[i-1] != nil {
			err = errors.Join(err, p.workers[i-1].Close())
		}
	}
	p.workers = nil
	for i := len(p.limits); i > 0; i-- {
		if p.limits[i-1] != nil {
			p.limits[i-1].Close()
		}
	}
	p.limits = nil
	if p.exchange != nil {
		err = errors.Join(err, p.exchange.Close())
		p.exchange = nil
	}
	return err
}
