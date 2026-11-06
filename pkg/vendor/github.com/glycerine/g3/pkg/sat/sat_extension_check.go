package sat

import "github.com/glycerine/g3/pkg/util/lbool"

func (s *Solver) finalExtensionCheck() (lbool.LBool, error) {
	if !s.propagation.extensionPresent {
		if err := s.mkModel(); err != nil {
			return lbool.Undef, err
		}
		return lbool.True, nil
	}
	extension, err := s.propagation.sourceExtension()
	if err != nil {
		return lbool.Undef, err
	}
	result, err := extension.Check()
	if err != nil {
		return lbool.Undef, err
	}
	switch result {
	case SATExtensionDone:
		if err = s.mkModel(); err != nil {
			return lbool.Undef, err
		}
		return lbool.True, nil
	case SATExtensionContinue:
		return lbool.Undef, nil
	case SATExtensionGiveUp:
		return lbool.Undef, satSolverAbort{}
	default:
		return lbool.Undef, ErrSATAssertion
	}
}
