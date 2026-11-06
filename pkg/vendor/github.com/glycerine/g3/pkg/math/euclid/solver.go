package euclid

// Source: src/math/euclid/euclidean_solver.h:30-86;
// src/math/euclid/euclidean_solver.cpp:485-501,788-845.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// Solver is the concrete Euclidean equation solver. Numerals passed to it
// belong to Manager; a supplied manager remains owned by its caller.
type Solver struct {
	self    *Solver
	setup   *solverSetup
	manager *numeral.RationalManager
	debug   *diagnostic.DebugContext
	trace   *diagnostic.TraceTags
	owns    bool
	closed  bool
	stop    error
}

func NewSolver(m *numeral.RationalManager) (*Solver, error) {
	return NewSolverWithDebug(m, nil)
}

// NewSolverWithDebug borrows the source heap assertion owner through the
// Euclidean solved-variable queue and across Reset.
func NewSolverWithDebug(m *numeral.RationalManager, debug *diagnostic.DebugContext) (*Solver, error) {
	owned := m == nil
	if owned {
		var e error
		budget := arena.NewBudget(^uint64(0))
		m, e = numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{
			Cells: arena.Config{Budget: budget},
			Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
		}})
		if e != nil {
			return nil, e
		}
	} else if e := m.CheckOwnerReady(); e != nil {
		return nil, e
	}
	setup, e := newSolverSetupWithDebugTrace(m, debug, nil, 784)
	if e != nil {
		if owned {
			e = errors.Join(e, m.CloseChecked())
		}
		return nil, e
	}
	s := &Solver{setup: setup, manager: m, debug: debug, owns: owned}
	s.self = s
	return s, nil
}

func (s *Solver) open() error {
	if s == nil || s.self != s || s.manager == nil {
		return arena.ErrInvalidSlot
	}
	if s.closed || s.setup == nil {
		return arena.ErrClosed
	}
	if s.stop != nil {
		return s.stop
	}
	return s.setup.open()
}

func (s *Solver) Manager() (*numeral.RationalManager, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	return s.manager, nil
}

// BindTrace borrows the source solver's trace stream. Nil restores the
// process trace tags; Reset preserves an explicit binding.
func (s *Solver) BindTrace(tags *diagnostic.TraceTags) error {
	if e := s.open(); e != nil {
		return e
	}
	s.trace = tags
	s.setup.trace = tags
	for _, eq := range s.setup.equations {
		if eq != nil {
			eq.trace = tags
		}
	}
	for _, eq := range s.setup.solution {
		if eq != nil {
			eq.trace = tags
		}
	}
	if s.setup.pendingEq != nil {
		s.setup.pendingEq.trace = tags
	}
	return nil
}

func (s *Solver) MkVar() (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.setup.mkVar(false)
}

func (s *Solver) MkJustification() (uint32, error) {
	if e := s.open(); e != nil {
		return 0, e
	}
	return s.setup.mkJustification()
}

func (s *Solver) AssertEq(as []numeral.Int, xs []uint32, c *numeral.Int, justification uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	return s.setup.assertEquation(as, xs, c, justification)
}

func (s *Solver) Solve() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	return s.setup.solve()
}

func (s *Solver) Inconsistent() (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	return s.setup.inconsistent != nullEquationIndex, nil
}

func (s *Solver) IsParameter(x uint32) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if x >= uint32(len(s.setup.parameter)) {
		return false, arena.ErrBounds
	}
	return s.setup.parameter[x], nil
}

// GetJustification returns a copy of the explanation IDs for the first
// contradictory equation. The source returns a const borrowed vector.
func (s *Solver) GetJustification() ([]uint32, error) {
	if e := s.open(); e != nil {
		return nil, e
	}
	if e := euclidSourceAssertion(569, "inconsistent()", s.setup.inconsistent != nullEquationIndex); e != nil {
		return nil, e
	}
	i := s.setup.inconsistent
	if i == nullEquationIndex || i >= uint32(len(s.setup.equations)) || s.setup.equations[i] == nil {
		return nil, arena.ErrInvalidSlot
	}
	return append([]uint32(nil), s.setup.equations[i].js...), nil
}

