package sat

func (s *satSolverPropagationState) propagateExtension(literal Literal, watched satWatched, keep bool) (bool, error) {
	extension, err := s.sourceExtension()
	if err != nil {
		return keep, err
	}
	index, err := watched.ExternalIndex()
	if err != nil {
		return keep, err
	}
	return extension.Propagate(literal, SATExternalConstraintIndex(index), keep)
}
