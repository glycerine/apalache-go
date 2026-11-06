package ast

// Source: src/util/sorting_network.h weighted le/ge/eq and cmp.

func (s *PsortASTNetwork) weightedGE(k uint64, n uint32, weights []uint32, p psortASTPointer) (result ASTHandle, err error) {
	var sum uint64
	var maximum uint32
	for i := uint32(0); i < n; i++ {
		sum += uint64(weights[i])
		if weights[i] > maximum {
			maximum = weights[i]
		}
	}
	if sum < k {
		return s.context.False()
	}
	if k == 0 {
		return s.context.True()
	}
	if maximum == 0 {
		return s.context.False()
	}
	bits := uint32(0)
	for v := maximum; v != 0; v >>= 1 {
		bits++
	}
	power := uint32(1) << (bits - 1)
	a := uint32((k + uint64(power) - 1) / uint64(power))
	rounded := uint64(a) * uint64(power)

	f, e := s.beginFrame(5)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return result, e
	}
	xs, digits, even, sorted, work := &f.vectors[0], &f.vectors[1], &f.vectors[2], &f.vectors[3], &f.vectors[4]
	ws := append([]uint32(nil), weights[:n]...)
	if e = s.appendPointer(xs, n, p); e != nil {
		return result, e
	}
	if rounded > k {
		ws = append(ws, uint32(rounded-uint64(k)))
		truth, x := s.context.True()
		if x != nil {
			return result, x
		}
		if e = xs.Push(truth); e != nil {
			return result, e
		}
		n++
	}
	for bit := uint32(0); bit < bits; bit++ {
		if e = digits.Reset(); e != nil {
			return result, e
		}
		for j := uint32(0); j < n; j++ {
			if ws[j]&(uint32(1)<<bit) != 0 {
				h, x := xs.At(j)
				if x != nil {
					return result, x
				}
				if e = digits.Push(h); e != nil {
					return result, e
				}
			}
		}
		if e = even.Reset(); e != nil {
			return result, e
		}
		wn, x := work.Size()
		if x != nil {
			return result, x
		}
		for j := uint32(1); j < wn; j += 2 {
			h, x := work.At(j)
			if x != nil {
				return result, x
			}
			if e = even.Push(h); e != nil {
				return result, e
			}
		}
		if e = sorted.Reset(); e != nil {
			return result, e
		}
		dn, x := digits.Size()
		if x != nil {
			return result, x
		}
		dp, x := psortVectorPointer(digits)
		if x != nil {
			return result, x
		}
		if e = s.sorting(dn, dp, sorted); e != nil {
			return result, e
		}
		if e = work.Reset(); e != nil {
			return result, e
		}
		sn, x := sorted.Size()
		if x != nil {
			return result, x
		}
		sp, x := psortVectorPointer(sorted)
		if x != nil {
			return result, x
		}
		en, x := even.Size()
		if x != nil {
			return result, x
		}
		ep, x := psortVectorPointer(even)
		if x != nil {
			return result, x
		}
		if e = s.merge(sn, sp, en, ep, work); e != nil {
			return result, e
		}
	}
	return work.At(a - 1)
}

func (s *PsortASTNetwork) weightedLE(k, n uint32, weights []uint32, p psortASTPointer) (result ASTHandle, err error) {
	var sum uint64
	f, e := s.beginFrame(1)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return result, e
	}
	negated := &f.vectors[0]
	for i := uint32(0); i < n; i++ {
		sum += uint64(weights[i])
		h, x := p.at(i)
		if x != nil {
			return result, x
		}
		h, x = s.not(h)
		if x != nil {
			return result, x
		}
		if e = negated.Push(h); e != nil {
			return result, e
		}
	}
	if uint64(k) >= sum {
		return s.context.True()
	}
	np, e := psortVectorPointer(negated)
	if e != nil {
		return result, e
	}
	return s.weightedGE(sum-uint64(k), n, weights, np)
}
