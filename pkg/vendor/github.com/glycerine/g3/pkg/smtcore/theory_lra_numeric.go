package smtcore

// Source: src/smt/theory_lra.cpp imp::init_solver, get_var_index,
// init_left_side, internalize_def, assert_bound, get_ivalue and make_feasible.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/lp"
	"github.com/glycerine/g3/pkg/util/numeral"
)

const smtLRANoIndex = ^uint32(0)

type smtLRACoefficient struct {
	variable SMTTheoryVar
	value    big.Rat
}

type smtLRANumericConfig struct {
	strategy         lp.SimplexStrategy
	boundPropagation bool
	pivotedRows      bool
	limit            lp.ResourceLimit
}

type smtLRAEvidence struct {
	constraint  uint32
	literal     SMTLiteral
	coefficient big.Rat
}

// smtLRANumeric is the concrete LAR owner of the retained LRA implementation.
// Its manager is borrowed and must outlive Close. It does not substitute the
// math/simplex algorithm for LAR. SMTLRATheory owns the AST/event half, and
// automatic QF_LRA/QF_UFLRA setup selects it at the first setup boundary.
type smtLRANumeric struct {
	manager      *numeral.RationalManager
	solver       *lp.LARSolver
	indices      []uint32
	variables    []SMTTheoryVar
	terms        []SMTTheoryVar
	trail        []SMTTheoryVar
	limits       []int
	inequalities []SMTLiteral
	assertLower  uint64
	assertUpper  uint64
	statistics   smtLRANumericStatistics
	// Failed LAR calls may retain borrowed operands in their cleanup state.
	// Release these only after closing the solver that borrowed them.
	pending     []numeral.Rational
	model       *lp.LARRationalModel
	explanation *lp.SparseRationalVector
	stopped     bool
}

func newSMTLRANumeric(manager *numeral.RationalManager, config smtLRANumericConfig) (*smtLRANumeric, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	solver, err := lp.NewLARSolver(manager)
	owner := &smtLRANumeric{manager: manager, solver: solver, stopped: err != nil}
	if err != nil {
		// A failed numerical constructor can still own cleanup state. Return
		// its owner so a caller can release it after lifting the refusal.
		return owner, err
	}
	solver.Settings.SimplexStrategy = config.strategy
	solver.Settings.BoundPropagation = config.boundPropagation
	solver.Settings.SetResourceLimit(config.limit)
	solver.SetPropagateBoundsOnPivotedRowsMode(config.pivotedRows)
	return owner, nil
}

func (s *smtLRANumeric) ready() error {
	if s == nil || s.solver == nil {
		return ast.ErrASTHandle
	}
	if s.stopped {
		return lp.ErrExactVectorPending
	}
	return nil
}

func (s *smtLRANumeric) finish(values []numeral.Rational, cause error) error {
	if cause != nil {
		s.stopped = true
		s.pending = append(s.pending, values...)
		return cause
	}
	for i := range values {
		if err := s.manager.Del(&values[i]); err != nil {
			s.stopped = true
			s.pending = append(s.pending, values[i:]...)
			return err
		}
	}
	return nil
}

func (s *smtLRANumeric) index(v SMTTheoryVar) (uint32, bool) {
	if v < 0 || int(v) >= len(s.indices) {
		return smtLRANoIndex, false
	}
	index := s.indices[v]
	return index, index != smtLRANoIndex
}

func (s *smtLRANumeric) setIndex(v SMTTheoryVar, index uint32) {
	for len(s.indices) <= int(v) {
		s.indices = append(s.indices, smtLRANoIndex)
	}
	s.indices[v] = index
	if s.solver.IsTerm(index) {
		for len(s.terms) <= int(index-lp.LARTermStartIndex) {
			s.terms = append(s.terms, SMTNullTheoryVar)
		}
		s.terms[index-lp.LARTermStartIndex] = v
	} else {
		for len(s.variables) <= int(index) {
			s.variables = append(s.variables, SMTNullTheoryVar)
		}
		s.variables[index] = v
	}
	s.trail = append(s.trail, v)
}

