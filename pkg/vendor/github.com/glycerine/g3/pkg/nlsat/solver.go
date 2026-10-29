package nlsat

// Source: src/nlsat/nlsat_solver.cpp:167-231,1258-1309;
// nlsat_solver.h:38-55,64-112,172-205,214.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type SolverConfig struct {
	Algebraic    *polynomial.AlgebraicManager
	Polynomial   *polynomial.MultivariatePolynomialManager
	Limit        *control.Reslimit
	MaxMemory    uint64
	Lazy         uint32
	SimplifyCore bool
	MinimizeCore bool
	Randomize    bool
	Factor       bool
	MaxConflicts uint32
}

// Solver composes the retained concrete NLSAT components. The numerical and
// polynomial managers are borrowed and must outlive Close.
type Solver struct {
	config      SolverConfig
	allocator   *umemory.SmallAllocator
	cache       *polynomial.PolynomialCache
	atoms       *atomFactory
	assumptions *assumptionManager
	clauses     *clauseStore
	assignment  *Assignment
	evaluator   *Evaluator
	search      *searchState
	explanation *explanationAssumptions
	projection  *projectionWorklist
	resolution  *searchResolution
	closed      bool
}

type integerCut struct {
	variable uint32
	floor    unumeral.Rational
}

func NewSolver(config SolverConfig) (s *Solver, err error) {
	s = &Solver{config: config}
	if config.Algebraic == nil || config.Polynomial == nil || config.Limit == nil || config.Limit.Closed() {
		return s, arena.ErrConfig
	}
	if config.MaxMemory == 0 {
		s.config.MaxMemory = ^uint64(0)
	}
	q, err := config.Algebraic.RationalManager()
	if err != nil {
		return s, err
	}
	s.allocator, err = umemory.NewSmallAllocator(umemory.SmallAllocatorConfig{Runtime: q.Integers().RuntimeOwner(), Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: diagnostic.DebugBuild}})
	if err != nil {
		return s, err
	}
	s.cache, err = polynomial.NewPolynomialCache(config.Polynomial)
	if err != nil {
		return s, err
	}
	s.atoms, err = newAtomFactory(config.Polynomial, s.cache, s.allocator)
	if err != nil {
		return s, err
	}
	s.assumptions, err = newAssumptionManager(s.allocator)
	if err != nil {
		return s, err
	}
	s.clauses, err = newClauseStore(s.atoms, s.assumptions)
	if err != nil {
		return s, err
	}
	if err = s.clauses.mkTrueBooleanVariable(); err != nil {
		return s, err
	}
	s.assignment, err = NewAssignment(config.Algebraic)
	if err != nil {
		return s, err
	}
	s.evaluator, err = NewEvaluator(s.assignment, config.Polynomial, s.allocator)
	if err != nil {
		return s, err
	}
	s.search, err = newSearchState(s.clauses, s.evaluator, searchConfig{Limit: config.Limit, MaxMemory: s.config.MaxMemory, Lazy: config.Lazy, SimplifyCores: config.SimplifyCore, Randomize: config.Randomize, MaxConflicts: config.MaxConflicts})
	if err != nil {
		return s, err
	}
	s.explanation, err = newExplanationAssumptions(s.search)
	if err != nil {
		return s, err
	}
	if config.MinimizeCore {
		if _, err = newExplanationMinimizer(s.search); err != nil {
			return s, err
		}
	}
	s.projection, err = newProjectionWorklist(s.search)
	if err != nil {
		return s, err
	}
	if err = s.explanation.bindProjection(s.projection, config.Factor); err != nil {
		return s, err
	}
	s.resolution, err = newSearchResolution(s.search)
	return s, err
}

func (s *Solver) open() error {
	if s == nil || s.closed || s.search == nil || s.resolution == nil {
		return ErrSearchOwner
	}
	return s.search.open()
}

func (s *Solver) PolynomialManager() (*polynomial.MultivariatePolynomialManager, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.config.Polynomial, nil
}

func (s *Solver) AlgebraicManager() (*polynomial.AlgebraicManager, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.config.Algebraic, nil
}

func (s *Solver) MkVar(integer bool) (uint32, error) {
	if err := s.open(); err != nil {
		return NullVariable, err
	}
	return s.clauses.mkVar(integer)
}

// MkExprPolynomialVariable implements ast.ExprPolynomialVariables for the
// retained goal-to-NLSAT specialization.
func (s *Solver) MkExprPolynomialVariable(integer bool) (uint32, error) {
	return s.MkVar(integer)
}

// IsExprPolynomialInteger implements ast.ExprPolynomialVariables for the
// retained goal-to-NLSAT specialization.
func (s *Solver) IsExprPolynomialInteger(variable uint32) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if uint64(variable) >= uint64(len(s.clauses.vars.isInt)) {
		return false, arena.ErrBounds
	}
	return s.clauses.vars.isInt[variable], nil
}

