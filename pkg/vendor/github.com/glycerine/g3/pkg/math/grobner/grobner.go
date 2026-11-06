package grobner

// Package grobner implements the retained, unindexed Grobner basis engine from
// src/math/grobner/grobner.{h,cpp}.

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

const (
	arithNumeral = int32(0)
	arithAdd     = int32(6)
	arithMul     = int32(9)
)

// Stats is the source grobner_stats record.
type Stats struct {
	Simplify, Superpose, ComputeBasis, NumProcessed int64
}

func (s *Stats) Reset() { *s = Stats{} }

// Monomial owns references to its sorted variables until it is consumed by an
// assertion or released with DelMonomial.
type Monomial struct {
	coeff big.Rat
	vars  []ast.ASTHandle
	owner *Solver
	live  bool
}

func (m *Monomial) Coefficient() *big.Rat {
	if m == nil {
		return new(big.Rat)
	}
	return new(big.Rat).Set(&m.coeff)
}
func (m *Monomial) Degree() uint32 { return uint32(len(m.vars)) }
func (m *Monomial) Size() uint32   { return uint32(len(m.vars)) }
func (m *Monomial) Variable(i uint32) ast.ASTHandle {
	if m == nil || i >= uint32(len(m.vars)) {
		return ast.ASTHandle{}
	}
	return m.vars[i]
}
func (m *Monomial) Variables() []ast.ASTHandle {
	if m == nil {
		return nil
	}
	return append([]ast.ASTHandle(nil), m.vars...)
}

// Equation is borrowed from Solver and remains valid until the solver deletes
// it during a later basis computation, pop, reset, or close.
type Equation struct {
	scope      uint32
	index      uint32
	linear     bool
	monomials  []*Monomial
	dependency ast.ASTDependency
	owner      *Solver
	live       bool
}

func (e *Equation) NumMonomials() uint32 { return uint32(len(e.monomials)) }
func (e *Equation) Monomial(i uint32) *Monomial {
	if e == nil || i >= uint32(len(e.monomials)) {
		return nil
	}
	return e.monomials[i]
}
func (e *Equation) Monomials() []*Monomial {
	if e == nil {
		return nil
	}
	return append([]*Monomial(nil), e.monomials...)
}
func (e *Equation) Dependency() ast.ASTDependency { return e.dependency }
func (e *Equation) IsLinearCombination() bool     { return e.linear }

type equationSet struct {
	items []*Equation
	has   map[*Equation]bool
}

func (s *equationSet) insert(e *Equation) {
	if e == nil {
		return
	}
	if s.has == nil {
		s.has = make(map[*Equation]bool)
	}
	if !s.has[e] {
		s.has[e] = true
		s.items = append(s.items, e)
	}
}
func (s *equationSet) erase(e *Equation) {
	if !s.has[e] {
		return
	}
	delete(s.has, e)
	for i, x := range s.items {
		if x == e {
			copy(s.items[i:], s.items[i+1:])
			s.items = s.items[:len(s.items)-1]
			return
		}
	}
}
func (s *equationSet) reset() { s.items = nil; s.has = nil }
func (s *equationSet) snapshot() []*Equation {
	return append([]*Equation(nil), s.items...)
}

type scope struct{ unfreeze, delete int }

// Solver is the source grobner object. It is single-goroutine state tied to
// one AST manager.
type Solver struct {
	manager *ast.ASTManager
	family  ast.FamilyID
	weights map[ast.ASTHandle]int
	ids     map[ast.ASTHandle]uint32

	processed, pending  equationSet
	unfreeze, equations []*Equation
	scopes              []scope
	changedLeading      bool
	unsat               *Equation
	numNew              uint32
	closed              bool

	Stats Stats
}

func New(manager *ast.ASTManager) (*Solver, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	return &Solver{manager: manager, family: family, weights: make(map[ast.ASTHandle]int), ids: make(map[ast.ASTHandle]uint32)}, nil
}

func (s *Solver) check() error {
	if s == nil || s.closed || s.manager == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *Solver) ScopeLevel() uint32      { return uint32(len(s.scopes)) }
func (s *Solver) Inconsistent() bool      { return s != nil && s.unsat != nil }
func (s *Solver) NumNewEquations() uint32 { return s.numNew }

