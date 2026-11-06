package opt

// Source: src/opt/maxsmt.cpp commit_assignment consistency guard.

func (s *ArithmeticSolver) validateSoftCommit(index int, value ObjectiveValue) error {
	weight, err := softAssignmentWeight(&s.objectives[index], value.SoftAssignment)
	if err != nil {
		return err
	}
	if weight.Cmp(&value.SoftSatisfied) != 0 {
		return errMaxSMTAssignment
	}
	return nil
}
