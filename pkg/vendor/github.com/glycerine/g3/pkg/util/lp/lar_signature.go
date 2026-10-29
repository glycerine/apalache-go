package lp

// Source: src/util/lp/lar_solution_signature.h;
// lar_core_solver.h:356-405,653-662. Actual pair and double consumers.

import "sort"

// LARSolutionSignature records nonbasic bound positions. C++ uses an unordered
// map. Preparation uses ascending Go keys for reproducible RNG/error prefixes;
// no insertion-order contract is imposed on this native map.
type LARSolutionSignature map[uint32]NonBasicColumnValuePosition

func (c *LARCoreSolver) ExtractSignatureFromPairSolver(signature *LARSolutionSignature, s *PairPrimalCoreSolver) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if signature == nil || s == nil {
		return ErrBounds
	}
	if err = s.ready(); err != nil {
		return err
	}
	if *signature == nil {
		*signature = make(LARSolutionSignature)
	}
	clear(*signature)
	for j, h := range *s.Heading {
		if h < 0 {
			position, e := s.GetNonBasicColumnValuePosition(uint32(j))
			if e != nil {
				return e
			}
			(*signature)[uint32(j)] = position
		}
	}
	return nil
}

func (c *LARCoreSolver) ExtractSignatureFromFloatSolver(signature *LARSolutionSignature, s *FloatPrimalCoreSolver) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if signature == nil || s == nil {
		return ErrBounds
	}
	if *signature == nil {
		*signature = make(LARSolutionSignature)
	}
	clear(*signature)
	for j, h := range *s.Heading {
		if h < 0 {
			position, e := s.GetNonBasicColumnValuePosition(uint32(j))
			if e != nil {
				return e
			}
			(*signature)[uint32(j)] = position
		}
	}
	return nil
}

// signatureBoundSide is only the common control flow of the two retained
// concrete preparations: 0 leaves X unchanged, 1/2 copy lower/upper, 3 sets zero.
func (c *LARCoreSolver) signatureBoundSide(j uint32, position NonBasicColumnValuePosition) (uint8, error) {
	if sourceDebugEnabled {
		if uint64(j) >= uint64(len(c.RHeading)) {
			return 0, ErrBounds
		}
		if c.RHeading[j] >= 0 {
			return 0, ErrAssertion
		}
	}
	switch position {
	case PositionAtLowBound:
		return 1, nil
	case PositionAtFixed, PositionAtUpperBound:
		return 2, nil
	case PositionFreeOfBounds:
		return 3, nil
	case PositionNotAtBound:
		if uint64(j) >= uint64(len(c.Bounds.Types.values)) {
			return 0, ErrBounds
		}
		switch c.Bounds.Types.values[j] {
		case ColumnFree:
			if sourceDebugEnabled {
				return 0, ErrAssertion
			}
			// Source intentionally falls through to the upper-bound case.
			return 2, nil
		case ColumnUpperBound:
			return 2, nil
		case ColumnLowBound, ColumnFixed:
			return 1, nil
		case ColumnBoxed:
			if c.settings.RandomNext()%2 != 0 {
				return 1, nil
			}
			return 2, nil
		default:
			return 0, sourceAssertionFailure()
		}
	default:
		return 0, sourceAssertionFailure()
	}
}

func signatureKeys(signature LARSolutionSignature) []uint32 {
	keys := make([]uint32, 0, len(signature))
	for j := range signature {
		keys = append(keys, j)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func (c *LARCoreSolver) PreparePairSolverXWithSignature(signature LARSolutionSignature, s *PairPrimalCoreSolver) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if s == nil {
		return ErrBounds
	}
	if err = s.ready(); err != nil {
		return err
	}
	for _, j := range signatureKeys(signature) {
		side, e := c.signatureBoundSide(j, signature[j])
		if e != nil {
			return e
		}
		if side == 0 {
			continue
		}
		if uint64(j) >= uint64(len(*s.X)) {
			return ErrBounds
		}
		zero := NewRationalPair()
		value := &zero
		if side != 3 {
			bounds := s.LowBounds
			if side == 2 {
				bounds = s.UpperBounds
			}
			if uint64(j) >= uint64(len(*bounds)) {
				return ErrBounds
			}
			value = &(*bounds)[j]
		}
		before, e := s.calls.begin(s.manager)
		if e != nil {
			return e
		}
		e = (*s.X)[j].Copy(s.manager, value)
		if err = s.calls.after(s.manager, before, e); err != nil {
			return err
		}
	}
	if sourceDebugEnabled {
		for i := range *s.B {
			zero, e := (*s.B)[i].IsZero(s.manager)
			if e != nil {
				return e
			}
			if !zero {
				return ErrAssertion
			}
		}
	}
	return s.SolveAxEqualsB()
}

func (c *LARCoreSolver) PrepareFloatSolverXWithSignature(signature LARSolutionSignature, s *FloatPrimalCoreSolver) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if s == nil {
		return ErrBounds
	}
	for _, j := range signatureKeys(signature) {
		side, e := c.signatureBoundSide(j, signature[j])
		if e != nil {
			return e
		}
		if side == 0 {
			continue
		}
		if uint64(j) >= uint64(len(*s.X)) {
			return ErrBounds
		}
		if side == 3 {
			(*s.X)[j] = 0
			continue
		}
		bounds := s.LowBounds
		if side == 2 {
			bounds = s.UpperBounds
		}
		if uint64(j) >= uint64(len(*bounds)) {
			return ErrBounds
		}
		(*s.X)[j] = (*bounds)[j]
	}
	if sourceDebugEnabled {
		for _, b := range *s.B {
			if b != 0 {
				return ErrAssertion
			}
		}
	}
	return s.SolveAxEqualsB()
}
