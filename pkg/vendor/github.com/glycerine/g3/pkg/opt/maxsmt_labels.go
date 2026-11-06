package opt

// Source: src/opt/maxsmt.cpp set_model/get_model label retention.

func (s *ArithmeticSolver) captureResultLabels(result *ArithmeticResult) error {
	labels, err := s.backend.Labels()
	if err != nil {
		return err
	}
	result.Labels = cloneLabels(labels)
	if !result.labelsCaptured {
		result.InitialLabels = cloneLabels(labels)
		result.labelsCaptured = true
	}
	return nil
}
