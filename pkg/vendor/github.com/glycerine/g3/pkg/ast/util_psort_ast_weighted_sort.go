package ast

// SortingWeightedGE selects the full comparator direction required by the
// retained PB2BV mixed-radix consumer before sorting contribution digits.
func (s *PsortASTNetwork) SortingWeightedGE(n uint32, p ASTLiteralVectorSpan, out *ASTLiteralVector) error {
	if e := s.open(); e != nil {
		return e
	}
	if out == nil || out.self != out {
		return ErrASTHandle
	}
	s.running = true
	defer func() { s.running = false }()
	s.mode, s.modeInitialized = psortGEFull, true
	return s.sorting(n, psortASTPointer{span: p}, out)
}
