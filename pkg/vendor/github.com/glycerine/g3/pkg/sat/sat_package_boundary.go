package sat

// Source: src/sat/sat_solver.h public solver inspection; src/sat/sat_types.h.

import (
	"github.com/glycerine/g3/pkg/arena"

	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func (s *Solver) Budget() *arena.Budget          { return s.budget }
func (s *Solver) RuntimeOwner() *umemory.Runtime { return s.runtime.memory }
func (s *Solver) CheckOwner() error              { return s.open() }
func (s *Solver) PopToBaseLevel() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.popToBase()
}
func (s *Solver) SourceInconsistent() (bool, error)             { return s.sourceInconsistent() }
func (s *Solver) SourceNumVars() (uint32, error)                { return s.sourceNumVars() }
func (s *Solver) SourceModelIsCurrent() (bool, error)           { return s.sourceModelIsCurrent() }
func (s *Solver) SourceValueVar(v BoolVar) (lbool.LBool, error) { return s.sourceValueVar(v) }

// CollectBinClauses appends in watch traversal order. On error it returns the
// completed prefix, including the caller's original contents.
func (s *Solver) CollectBinClauses(out []BinaryLiteralPair, learned bool) ([]BinaryLiteralPair, error) {
	return s.collectBinClauses(out, learned)
}
func (s *Solver) ClauseAllocator() *ClauseAllocator { return s.Clauses }
func (s *Solver) ClauseReferences(learned bool) []StoredClauseRef {
	if learned {
		return s.Lifecycle.learned
	}
	return s.Lifecycle.Original
}
func (l Literal) Variable() BoolVar                 { return l.variable() }
func (l Literal) Sign() bool                        { return l.sign() }
func (l Literal) Index() uint32                     { return l.index() }
func (m Model) Read(v BoolVar) (lbool.LBool, error) { return (satSolverModelSpan{slice: m}).read(v) }

func (s *Solver) BoolParameter(name *string, fallback bool) (bool, error) {
	return s.context.GetBoolName(&s.params, name, fallback)
}
func (s *Solver) UIntParameter(name *string, fallback uint32) (uint32, error) {
	return s.context.GetUIntName(&s.params, name, fallback)
}
func (p BinaryLiteralPair) FirstIndex() uint32  { return p.first }
func (p BinaryLiteralPair) SecondIndex() uint32 { return p.second }
