package sat

// Source: src/sat/sat_solver.h:245-286,318-323.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// These copyable nonowning values borrow the actual named solver member, not
// its payload. Each access resolves current backing, including after growth.
// The caller must keep the solver open; no allocation or retain is introduced.
type satSolverModelView struct{ owner *Solver }
type satSolverCoreView struct{ owner *Solver }

func (s *Solver) borrowModel() (satSolverModelView, error) {
	if e := s.open(); e != nil {
		return satSolverModelView{}, e
	}
	return satSolverModelView{s}, nil
}
func (s *Solver) BorrowCore() (satSolverCoreView, error) {
	if e := s.open(); e != nil {
		return satSolverCoreView{}, e
	}
	return satSolverCoreView{s}, nil
}
func (v satSolverModelView) Size() (uint32, error) {
	if e := v.owner.open(); e != nil {
		return 0, e
	}
	return v.owner.model.Size()
}
func (v satSolverModelView) Read(i BoolVar) (lbool.LBool, error) {
	if e := v.owner.open(); e != nil {
		return lbool.Undef, e
	}
	x, e := v.owner.model.Read(uint32(i))
	return lbool.LBool(int32(x)), e
}
func (v satSolverCoreView) Size() (uint32, error) {
	if e := v.owner.open(); e != nil {
		return 0, e
	}
	return v.owner.conflict.core.Size()
}
func (v satSolverCoreView) Read(i uint32) (Literal, error) {
	if e := v.owner.open(); e != nil {
		return 0, e
	}
	x, e := v.owner.conflict.core.Read(i)
	return Literal(x), e
}
func (s *Solver) sourceInconsistent() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	return s.propagation.inconsistent, nil
}
func (s *Solver) sourceNumVars() (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.Variables.numVarsChecked()
}
func (s *Solver) sourceNumRestarts() (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.gc.restarts, nil
}
func (s *Solver) SourceIsExternal(v BoolVar) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	return s.Variables.IsExternal(v)
}
func (s *Solver) sourceSetExternal(v BoolVar) error {
	if e := s.open(); e != nil {
		return e
	}
	return s.Variables.setExternal(v, true)
}
func (s *Solver) sourceWasEliminated(v BoolVar) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	return s.Variables.wasEliminated(v)
}
func (s *Solver) sourceScopeLevel() (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.Variables.scopeLevel, nil
}
func (s *Solver) sourceValueLiteral(l Literal) (lbool.LBool, error) {
	if e := s.open(); e != nil {
		return lbool.Undef, e
	}
	return s.Variables.valueLiteral(l)
}
func (s *Solver) sourceValueVar(v BoolVar) (lbool.LBool, error) {
	if e := s.open(); e != nil {
		return lbool.Undef, e
	}
	return s.Variables.valueVar(v)
}
func (s *Solver) sourceLevelVar(v BoolVar) (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.Variables.levelVar(v)
}
func (s *Solver) sourceLevelLiteral(l Literal) (uint32, error) {
	return s.sourceLevelVar(l.variable())
}
func (s *Solver) sourceInitTrailSize() (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	if s.Variables.scopeLevel == 0 {
		return s.Variables.trail.Size(s.Variables.trailRef)
	}
	if len(s.Lifecycle.scopes) == 0 {
		return 0, arena.ErrBounds
	}
	return s.Lifecycle.scopes[0].trailLimit, nil
}
func (s *Solver) sourceModelIsCurrent() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	return s.Variables.modelIsCurrent, nil
}
func (s *Solver) sourceCanceled() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	return !s.Limit.Inc(), nil
}

// These private concrete pointer borrows preserve member identity. Go has no
// const reference type; mutation through these pointers is outside this facade.
func (s *Solver) borrowConfig() (*satMainConfig, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	return s.config, nil
}
func (s *Solver) borrowModelConverter() (*satModelConverter, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	return s.converter, nil
}