func (s *Solver) MkBoolVar() (sat.BoolVar, error) {
	if err := s.open(); err != nil {
		return sat.NullBoolVar, err
	}
	return s.atoms.mkBoolVar()
}

func (s *Solver) MkInequality(kind AtomKind, factors []polynomial.Polynomial, even []bool, negative bool) (sat.Literal, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if len(factors) == 0 || len(factors) != len(even) {
		return 0, arena.ErrBounds
	}
	constant := true
	sign := 1
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return 0, err
	}
	for i, factor := range factors {
		max, err := core.MaxVariable(factor)
		if err != nil {
			return 0, err
		}
		if max != NullVariable {
			constant = false
			break
		}
		current, err := s.config.Algebraic.EvalSign(s.config.Polynomial, factor, s.assignment)
		if err != nil {
			return 0, err
		}
		if current == 0 {
			sign = 0
		} else if !even[i] {
			sign *= current
		}
	}
	if constant {
		literal := TrueLiteral
		if !satisfied(sign, kind, negative) {
			literal = literal.Negated()
		}
		return literal, nil
	}
	literal, err := s.atoms.mkInequalityLiteral(kind, factors, even)
	if negative {
		literal = literal.Negated()
	}
	return literal, err
}

func (s *Solver) MkRoot(kind AtomKind, variable, index uint32, p polynomial.Polynomial, negative bool) (sat.Literal, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	b, err := s.atoms.mkRoot(kind, variable, index, p)
	if err != nil {
		return 0, err
	}
	return sat.MakeLiteral(b, negative), nil
}

func (s *Solver) AddClause(literals []sat.Literal, assumption Assumption) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.clauses.mkInputClause(literals, assumption)
}

func (s *Solver) Check() (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	for {
		fullDimensional, err := s.clauses.isFullDimensional()
		if err != nil {
			return lbool.Undef, err
		}
		s.explanation.fullDimensional = fullDimensional
		result, err := s.search.search(s.resolution, s.explanation)
		if err != nil || result != lbool.True {
			return result, err
		}
		var cuts []integerCut
		q, err := s.config.Algebraic.RationalManager()
		if err != nil {
			return lbool.Undef, err
		}
		releaseCuts := func() error {
			var cause error
			for i := len(cuts) - 1; i >= 0; i-- {
				cause = errors.Join(cause, q.Del(&cuts[i].floor))
			}
			cuts = nil
			return cause
		}
		for variable, integer := range s.clauses.vars.isInt {
			if !integer {
				continue
			}
			assigned, err := s.assignment.Contains(uint32(variable))
			if err != nil {
				return lbool.Undef, errors.Join(err, releaseCuts())
			}
			if !assigned {
				continue
			}
			value, err := s.assignment.Value(uint32(variable))
			if err != nil {
				return lbool.Undef, errors.Join(err, releaseCuts())
			}
			integerValue, err := s.config.Algebraic.IsInt(value)
			if err != nil {
				return lbool.Undef, errors.Join(err, releaseCuts())
			}
			if integerValue {
				continue
			}
			cut := integerCut{variable: uint32(variable)}
			if err = s.integerFloor(value, &cut.floor); err != nil {
				return lbool.Undef, errors.Join(err, q.Del(&cut.floor), releaseCuts())
			}
			cuts = append(cuts, cut)
		}
		if len(cuts) == 0 {
			return lbool.True, nil
		}
		for i := range cuts {
			if err := s.addIntegerCut(cuts[i].variable, &cuts[i].floor); err != nil {
				return lbool.Undef, errors.Join(err, releaseCuts())
			}
		}
		if err := releaseCuts(); err != nil {
			return lbool.Undef, err
		}
	}
}

func (s *Solver) Statistics() (map[string]float64, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return map[string]float64{
		"nlsat conflicts":              float64(s.search.conflicts),
		"nlsat propagations":           float64(s.search.propagations),
		"nlsat decisions":              float64(s.search.decisions),
		"nlsat stages":                 float64(s.search.stages),
		"nlsat irrational assignments": float64(s.search.irrationalAssignments),
	}, nil
}

func (s *Solver) ResetStatistics() error {
	if err := s.open(); err != nil {
		return err
	}
	s.search.conflicts = 0
	s.search.propagations = 0
	s.search.decisions = 0
	s.search.stages = 0
	s.search.irrationalAssignments = 0
	return nil
}

func (s *Solver) integerFloor(value *polynomial.Algebraic, out *unumeral.Rational) (err error) {
	var lower polynomial.Algebraic
	if err = s.config.Algebraic.IntLT(value, &lower); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.config.Algebraic.Del(&lower)) }()
	if err = s.config.Algebraic.ToRational(&lower, out); err != nil {
		return err
	}
	q, err := s.config.Algebraic.RationalManager()
	if err != nil {
		return err
	}
	for {
		if err = q.Inc(out); err != nil {
			return err
		}
		greater, err := s.config.Algebraic.GreaterRational(value, out)
		if err != nil {
			return err
		}
		if !greater {
			return q.Dec(out)
		}
	}
}

