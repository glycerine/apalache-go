package nlsat

import "github.com/glycerine/g3/pkg/sat"

type simpleGeneralCandidate struct {
	kind    uint8
	base    simpleSumCandidate
	general simpleGeneralForm
}

func (s *Solver) needsGeneralLiteral(literal sat.Literal) (bool, error) {
	raw, err := s.clauses.literalAtom(literal)
	if err != nil || raw == nil {
		return false, err
	}
	atom, ok := raw.(*InequalityAtom)
	if !ok {
		return false, nil
	}
	core := s.atoms.ineq.core
	mm, err := core.MonomialManager()
	if err != nil {
		return false, err
	}
	if atom.Size() > 1 {
		return true, nil
	}
	for i := uint32(0); i < atom.Size(); i++ {
		even, cause := atom.IsEven(i)
		if cause != nil {
			return false, cause
		}
		if even {
			return true, nil
		}
		p, cause := atom.Polynomial(i)
		if cause != nil {
			return false, cause
		}
		info, cause := core.Info(p)
		if cause != nil {
			return false, cause
		}
		for j := uint32(0); j < info.Size; j++ {
			term, cause := core.TermMonomial(p, j)
			if cause != nil {
				return false, cause
			}
			ti, cause := mm.Info(term)
			if cause != nil {
				return false, cause
			}
			if ti.Size > 1 {
				return true, nil
			}
		}
	}
	return false, nil
}
func (s *Solver) simpleGeneralCandidate(literal sat.Literal) (candidate simpleGeneralCandidate, supported bool, err error) {
	general, err := s.needsGeneralLiteral(literal)
	if err != nil {
		return candidate, false, err
	}
	if general {
		candidate.general, supported, err = s.simpleGeneralLiteral(literal)
		candidate.kind = 2
		return
	}
	candidate.base, supported, err = s.simpleCombinedCandidate(literal)
	if supported {
		candidate.kind = 1
		return
	}
	candidate.general, supported, err = s.simpleGeneralLiteral(literal)
	if supported {
		candidate.kind = 2
	}
	return
}
