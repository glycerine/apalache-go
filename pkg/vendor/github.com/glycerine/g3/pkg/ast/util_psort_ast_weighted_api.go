package ast

// Source: src/util/sorting_network.h:368-459 concrete ast_ext2 specialization.

func (s *PsortASTNetwork) WeightedGe(k uint32, weights []uint32, literals ASTLiteralVectorSpan) (ASTHandle, error) {
	if e := s.open(); e != nil {
		return ASTHandle{}, e
	}
	if len(weights) == 0 {
		if k == 0 {
			return s.context.True()
		}
		return s.context.False()
	}
	s.running = true
	defer func() { s.running = false }()
	s.mode, s.modeInitialized = psortGEFull, true
	return s.weightedGE(uint64(k), uint32(len(weights)), weights, psortASTPointer{span: literals})
}

func (s *PsortASTNetwork) WeightedLe(k uint32, weights []uint32, literals ASTLiteralVectorSpan) (ASTHandle, error) {
	if e := s.open(); e != nil {
		return ASTHandle{}, e
	}
	if len(weights) == 0 {
		return s.context.True()
	}
	s.running = true
	defer func() { s.running = false }()
	s.mode, s.modeInitialized = psortGEFull, true
	return s.weightedLE(k, uint32(len(weights)), weights, psortASTPointer{span: literals})
}

func (s *PsortASTNetwork) WeightedEq(k uint32, weights []uint32, literals ASTLiteralVectorSpan) (ASTHandle, error) {
	if e := s.open(); e != nil {
		return ASTHandle{}, e
	}
	if len(weights) == 0 {
		return s.context.manager.MkBoolVal(k == 0)
	}
	s.running = true
	defer func() { s.running = false }()
	s.mode, s.modeInitialized = psortGEFull, true
	p := psortASTPointer{span: literals}
	ge, e := s.weightedGE(uint64(k), uint32(len(weights)), weights, p)
	if e != nil {
		return ASTHandle{}, e
	}
	le, e := s.weightedLE(k, uint32(len(weights)), weights, p)
	if e != nil {
		return ASTHandle{}, e
	}
	return s.makeAnd2(ge, le)
}