func (s *Solver) SetWeight(n ast.ASTHandle, weight int) error {
	if err := s.check(); err != nil {
		return err
	}
	if weight == 0 {
		return nil
	}
	if _, ok := s.weights[n]; !ok {
		if err := s.manager.IncRef(n); err != nil {
			return err
		}
	}
	s.weights[n] = weight
	return nil
}
func (s *Solver) Weight(n ast.ASTHandle) int { return s.weights[n] }

func (s *Solver) astID(n ast.ASTHandle) (uint32, error) {
	if id, ok := s.ids[n]; ok {
		return id, nil
	}
	id, err := s.manager.GetASTID(n)
	if err == nil {
		s.ids[n] = id
	}
	return id, err
}

func (s *Solver) varLess(a, b ast.ASTHandle) (bool, error) {
	if a == b {
		return false, nil
	}
	wa, wb := s.weights[a], s.weights[b]
	if wa != wb {
		return wa > wb, nil
	}
	ia, err := s.astID(a)
	if err != nil {
		return false, err
	}
	ib, err := s.astID(b)
	if err != nil {
		return false, err
	}
	return ia < ib, nil
}

func (s *Solver) monomialLess(a, b *Monomial) (bool, error) {
	if len(a.vars) != len(b.vars) {
		return len(a.vars) > len(b.vars), nil
	}
	for i := range a.vars {
		if a.vars[i] == b.vars[i] {
			continue
		}
		return s.varLess(a.vars[i], b.vars[i])
	}
	return false, nil
}

func (s *Solver) sortVars(v []ast.ASTHandle) error {
	var cause error
	sort.SliceStable(v, func(i, j int) bool {
		less, err := s.varLess(v[i], v[j])
		if err != nil {
			cause = errors.Join(cause, err)
		}
		return less
	})
	return cause
}

func (s *Solver) sortMonomials(ms []*Monomial) error {
	var cause error
	sort.SliceStable(ms, func(i, j int) bool {
		less, err := s.monomialLess(ms[i], ms[j])
		if err != nil {
			cause = errors.Join(cause, err)
		}
		return less
	})
	return cause
}

func (s *Solver) NewMonomial(coeff *big.Rat, vars []ast.ASTHandle) (*Monomial, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if coeff == nil {
		return nil, ast.ErrASTHandle
	}
	m := &Monomial{owner: s, live: true}
	m.coeff.Set(coeff)
	for _, v := range vars {
		if _, _, numeral, err := s.manager.ArithmeticNumeralText(v); err != nil {
			s.delMonomial(m)
			return nil, err
		} else if numeral {
			s.delMonomial(m)
			return nil, ast.ErrASTDomain
		}
		if err := s.manager.IncRef(v); err != nil {
			s.delMonomial(m)
			return nil, err
		}
		m.vars = append(m.vars, v)
	}
	if err := s.sortVars(m.vars); err != nil {
		s.delMonomial(m)
		return nil, err
	}
	return m, nil
}

func (s *Solver) delMonomial(m *Monomial) error {
	if m == nil || !m.live {
		return nil
	}
	if m.owner != s {
		return ast.ErrASTHandle
	}
	var result error
	for _, v := range m.vars {
		result = errors.Join(result, s.manager.DecRef(v))
	}
	m.vars = nil
	m.live = false
	return result
}
func (s *Solver) DelMonomial(m *Monomial) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.delMonomial(m)
}

func (s *Solver) cloneMonomial(m *Monomial) (*Monomial, error) {
	return s.NewMonomial(&m.coeff, m.vars)
}

