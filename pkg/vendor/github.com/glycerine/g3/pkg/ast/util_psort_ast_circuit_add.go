package ast

// Source: src/util/sorting_network.h mk_unit_circuit, mk_add_circuit,
// circuit_add.

func (s *PsortASTNetwork) circuitAddVectors(left, right []ASTHandle) ([]ASTHandle, ASTHandle, error) {
	carry, e := s.context.False()
	if e != nil {
		return nil, ASTHandle{}, e
	}
	output := make([]ASTHandle, 0, len(left))
	for i := range left {
		nc, e := s.not(carry)
		if e != nil {
			return nil, ASTHandle{}, e
		}
		nl, e := s.not(left[i])
		if e != nil {
			return nil, ASTHandle{}, e
		}
		nr, e := s.not(right[i])
		if e != nil {
			return nil, ASTHandle{}, e
		}
		a, e := s.logicAnd(carry, nl, nr)
		if e != nil {
			return nil, ASTHandle{}, e
		}
		b, e := s.logicAnd(left[i], nc, nr)
		if e != nil {
			return nil, ASTHandle{}, e
		}
		c, e := s.logicAnd(right[i], nc, nl)
		if e != nil {
			return nil, ASTHandle{}, e
		}
		d, e := s.logicAnd(carry, left[i], right[i])
		if e != nil {
			return nil, ASTHandle{}, e
		}
		bit, e := s.logicOr(a, b, c, d)
		if e != nil {
			return nil, ASTHandle{}, e
		}
		output = append(output, bit)
		a, e = s.logicAnd(carry, left[i])
		if e != nil {
			return nil, ASTHandle{}, e
		}
		b, e = s.logicAnd(carry, right[i])
		if e != nil {
			return nil, ASTHandle{}, e
		}
		c, e = s.logicAnd(left[i], right[i])
		if e != nil {
			return nil, ASTHandle{}, e
		}
		carry, e = s.logicOr(a, b, c)
		if e != nil {
			return nil, ASTHandle{}, e
		}
	}
	return output, carry, nil
}

func (s *PsortASTNetwork) circuitAdd(bits, n uint32, p psortASTPointer) ([]ASTHandle, ASTHandle, error) {
	if bits == 0 {
		overflow, e := s.context.False()
		return nil, overflow, e
	}
	if n == 0 {
		out := make([]ASTHandle, bits)
		for i := range out {
			h, e := s.context.False()
			if e != nil {
				return nil, ASTHandle{}, e
			}
			out[i] = h
		}
		overflow, e := s.context.False()
		return out, overflow, e
	}
	if n == 1 {
		x, e := p.at(0)
		if e != nil {
			return nil, ASTHandle{}, e
		}
		out := make([]ASTHandle, bits)
		out[0] = x
		for i := uint32(1); i < bits; i++ {
			out[i], e = s.context.False()
			if e != nil {
				return nil, ASTHandle{}, e
			}
		}
		overflow, e := s.context.False()
		return out, overflow, e
	}
	half := n / 2
	left, leftOverflow, e := s.circuitAdd(bits, half, p)
	if e != nil {
		return nil, ASTHandle{}, e
	}
	right, rightOverflow, e := s.circuitAdd(bits, n-half, p.add(half))
	if e != nil {
		return nil, ASTHandle{}, e
	}
	out, addOverflow, e := s.circuitAddVectors(left, right)
	if e != nil {
		return nil, ASTHandle{}, e
	}
	overflow, e := s.logicOr(leftOverflow, rightOverflow, addOverflow)
	return out, overflow, e
}
