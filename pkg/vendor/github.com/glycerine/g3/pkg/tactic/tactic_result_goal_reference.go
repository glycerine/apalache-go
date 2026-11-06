package tactic

import "github.com/glycerine/g3/pkg/arena"

// AcquireGoalReference bridges the native direct owners used by TacticResult
// to a counted public goal handle. A TakeGoal entry transfers its direct
// cleanup obligation to that reference; AddGoal entries remain borrowed.
func (r *TacticResult) AcquireGoalReference(index uint32) (*Goal, error) {
	if r == nil || r.closed || uint64(index) >= uint64(len(r.Goals)) || uint64(index) >= uint64(len(r.goalOwned)) {
		return nil, arena.ErrBounds
	}
	goal := r.Goals[index]
	if err := goal.IncRef(); err != nil {
		return nil, err
	}
	r.goalOwned[index] = false
	return goal, nil
}