func (s *Solver) monomialFromExpr(coeff *big.Rat, h ast.ASTHandle) (*Monomial, error) {
	text, _, numeral, err := s.manager.ArithmeticNumeralText(h)
	if err != nil {
		return nil, err
	}
	if numeral {
		value, ok := new(big.Rat).SetString(text)
		if !ok {
			return nil, ast.ErrASTDomain
		}
		value.Mul(value, coeff)
		return s.NewMonomial(value, nil)
	}
	isMul, err := s.isArith(h, arithMul)
	if err != nil {
		return nil, err
	}
	if !isMul {
		return s.NewMonomial(coeff, []ast.ASTHandle{h})
	}

	body := h
	value := new(big.Rat).Set(coeff)
	first, err := s.manager.GetArg(body, 0)
	if err != nil {
		return nil, err
	}
	if text, _, yes, e := s.manager.ArithmeticNumeralText(first); e != nil {
		return nil, e
	} else if yes {
		q, ok := new(big.Rat).SetString(text)
		if !ok {
			return nil, ast.ErrASTDomain
		}
		value.Mul(value, q)
		body, err = s.manager.GetArg(body, 1)
		if err != nil {
			return nil, err
		}
	}
	var vars []ast.ASTHandle
	for {
		mul, e := s.isArith(body, arithMul)
		if e != nil {
			return nil, e
		}
		if !mul {
			break
		}
		v, e := s.manager.GetArg(body, 0)
		if e != nil {
			return nil, e
		}
		vars = append(vars, v)
		body, e = s.manager.GetArg(body, 1)
		if e != nil {
			return nil, e
		}
	}
	vars = append(vars, body)
	return s.NewMonomial(value, vars)
}

