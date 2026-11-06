package arith

// Source: src/tactic/arith/fm_tactic.cpp:1088-1133.
type FMRegistration uint8

const (
	FMRegistrationIndexed FMRegistration = iota
	FMRegistrationDeferred
	FMRegistrationFalse
)

func FMConstraintIsFalse(c *FMConstraint) bool {
	return c != nil && len(c.Literals) == 0 && len(c.Variables) == 0 && (c.Constant.Sign() < 0 || c.Strict && c.Constant.Sign() == 0)
}

// Register normalizes a constraint and places it in each permitted variable's
// occurrence list. Deferred constraints are the source's "all forbidden"
// branch and remain owned by the caller for reconstruction in the output goal.
func (t *FMVariableTable) Register(c *FMConstraint) FMRegistration {
	c.NormalizeCoefficients()
	if FMConstraintIsFalse(c) {
		c.Dead = true
		return FMRegistrationFalse
	}
	indexed := false
	for _, x := range c.Variables {
		if !t.IsForbidden(x) {
			indexed = true
			break
		}
	}
	if !indexed {
		return FMRegistrationDeferred
	}
	t.Index.Add(c)
	return FMRegistrationIndexed
}
