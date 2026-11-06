package smtcore

// Source: src/util/pool.h, the concrete pool<enode_vector> used by
// src/smt/mam.cpp. Vectors borrow enodes and are cleared before reuse.

type smtMAMEnodePool struct{ free [][]*SMTEnode }

func (p *smtMAMEnodePool) take(size int) []*SMTEnode {
	if len(p.free) == 0 {
		return make([]*SMTEnode, size)
	}
	i := len(p.free) - 1
	vector := p.free[i]
	p.free[i] = nil
	p.free = p.free[:i]
	clear(vector)
	if cap(vector) < size {
		return make([]*SMTEnode, size)
	}
	return vector[:size]
}

func (p *smtMAMEnodePool) recycle(vector []*SMTEnode) {
	p.free = append(p.free, vector)
}