func (s *smtLRANumeric) getVarIndex(v SMTTheoryVar) (index uint32, err error) {
	if err = s.ready(); err != nil {
		return smtLRANoIndex, err
	}
	if v < 0 {
		return smtLRANoIndex, ast.ErrASTDomain
	}
	if index, ok := s.index(v); ok {
		return index, nil
	}
	index, err = s.solver.AddVar(uint32(v))
	if err != nil {
		s.stopped = true
		return index, err
	}
	s.setIndex(v, index)
	return index, nil
}

// Source m_columns combines repeats, then emits the first occurrence of each
// nonzero variable in input order. A cancelled variable is never registered.
func (s *smtLRANumeric) leftSide(coefficients []smtLRACoefficient) ([]lp.RationalSparseEntry, []numeral.Rational, error) {
	sums := make(map[SMTTheoryVar]*big.Rat)
	var order []SMTTheoryVar
	for _, coefficient := range coefficients {
		if coefficient.variable < 0 {
			return nil, nil, ast.ErrASTDomain
		}
		value := sums[coefficient.variable]
		if value == nil {
			value = new(big.Rat)
			sums[coefficient.variable] = value
			order = append(order, coefficient.variable)
		}
		value.Add(value, &coefficient.value)
	}
	values := make([]numeral.Rational, len(order))
	for i := range values {
		values[i] = numeral.NewRational()
	}
	var result []lp.RationalSparseEntry
	for i, v := range order {
		if sums[v].Sign() == 0 {
			continue
		}
		index, err := s.getVarIndex(v)
		if err != nil {
			return nil, values, err
		}
		if err = s.manager.SetString(&values[i], sums[v].RatString()); err != nil {
			return nil, values, err
		}
		result = append(result, lp.RationalSparseEntry{Index: index, Value: values[i]})
	}
	return result, values, nil
}

func (s *smtLRANumeric) addTerm(v SMTTheoryVar, coefficients []smtLRACoefficient, constant *big.Rat) (index uint32, err error) {
	if err = s.ready(); err != nil {
		return smtLRANoIndex, err
	}
	if v < 0 || constant == nil {
		return smtLRANoIndex, ast.ErrASTDomain
	}
	entries, values, err := s.leftSide(coefficients)
	defer func() { err = s.finish(values, err) }()
	if err != nil {
		return smtLRANoIndex, err
	}
	// Native internalize_def builds its left side before checking the cache.
	if index, ok := s.index(v); ok {
		return index, nil
	}
	values = append(values, numeral.NewRational())
	if err = s.manager.SetString(&values[len(values)-1], constant.RatString()); err != nil {
		return smtLRANoIndex, err
	}
	index, err = s.solver.AddTerm(entries, &values[len(values)-1])
	if err == nil {
		s.setIndex(v, index)
	}
	return index, err
}

func (s *smtLRANumeric) addBound(v SMTTheoryVar, value *big.Rat, upper, truth bool) (ci uint32, err error) {
	if err = s.ready(); err != nil {
		return smtLRANoIndex, err
	}
	if v < 0 || value == nil {
		return smtLRANoIndex, ast.ErrASTDomain
	}
	if s.solver.Bounds.Status.Value == lp.StatusInfeasible {
		return smtLRANoIndex, nil
	}
	index, err := s.getVarIndex(v)
	if err != nil {
		return smtLRANoIndex, err
	}
	kind := lp.LConstraintGE
	if upper {
		kind = lp.LConstraintLE
	}
	if !truth {
		kind = -kind / 2
	}
	// Preserve the native counter names, including their reversed direction.
	if kind < 0 {
		s.assertLower++
	} else {
		s.assertUpper++
	}
	values := []numeral.Rational{numeral.NewRational()}
	defer func() { err = s.finish(values, err) }()
	if err = s.manager.SetString(&values[0], value.RatString()); err != nil {
		return smtLRANoIndex, err
	}
	ci, err = s.solver.AddVarBound(index, kind, &values[0])
	if err != nil {
		return ci, err
	}
	return ci, nil
}

