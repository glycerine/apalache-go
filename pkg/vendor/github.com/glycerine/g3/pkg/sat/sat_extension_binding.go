package sat

func (s *Solver) SetExtension(extension SATExtension) error {
	if err := s.open(); err != nil {
		return err
	}
	s.propagation.extension = extension
	s.propagation.extensionPresent = extension != nil
	return nil
}

func (s *satSolverPropagationState) sourceExtension() (SATExtension, error) {
	if !s.extensionPresent {
		return nil, nil
	}
	if s.extension == nil {
		return nil, ErrSATMissingSourceExtBinding
	}
	return s.extension, nil
}
