package sat

func (s *satSolverPropagationState) notifyExtensionClausesModified() error {
	if !s.extensionPresent {
		return nil
	}
	extension, err := s.sourceExtension()
	if err != nil {
		return err
	}
	return extension.ClausesModified()
}

func (s *satSolverPropagationState) simplifyExtension() error {
	if !s.extensionPresent {
		return nil
	}
	extension, err := s.sourceExtension()
	if err != nil {
		return err
	}
	if err = extension.ClausesModified(); err != nil {
		return err
	}
	return extension.Simplify()
}