func (s *smtLRANumeric) registerBoundConstraint(ci uint32, bv int32, truth bool) {
	for len(s.inequalities) <= int(ci) {
		s.inequalities = append(s.inequalities, SMTNullLiteral)
	}
	s.inequalities[ci] = NewSMTLiteral(bv, !truth)
	s.statistics.rows++
}

// Direct numerical consumers retain both phases. The AST frontend emits its
// source assert_bound trace between creation and inequality publication.
func (s *smtLRANumeric) assertBound(v SMTTheoryVar, bv int32, value *big.Rat, upper, truth bool) (uint32, error) {
	if err := s.ready(); err != nil {
		return smtLRANoIndex, err
	}
	if bv < 0 {
		return smtLRANoIndex, ast.ErrASTDomain
	}
	ci, err := s.addBound(v, value, upper, truth)
	if err == nil && ci != smtLRANoIndex {
		s.registerBoundConstraint(ci, bv, truth)
	}
	return ci, err
}

func (s *smtLRANumeric) startFeasibility() error {
	if err := s.ready(); err != nil {
		return err
	}
	s.statistics.makeFeasible++
	if n := s.solver.Core.RA.ColumnCount(); n > s.statistics.maxColumns {
		s.statistics.maxColumns = n
	}
	if n := s.solver.Core.RA.RowCount(); n > s.statistics.maxRows {
		s.statistics.maxRows = n
	}
	return nil
}

func (s *smtLRANumeric) findFeasible() (lbool.LBool, lp.Status, error) {
	status, err := s.solver.FindFeasibleSolution()
	if err != nil {
		s.stopped = true
		return lbool.Undef, status, err
	}
	s.statistics.iterations = s.solver.Settings.Stats.TotalIterations
	s.statistics.factorizations = s.solver.Settings.Stats.Factorizations
	s.statistics.pivots = s.solver.Settings.Stats.NeedToSolveInf
	switch status {
	case lp.StatusFeasible, lp.StatusOptimal:
		return lbool.True, status, nil
	case lp.StatusInfeasible:
		return lbool.False, status, nil
	default:
		return lbool.Undef, status, nil
	}
}

func (s *smtLRANumeric) makeFeasible() (lbool.LBool, error) {
	if err := s.startFeasibility(); err != nil {
		return lbool.Undef, err
	}
	result, _, err := s.findFeasible()
	return result, err
}

// Source has_bound queries proper columns through LAR; terms have their own
// frontend directory. A refused output copy stays owned until LAR closes.
func (s *smtLRANumeric) hasRationalBound(index uint32, lower bool, expected *big.Rat) (witness uint32, found bool, err error) {
	if err = s.ready(); err != nil {
		return smtLRANoIndex, false, err
	}
	values := []numeral.Rational{numeral.NewRational()}
	defer func() { err = s.finish(values, err) }()
	strict := false
	witness = smtLRANoIndex
	if lower {
		found, err = s.solver.HasLowerBound(index, &witness, &values[0], &strict)
	} else {
		found, err = s.solver.HasUpperBound(index, &witness, &values[0], &strict)
	}
	if err != nil || !found || strict {
		return witness, false, err
	}
	value, err := s.rational(&values[0])
	if err != nil {
		return witness, false, err
	}
	return witness, value.Cmp(expected) == 0, nil
}

func (s *smtLRANumeric) rational(value *numeral.Rational) (*big.Rat, error) {
	text, err := s.manager.String(value)
	if err != nil {
		return nil, err
	}
	result, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return result, nil
}

func (s *smtLRANumeric) columnValue(index uint32) (simplex.Infinitesimal, error) {
	value, err := s.solver.Value(index)
	if err != nil {
		return simplex.Infinitesimal{}, err
	}
	r, err := s.rational(&value.X)
	if err != nil {
		return simplex.Infinitesimal{}, err
	}
	e, err := s.rational(&value.Y)
	if err != nil {
		return simplex.Infinitesimal{}, err
	}
	return simplex.NewInfinitesimal(r, e), nil
}

