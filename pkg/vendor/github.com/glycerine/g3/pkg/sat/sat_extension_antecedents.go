package sat

func (s *satSolverConflictAnalysis) extensionAntecedents(consequent Literal, j satJustification) ([]Literal, error) {
	if !s.propagation.extensionPresent {
		return nil, ErrSATMissingSourceExtBinding
	}
	extension, err := s.propagation.sourceExtension()
	if err != nil {
		return nil, err
	}
	var result []Literal
	if err = extension.GetAntecedents(consequent, SATExternalJustificationIndex(j.ExternalIndex()), &result); err != nil {
		return nil, err
	}
	return result, nil
}