func (s *Solver) addIntegerCut(variable uint32, floor *unumeral.Rational) (err error) {
	q, err := s.config.Algebraic.RationalManager()
	if err != nil {
		return err
	}
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return err
	}
	var one, upper, constant unumeral.Rational
	defer func() { err = errors.Join(err, q.Del(&constant), q.Del(&upper), q.Del(&one)) }()
	if err = q.SetInt32(&one, 1); err != nil {
		return err
	}
	if err = q.Copy(&upper, floor); err != nil {
		return err
	}
	if err = q.Inc(&upper); err != nil {
		return err
	}
	lemma := make([]sat.Literal, 0, 2)
	for _, bound := range []*unumeral.Rational{floor, &upper} {
		if err = q.Copy(&constant, bound); err != nil {
			return err
		}
		if err = q.Neg(&constant); err != nil {
			return err
		}
		p, cause := core.MkRationalLinear([]unumeral.Rational{one}, []uint32{variable}, &constant)
		if cause != nil {
			return cause
		}
		ref, cause := polynomial.NewPolynomialRef(core)
		if cause != nil {
			return cause
		}
		if cause = ref.Assign(p); cause != nil {
			return errors.Join(cause, ref.Close())
		}
		kind := AtomGT
		if len(lemma) == 1 {
			kind = AtomLT
		}
		literal, cause := s.MkInequality(kind, []polynomial.Polynomial{p}, []bool{false}, true)
		cause = errors.Join(cause, ref.Close())
		if cause != nil {
			return cause
		}
		lemma = append(lemma, literal)
		if err = q.Del(&constant); err != nil {
			return err
		}
		constant = unumeral.Rational{}
	}
	return s.AddClause(lemma, Assumption{})
}

func (s *Solver) Value(variable uint32, out *polynomial.Algebraic) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return polynomial.ErrAlgebraicOwner
	}
	assigned, err := s.assignment.Contains(variable)
	if err != nil {
		return err
	}
	if !assigned {
		return arena.ErrBounds
	}
	value, err := s.assignment.Value(variable)
	if err != nil {
		return err
	}
	return s.config.Algebraic.Copy(out, value)
}

func (s *Solver) BooleanValue(variable sat.BoolVar) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	if uint64(variable) >= uint64(len(s.atoms.bools.values)) {
		return lbool.Undef, arena.ErrBounds
	}
	return s.atoms.bools.values[variable], nil
}

// IsInterpreted reports whether a Boolean variable is backed by an arithmetic
// atom rather than a frontend Boolean abstraction.
func (s *Solver) IsInterpreted(variable sat.BoolVar) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] {
		return false, arena.ErrBounds
	}
	return s.atoms.bools.atoms[variable] != nil, nil
}

func (s *Solver) Core(out *[]Assumption) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	*out = (*out)[:0]
	return s.assumptions.linearize(s.resolution.lemmaAssumptions, out)
}

func (s *Solver) Close() (err error) {
	if s == nil || s.closed {
		return nil
	}
	if s.resolution != nil {
		err = errors.Join(err, s.resolution.close())
		if err == nil {
			s.resolution = nil
		}
	}
	if err == nil && s.search != nil {
		err = s.search.close()
		if err == nil {
			s.search, s.explanation, s.projection = nil, nil, nil
		}
	}
	if err == nil && s.evaluator != nil {
		err = s.evaluator.Close()
		if err == nil {
			s.evaluator = nil
		}
	}
	if err == nil && s.assignment != nil {
		err = s.assignment.Close()
		if err == nil {
			s.assignment = nil
		}
	}
	if err == nil && s.clauses != nil {
		err = s.clauses.close()
		if err == nil {
			s.clauses = nil
		}
	}
	if err == nil && s.assumptions != nil {
		err = s.assumptions.close()
		if err == nil {
			s.assumptions = nil
		}
	}
	if err == nil && s.atoms != nil {
		if cleanup := s.atoms.cleanup(); cleanup != nil {
			err = cleanup
		}
		for _, atom := range s.atoms.bools.atoms {
			if err != nil {
				break
			}
			if atom != nil {
				err = s.atoms.deleteAtom(atom)
			}
		}
		if err == nil {
			err = s.atoms.close()
		}
		if err == nil {
			s.atoms = nil
		}
	}
	if err == nil && s.cache != nil {
		err = s.cache.Close()
		if err == nil {
			s.cache = nil
		}
	}
	if err == nil && s.allocator != nil {
		err = s.allocator.Close()
		if err == nil {
			s.allocator = nil
		}
	}
	if err == nil {
		s.closed = true
	}
	return err
}