func (s *smtLRANumeric) value(v SMTTheoryVar) (simplex.Infinitesimal, error) {
	if err := s.ready(); err != nil {
		return simplex.Infinitesimal{}, err
	}
	index, ok := s.index(v)
	if !ok || !s.solver.VarIsRegistered(index) {
		return simplex.Infinitesimal{}, ast.ErrASTDomain
	}
	if !s.solver.IsTerm(index) {
		return s.columnValue(index)
	}
	term := s.solver.Terms[index-lp.LARTermStartIndex]
	constant, err := s.rational(&term.Value)
	if err != nil {
		return simplex.Infinitesimal{}, err
	}
	result := simplex.RationalInfinitesimal(constant)
	for j, coefficient := range term.Coefficients().All() {
		value, err := s.columnValue(j)
		if err != nil {
			return simplex.Infinitesimal{}, err
		}
		c, err := s.rational(coefficient)
		if err != nil {
			return simplex.Infinitesimal{}, err
		}
		value.Mul(c)
		result.Add(&value)
	}
	return result, nil
}

// Source imp::maximize uses the LAR term coefficients and adds its constant
// after maximize_term. False from that routine has the source infinity meaning.
// Its caller must retain the optsmt search/blocker loop: a raw callback can
// return an intermediate local improvement with the preceding OPTIMAL status.
func (s *smtLRANumeric) maximize(v SMTTheoryVar) (result SMTArithmeticObjectiveValue, err error) {
	if err = s.ready(); err != nil {
		return result, err
	}
	index, ok := s.index(v)
	if !ok {
		return SMTArithmeticObjectiveValue{Unbounded: true}, nil
	}
	var entries []lp.RationalSparseEntry
	var constant big.Rat
	values := []numeral.Rational{numeral.NewRational(), numeral.NewRational(), numeral.NewRational()}
	defer func() { err = s.finish(values, err) }()
	if s.solver.IsTerm(index) {
		term := s.solver.Terms[index-lp.LARTermStartIndex]
		c, e := s.rational(&term.Value)
		if e != nil {
			return result, e
		}
		constant.Set(c)
		for j, coefficient := range term.Coefficients().All() {
			entries = append(entries, lp.RationalSparseEntry{Index: j, Value: *coefficient})
		}
	} else {
		if err = s.manager.SetInt32(&values[0], 1); err != nil {
			return result, err
		}
		entries = append(entries, lp.RationalSparseEntry{Index: index, Value: values[0]})
	}
	out := lp.NewRationalPair()
	bounded, err := s.solver.MaximizeTerm(entries, &out)
	values[1], values[2] = out.X, out.Y
	if err != nil {
		return result, err
	}
	if !bounded {
		return SMTArithmeticObjectiveValue{Unbounded: true}, nil
	}
	r, err := s.rational(&out.X)
	if err != nil {
		return result, err
	}
	e, err := s.rational(&out.Y)
	if err != nil {
		return result, err
	}
	r.Add(r, &constant)
	return SMTArithmeticObjectiveValue{Finite: simplex.NewInfinitesimal(r, e)}, nil
}

// Source init_variable_values/get_value: GetModel supplies column values using
// the actual strict-bound delta. Term values are reconstructed in LAR order.
// Returned values own their math/big storage and survive the next solver call.
func (s *smtLRANumeric) modelValues(vars []SMTTheoryVar) (result map[SMTTheoryVar]*big.Rat, err error) {
	columns, err := s.modelColumnValues()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			s.stopped = true
		}
	}()
	result = make(map[SMTTheoryVar]*big.Rat)
	for _, v := range vars {
		value := new(big.Rat)
		result[v] = value
		index, ok := s.index(v)
		if !ok {
			continue
		}
		if !s.solver.IsTerm(index) {
			if c := columns[index]; c != nil {
				value.Set(c)
			}
			continue
		}
		term := s.solver.Terms[index-lp.LARTermStartIndex]
		c, e := s.rational(&term.Value)
		if e != nil {
			return nil, e
		}
		value.Set(c)
		for j, coefficient := range term.Coefficients().All() {
			c, e = s.rational(coefficient)
			if e != nil {
				return nil, e
			}
			if column := columns[j]; column != nil {
				value.Add(value, new(big.Rat).Mul(c, column))
			}
		}
	}
	return result, nil
}