func (s *Solver) isArith(h ast.ASTHandle, op int32) (bool, error) {
	kind, err := s.manager.GetKind(h)
	if err != nil {
		return false, err
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	family, err := s.manager.GetFamilyID(h)
	if err != nil {
		return false, err
	}
	if family != s.family {
		return false, nil
	}
	k, err := s.manager.GetDeclKind(h)
	return k == op, err
}

func sameBody(a, b *Monomial) bool {
	if len(a.vars) != len(b.vars) {
		return false
	}
	for i := range a.vars {
		if a.vars[i] != b.vars[i] {
			return false
		}
	}
	return true
}

func (s *Solver) simplifyMonomials(ms []*Monomial) ([]*Monomial, error) {
	if err := s.sortMonomials(ms); err != nil {
		return ms, err
	}
	if len(ms) == 0 {
		return ms, nil
	}
	out := ms[:0]
	for _, m := range ms {
		if len(out) != 0 && sameBody(out[len(out)-1], m) {
			out[len(out)-1].coeff.Add(&out[len(out)-1].coeff, &m.coeff)
			if err := s.delMonomial(m); err != nil {
				return out, err
			}
			continue
		}
		out = append(out, m)
	}
	keep := out[:0]
	for _, m := range out {
		if m.coeff.Sign() == 0 {
			if err := s.delMonomial(m); err != nil {
				return keep, err
			}
		} else {
			keep = append(keep, m)
		}
	}
	if len(keep) != 0 && keep[0].coeff.Cmp(big.NewRat(1, 1)) != 0 {
		lead := new(big.Rat).Set(&keep[0].coeff)
		for _, m := range keep {
			m.coeff.Quo(&m.coeff, lead)
		}
	}
	return keep, nil
}

func (s *Solver) initEquation(ms []*Monomial, dep ast.ASTDependency) (*Equation, error) {
	if err := s.manager.DependencyInc(dep); err != nil {
		return nil, err
	}
	e := &Equation{scope: s.ScopeLevel(), index: uint32(len(s.equations)), linear: true, monomials: ms, dependency: dep, owner: s, live: true}
	s.equations = append(s.equations, e)
	return e, nil
}

func (s *Solver) AssertMonomialsZero(ms []*Monomial, dep ast.ASTDependency) error {
	if err := s.check(); err != nil {
		return err
	}
	for _, m := range ms {
		if m == nil || !m.live || m.owner != s {
			return ast.ErrASTHandle
		}
	}
	work := append([]*Monomial(nil), ms...)
	result, err := s.simplifyMonomials(work)
	if err != nil {
		return err
	}
	if len(result) == 0 {
		return nil
	}
	e, err := s.initEquation(result, dep)
	if err != nil {
		return err
	}
	s.pending.insert(e)
	return nil
}

func (s *Solver) AssertExpressionsZero(expressions []ast.ASTHandle, dep ast.ASTDependency) error {
	coeffs := make([]big.Rat, len(expressions))
	for i := range coeffs {
		coeffs[i].SetInt64(1)
	}
	return s.AssertWeightedExpressionsZero(coeffs, expressions, dep)
}

func (s *Solver) AssertWeightedExpressionsZero(coeffs []big.Rat, expressions []ast.ASTHandle, dep ast.ASTDependency) error {
	if err := s.check(); err != nil {
		return err
	}
	if len(coeffs) != len(expressions) {
		return ast.ErrASTDomain
	}
	ms := make([]*Monomial, 0, len(expressions))
	for i, h := range expressions {
		m, err := s.monomialFromExpr(&coeffs[i], h)
		if err != nil {
			for _, x := range ms {
				_ = s.delMonomial(x)
			}
			return err
		}
		ms = append(ms, m)
	}
	return s.AssertMonomialsZero(ms, dep)
}

func (s *Solver) extractAdd(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	var out []ast.ASTHandle
	for {
		add, err := s.isArith(h, arithAdd)
		if err != nil {
			return nil, err
		}
		if !add {
			return append(out, h), nil
		}
		left, err := s.manager.GetArg(h, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, left)
		h, err = s.manager.GetArg(h, 1)
		if err != nil {
			return nil, err
		}
	}
}

func (s *Solver) AssertEquality(eq ast.ASTHandle, dep ast.ASTDependency) error {
	if err := s.check(); err != nil {
		return err
	}
	if kind, err := s.manager.GetDeclKind(eq); err != nil || kind != int32(ast.ASTOpEq) {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	lhs, err := s.manager.GetArg(eq, 0)
	if err != nil {
		return err
	}
	rhs, err := s.manager.GetArg(eq, 1)
	if err != nil {
		return err
	}
	text, integer, yes, err := s.manager.ArithmeticNumeralText(rhs)
	if err != nil {
		return err
	}
	if !yes {
		return ast.ErrASTDomain
	}
	terms, err := s.extractAdd(lhs)
	if err != nil {
		return err
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		return ast.ErrASTDomain
	}
	if q.Sign() != 0 {
		q.Neg(q)
		var c ast.ASTHandle
		if integer {
			c, err = s.manager.ArithmeticIntegerNumeralText(q.RatString())
		} else {
			c, err = s.manager.ArithmeticRealNumeralText(q.RatString())
		}
		if err != nil {
			return err
		}
		terms = append(terms, c)
	}
	return s.AssertExpressionsZero(terms, dep)
}

func (s *Solver) AssertMonomialTautology(h ast.ASTHandle) error {
	one, minus := big.NewRat(1, 1), big.NewRat(-1, 1)
	a, err := s.monomialFromExpr(one, h)
	if err != nil {
		return err
	}
	b, err := s.NewMonomial(minus, []ast.ASTHandle{h})
	if err != nil {
		_ = s.delMonomial(a)
		return err
	}
	return s.AssertMonomialsZero([]*Monomial{a, b}, ast.ASTDependency{})
}

func (s *Solver) deleteEquation(e *Equation) error {
	if e == nil || !e.live {
		return nil
	}
	s.processed.erase(e)
	s.pending.erase(e)
	if int(e.index) < len(s.equations) && s.equations[e.index] == e {
		s.equations[e.index] = nil
	}
	var result error
	for _, m := range e.monomials {
		result = errors.Join(result, s.delMonomial(m))
	}
	result = errors.Join(result, s.manager.DependencyDec(e.dependency))
	e.monomials = nil
	e.live = false
	return result
}

func (s *Solver) deleteFrom(limit int) error {
	var result error
	for _, e := range s.equations[limit:] {
		result = errors.Join(result, s.deleteEquation(e))
	}
	s.equations = s.equations[:limit]
	return result
}

func (s *Solver) Push() error {
	if err := s.check(); err != nil {
		return err
	}
	s.scopes = append(s.scopes, scope{len(s.unfreeze), len(s.equations)})
	return nil
}

func (s *Solver) Pop(n uint32) error {
	if err := s.check(); err != nil {
		return err
	}
	if n > uint32(len(s.scopes)) {
		return ast.ErrASTDomain
	}
	if n == 0 {
		return nil
	}
	level := len(s.scopes) - int(n)
	mark := s.scopes[level]
	for _, e := range s.unfreeze[mark.unfreeze:] {
		if e != nil && e.live {
			s.pending.insert(e)
		}
	}
	s.unfreeze = s.unfreeze[:mark.unfreeze]
	err := s.deleteFrom(mark.delete)
	s.scopes = s.scopes[:level]
	s.recomputeUnsat()
	return err
}

func (s *Solver) recomputeUnsat() {
	s.unsat = nil
	for _, e := range s.equations {
		if e != nil && e.live && inconsistent(e) {
			s.unsat = e
			return
		}
	}
}

func (s *Solver) Reset() error {
	if err := s.check(); err != nil {
		return err
	}
	err := s.deleteFrom(0)
	s.processed.reset()
	s.pending.reset()
	s.unfreeze = nil
	s.scopes = nil
	s.unsat = nil
	return err
}

func (s *Solver) Close() error {
	if s == nil || s.closed {
		return nil
	}
	var result error
	if s.manager != nil {
		result = errors.Join(result, s.deleteFrom(0))
		for h := range s.weights {
			result = errors.Join(result, s.manager.DecRef(h))
		}
	}
	s.closed = true
	s.manager = nil
	s.weights = nil
	s.ids = nil
	return result
}

func (s *Solver) updateEquationOrder(e *Equation) (bool, error) {
	if len(e.monomials) == 0 {
		return false, nil
	}
	first := e.monomials[0]
	for _, m := range e.monomials {
		if err := s.sortVars(m.vars); err != nil {
			return false, err
		}
	}
	if err := s.sortMonomials(e.monomials); err != nil {
		return false, err
	}
	return first != e.monomials[0], nil
}

func (s *Solver) UpdateOrder() error {
	if err := s.check(); err != nil {
		return err
	}
	for _, e := range s.pending.snapshot() {
		if _, err := s.updateEquationOrder(e); err != nil {
			return err
		}
	}
	for _, e := range s.processed.snapshot() {
		changed, err := s.updateEquationOrder(e)
		if err != nil {
			return err
		}
		if changed {
			s.processed.erase(e)
			s.pending.insert(e)
		}
	}
	return nil
}

func (s *Solver) Equations() []*Equation {
	if s == nil {
		return nil
	}
	r := append([]*Equation(nil), s.processed.items...)
	return append(r, s.pending.items...)
}

func (s *Solver) DisplayMonomial(w io.Writer, m *Monomial) error {
	if err := s.check(); err != nil {
		return err
	}
	if m == nil || !m.live || m.owner != s {
		return ast.ErrASTHandle
	}
	if len(m.vars) == 0 {
		_, err := io.WriteString(w, m.coeff.RatString())
		return err
	}
	if m.coeff.Cmp(big.NewRat(1, 1)) != 0 {
		if _, err := fmt.Fprintf(w, "%s*", m.coeff.RatString()); err != nil {
			return err
		}
	}
	for i := 0; i < len(m.vars); {
		j := i + 1
		for j < len(m.vars) && m.vars[j] == m.vars[i] {
			j++
		}
		if i != 0 {
			if _, err := io.WriteString(w, "*"); err != nil {
				return err
			}
		}
		id, err := s.astID(m.vars[i])
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "#%d", id); err != nil {
			return err
		}
		if j-i > 1 {
			if _, err = fmt.Fprintf(w, "^%d", j-i); err != nil {
				return err
			}
		}
		i = j
	}
	return nil
}

func (s *Solver) DisplayEquation(w io.Writer, e *Equation) error {
	if e == nil || !e.live || e.owner != s {
		return ast.ErrASTHandle
	}
	for i, m := range e.monomials {
		if i != 0 {
			if _, err := io.WriteString(w, " + "); err != nil {
				return err
			}
		}
		if err := s.DisplayMonomial(w, m); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, " = 0\n")
	return err
}

func (s *Solver) Display(w io.Writer) error {
	if len(s.processed.items) != 0 {
		if _, err := io.WriteString(w, "processed:\n"); err != nil {
			return err
		}
		for _, e := range s.processed.items {
			if err := s.DisplayEquation(w, e); err != nil {
				return err
			}
		}
	}
	if len(s.pending.items) != 0 {
		if _, err := io.WriteString(w, "to process:\n"); err != nil {
			return err
		}
		for _, e := range s.pending.items {
			if err := s.DisplayEquation(w, e); err != nil {
				return err
			}
		}
	}
	return nil
}

func inconsistent(e *Equation) bool { return len(e.monomials) == 1 && len(e.monomials[0].vars) == 0 }
