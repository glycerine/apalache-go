package tactic

// Source: src/tactic/tactical.cpp:462-884; src/tactic/tactical.h:55-61.

// NewParTactic selects the source _NO_OMP_ implementation for this
// single-goroutine port. It has the same ordered fallback behavior as or_else.
func NewParTactic(children ...Tactic) (Tactic, error) {
	return NewOrElseTactic(children...)
}

// NewParAndThenTactic selects the source _NO_OMP_ implementation for this
// single-goroutine port. It has the same subgoal composition as and_then.
func NewParAndThenTactic(children ...Tactic) (Tactic, error) {
	return NewAndThenTactic(children...)
}