func (s *smtLRANumeric) modelColumnValues() (columns map[uint32]*big.Rat, err error) {
	if err = s.ready(); err != nil {
		return nil, err
	}
	s.model = lp.NewLARRationalModel(s.manager)
	defer func() {
		if err != nil {
			s.stopped = true
		}
	}()
	if err = s.solver.GetModel(s.model); err != nil {
		return nil, err
	}
	columns = make(map[uint32]*big.Rat)
	for j, value := range s.model.Values {
		columns[j], err = s.rational(value)
		if err != nil {
			return nil, err
		}
	}
	if err = s.model.Close(); err != nil {
		return nil, err
	}
	s.model = nil
	return columns, nil
}

func (s *smtLRANumeric) infeasibilityEvidence() (result []smtLRAEvidence, err error) {
	if err = s.ready(); err != nil {
		return nil, err
	}
	s.explanation = lp.NewSparseRationalVector(s.manager)
	defer func() {
		if err != nil {
			s.stopped = true
		}
	}()
	if err = s.solver.GetInfeasibilityExplanation(s.explanation); err != nil {
		return nil, err
	}
	for _, entry := range s.explanation.Data {
		coefficient, e := s.rational(&entry.Value)
		if e != nil {
			return nil, e
		}
		literal := SMTNullLiteral
		if coefficient.Sign() != 0 && (entry.Index >= s.solver.NumberOfConstraints() || int(entry.Index) >= len(s.inequalities) || s.inequalities[entry.Index] == SMTNullLiteral) {
			return nil, ast.ErrASTDomain
		}
		if int(entry.Index) < len(s.inequalities) {
			literal = s.inequalities[entry.Index]
		}
		// Source display_evidence's second pass includes zero coefficients.
		// Consumers that form conflict premises filter them independently.
		result = append(result, smtLRAEvidence{entry.Index, literal, *coefficient})
	}
	if err = s.explanation.Close(); err != nil {
		return nil, err
	}
	s.explanation = nil
	return result, nil
}

func (s *smtLRANumeric) push() error {
	if err := s.ready(); err != nil {
		return err
	}
	s.limits = append(s.limits, len(s.trail))
	if err := s.solver.Push(); err != nil {
		s.stopped = true
		return err
	}
	return nil
}

func (s *smtLRANumeric) pop(n uint32) error {
	if err := s.ready(); err != nil {
		return err
	}
	if uint64(n) > uint64(len(s.limits)) {
		return ast.ErrASTDomain
	}
	if n == 0 {
		return nil
	}
	level := len(s.limits) - int(n)
	limit := s.limits[level]
	for _, v := range s.trail[limit:] {
		index := s.indices[v]
		if s.solver.IsTerm(index) {
			s.terms[index-lp.LARTermStartIndex] = SMTNullTheoryVar
		} else if int(index) < len(s.variables) {
			s.variables[index] = SMTNullTheoryVar
		}
		s.indices[v] = smtLRANoIndex
	}
	s.trail = s.trail[:limit]
	s.limits = s.limits[:level]
	if err := s.solver.Pop(n); err != nil {
		s.stopped = true
		return err
	}
	return nil
}

func (s *smtLRANumeric) Close() error {
	if s == nil || s.solver == nil {
		return nil
	}
	if s.model != nil {
		if err := s.model.Close(); err != nil {
			return err
		}
		s.model = nil
	}
	if s.explanation != nil {
		if err := s.explanation.Close(); err != nil {
			return err
		}
		s.explanation = nil
	}
	if err := s.solver.Close(); err != nil {
		return err
	}
	for i := range s.pending {
		if err := s.manager.Del(&s.pending[i]); err != nil {
			return err
		}
	}
	s.pending = nil
	s.solver = nil
	s.stopped = true
	return nil
}