func (s *Solver) Normalize(as []numeral.Int, xs []uint32, c, aPrime, cPrime *numeral.Int,
	js *[]uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	return s.setup.normalizeQuery(as, xs, c, aPrime, cPrime, js)
}

// Reset destroys the current solver state and constructs a fresh one over the
// same numeral manager, retaining the source's borrowed or owned manager mode.
func (s *Solver) Reset() error {
	if s == nil || s.self != s || s.manager == nil {
		return arena.ErrInvalidSlot
	}
	if s.closed {
		return arena.ErrClosed
	}
	if e := s.manager.CheckOwnerReady(); e != nil {
		return e
	}
	if s.setup != nil {
		s.setup.memoryDeallocLine = 799
		if e := s.setup.Close(); e != nil {
			return e
		}
		s.setup = nil
	}
	setup, e := newSolverSetupWithDebugTrace(s.manager, s.debug, s.trace, 800)
	if e != nil {
		s.stop = e
		return e
	}
	s.setup = setup
	setup.trace = s.trace
	s.stop = nil
	return nil
}

func (s *Solver) Close() error {
	if s == nil || s.self != s {
		return arena.ErrInvalidSlot
	}
	if s.closed {
		return nil
	}
	if s.setup != nil {
		if s.setup.memoryDeallocLine != 799 {
			s.setup.memoryDeallocLine = 788
		}
		if e := s.setup.Close(); e != nil {
			return e
		}
		s.setup = nil
	}
	if s.owns {
		if e := s.manager.CloseChecked(); e != nil {
			return e
		}
	}
	s.closed = true
	return nil
}

func writeSolverText(out io.Writer, value string) error {
	if out == nil {
		return arena.ErrConfig
	}
	n, e := io.WriteString(out, value)
	if e != nil {
		return e
	}
	if n != len(value) {
		return io.ErrShortWrite
	}
	return nil
}

func (s *Solver) displayEquation(out io.Writer, eq *equation) error {
	for i, j := range eq.js {
		if i != 0 {
			if e := writeSolverText(out, " "); e != nil {
				return e
			}
		}
		b, e := s.manager.String(&eq.bs[i])
		if e != nil {
			return e
		}
		if e = writeSolverText(out, b+"*j"+strconv.FormatUint(uint64(j), 10)); e != nil {
			return e
		}
	}
	if len(eq.js) != 0 {
		if e := writeSolverText(out, " "); e != nil {
			return e
		}
	}
	if e := writeSolverText(out, "|= "); e != nil {
		return e
	}
	for i, x := range eq.xs {
		a, e := s.manager.Integers().String(&eq.as[i])
		if e != nil {
			return e
		}
		if e = writeSolverText(out, a+"*x"+strconv.FormatUint(uint64(x), 10)+" + "); e != nil {
			return e
		}
	}
	c, e := s.manager.Integers().String(&eq.c)
	if e != nil {
		return e
	}
	return writeSolverText(out, c+" = 0")
}

func (s *Solver) displayEquations(out io.Writer, equations []*equation) error {
	for _, eq := range equations {
		if eq == nil {
			continue
		}
		if e := s.displayEquation(out, eq); e != nil {
			return e
		}
		if e := writeSolverText(out, "\n"); e != nil {
			return e
		}
	}
	return nil
}

// Display writes the source equation listing in current owner order.
func (s *Solver) Display(out io.Writer) error {
	if e := s.open(); e != nil {
		return e
	}
	if out == nil {
		return arena.ErrConfig
	}
	if s.setup.inconsistent != nullEquationIndex {
		if e := writeSolverText(out, "inconsistent: "); e != nil {
			return e
		}
		if e := s.displayEquation(out, s.setup.equations[s.setup.inconsistent]); e != nil {
			return e
		}
		if e := writeSolverText(out, "\n"); e != nil {
			return e
		}
	}
	if e := writeSolverText(out, "solution set:\n"); e != nil {
		return e
	}
	if e := s.displayEquations(out, s.setup.solution); e != nil {
		return e
	}
	if e := writeSolverText(out, "todo:\n"); e != nil {
		return e
	}
	return s.displayEquations(out, s.setup.equations)
}
