package ast

func (s *PsortASTNetwork) exactOneBimander(full bool, n uint32, p psortASTPointer) (result ASTHandle, err error) {
	f, e := s.beginFrame(1)
	if e != nil {
		return result, e
	}
	defer func() { err = s.finishFrame(f, err) }()
	if e = s.defaultVectors(f); e != nil {
		return result, e
	}
	ors := &f.vectors[0]
	result, e = s.atMostOneBimander(full, n, p, ors)
	if e != nil {
		return result, e
	}
	if full {
		disjunction, e := s.makeOrVector(ors)
		if e != nil {
			return result, e
		}
		return s.makeAnd2(result, disjunction)
	}
	size, e := ors.Size()
	if e != nil {
		return result, e
	}
	pointer, e := psortVectorPointer(ors)
	if e != nil {
		return result, e
	}
	return result, s.impliesOr(result, size, pointer)
}
