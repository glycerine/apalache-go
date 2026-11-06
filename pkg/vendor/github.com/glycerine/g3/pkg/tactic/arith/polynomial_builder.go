package arith

// Source: src/tactic/arith/propagate_ineqs_tactic.cpp:121-174.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// PolynomialBuilder owns the tactic's concrete rational coefficient buffer
// and expression pins. It borrows the AST manager and bound equation store.
type PolynomialBuilder struct {
	self     *PolynomialBuilder
	manager  *ast.ASTManager
	family   ast.FamilyID
	store    *BoundEquationStore
	coeff    *numeral.ScopedRationalVector
	ids      []uint32
	expr2var map[ast.ASTHandle]uint32
	var2expr []ast.ASTHandle
	refs     []ast.ASTRef
	closed   bool
}

func NewPolynomialBuilder(m *ast.ASTManager, store *BoundEquationStore) (*PolynomialBuilder, error) {
	if m == nil || store == nil {
		return nil, arena.ErrInvalidSlot
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	if err := store.open(); err != nil {
		return nil, err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	coeff, err := numeral.NewScopedRationalVector(store.manager)
	if err != nil {
		return nil, err
	}
	b := &PolynomialBuilder{manager: m, family: family, store: store, coeff: coeff,
		expr2var: make(map[ast.ASTHandle]uint32)}
	b.self = b
	return b, nil
}

func (b *PolynomialBuilder) open() error {
	if b == nil || b.self != b || b.manager == nil || b.store == nil || b.coeff == nil {
		return arena.ErrInvalidSlot
	}
	if b.closed {
		return arena.ErrClosed
	}
	return b.store.open()
}

func (b *PolynomialBuilder) isArithmetic(h ast.ASTHandle, operation int32) (bool, error) {
	kind, err := b.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := b.manager.GetFamilyID(h)
	if err != nil || family != b.family {
		return false, err
	}
	k, err := b.manager.GetDeclKind(h)
	return k == operation, err
}

func (b *PolynomialBuilder) mkVar(term ast.ASTHandle) (uint32, error) {
	toReal, err := b.isArithmetic(term, 14)
	if err != nil {
		return 0, err
	}
	if toReal {
		term, err = b.manager.GetArg(term, 0)
		if err != nil {
			return 0, err
		}
	}
	if x, found := b.expr2var[term]; found {
		return x, nil
	}
	if uint64(len(b.var2expr)) >= uint64(noEquationPosition) {
		return 0, arena.ErrBounds
	}
	sort, err := b.manager.GetSort(term)
	if err != nil {
		return 0, err
	}
	family, err := b.manager.GetFamilyID(sort)
	if err != nil {
		return 0, err
	}
	kind, err := b.manager.GetDeclKind(sort)
	if err != nil {
		return 0, err
	}
	x := uint32(len(b.var2expr))
	if err = b.store.MkVar(x, family == b.family && kind == 1); err != nil {
		return 0, err
	}
	ref, err := b.manager.NewRef(term)
	if err != nil {
		return 0, err
	}
	b.refs = append(b.refs, ref)
	b.expr2var[term] = x
	b.var2expr = append(b.var2expr, term)
	return x, nil
}

func (b *PolynomialBuilder) pushTerm(term ast.ASTHandle) (err error) {
	coefficient := numeral.NewRational()
	defer func() { err = errors.Join(err, b.store.manager.Del(&coefficient)) }()
	xTerm := term
	mul, err := b.isArithmetic(term, 9)
	if err != nil {
		return err
	}
	if mul {
		arity, e := b.manager.GetNumArgs(term)
		if e != nil {
			return e
		}
		if arity == 2 {
			left, e := b.manager.GetArg(term, 0)
			if e != nil {
				return e
			}
			text, _, matched, e := b.manager.ArithmeticNumeralText(left)
			if e != nil {
				return e
			}
			if matched {
				if e = b.store.manager.SetString(&coefficient, text); e != nil {
					return e
				}
				xTerm, e = b.manager.GetArg(term, 1)
				if e != nil {
					return e
				}
			}
		}
	}
	if xTerm == term {
		if err = b.store.manager.SetInt32(&coefficient, 1); err != nil {
			return err
		}
	}
	if err = b.coeff.Push(&coefficient); err != nil {
		return err
	}
	x, err := b.mkVar(xTerm)
	if err != nil {
		return err
	}
	b.ids = append(b.ids, x)
	return nil
}

// MkLinearPolynomial creates the source variable for an expression. An ADD
// also publishes the concrete equation sum(terms)-whole-expression=0.
func (b *PolynomialBuilder) MkLinearPolynomial(term ast.ASTHandle) (uint32, error) {
	if err := b.open(); err != nil {
		return 0, err
	}
	if x, found := b.expr2var[term]; found {
		return x, nil
	}
	x, err := b.mkVar(term)
	if err != nil {
		return 0, err
	}
	add, err := b.isArithmetic(term, 6)
	if err != nil || !add {
		return x, err
	}
	if err = b.coeff.Reset(); err != nil {
		return x, err
	}
	b.ids = b.ids[:0]
	arity, err := b.manager.GetNumArgs(term)
	if err != nil {
		return x, err
	}
	for i := uint32(0); i < arity; i++ {
		arg, e := b.manager.GetArg(term, i)
		if e != nil {
			return x, e
		}
		if e = b.pushTerm(arg); e != nil {
			return x, e
		}
	}
	minusOne := numeral.NewRational()
	if err = b.store.manager.SetInt32(&minusOne, -1); err != nil {
		return x, err
	}
	err = errors.Join(b.coeff.Push(&minusOne), b.store.manager.Del(&minusOne))
	if err != nil {
		return x, err
	}
	b.ids = append(b.ids, x)
	values, err := b.coeff.BorrowedValues()
	if err != nil {
		return x, err
	}
	_, err = b.store.MkRationalEquation(values, b.ids)
	return x, err
}

// ProcessBound is the source process(expr*) branch for equality and ordered
// arithmetic bounds. It consumes neither the expression nor a numeral owner;
// a recognized bound is published before returning true.
func (b *PolynomialBuilder) ProcessBound(term ast.ASTHandle) (processed bool, err error) {
	if err = b.open(); err != nil {
		return false, err
	}
	negated := false
	for {
		isNot, child, e := b.manager.IsNot(term)
		if e != nil {
			return false, e
		}
		if !isNot {
			break
		}
		negated = !negated
		term = child
	}
	kind, err := b.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := b.manager.GetFamilyID(term)
	if err != nil {
		return false, err
	}
	op, err := b.manager.GetDeclKind(term)
	if err != nil {
		return false, err
	}
	const (
		boundEqual = iota
		boundLessEqual
		boundGreaterEqual
	)
	bound := boundEqual
	strict := false
	switch {
	case family == b.manager.BasicFamilyID() && op == ast.ASTOpEq:
		if negated {
			return false, nil
		}
	case family == b.family && op == 2:
		bound = boundLessEqual
		if negated {
			bound, strict = boundGreaterEqual, true
		}
	case family == b.family && op == 3:
		bound = boundGreaterEqual
		if negated {
			bound, strict = boundLessEqual, true
		}
	default:
		return false, nil
	}
	lhs, err := b.manager.GetArg(term, 0)
	if err != nil {
		return false, err
	}
	rhs, err := b.manager.GetArg(term, 1)
	if err != nil {
		return false, err
	}
	if _, _, numeralLeft, e := b.manager.ArithmeticNumeralText(lhs); e != nil {
		return false, e
	} else if numeralLeft {
		lhs, rhs = rhs, lhs
		if bound == boundLessEqual {
			bound = boundGreaterEqual
		} else if bound == boundGreaterEqual {
			bound = boundLessEqual
		}
	}
	value, _, numeralRight, err := b.manager.ArithmeticNumeralText(rhs)
	if err != nil || !numeralRight {
		return false, err
	}
	x, err := b.MkLinearPolynomial(lhs)
	if err != nil {
		return false, err
	}
	k := numeral.NewRational()
	defer func() { err = errors.Join(err, b.store.manager.Del(&k)) }()
	if err = b.store.manager.SetString(&k, value); err != nil {
		return false, err
	}
	switch bound {
	case boundEqual:
		if _, err = b.store.AssertLower(x, &k, false); err != nil {
			return false, err
		}
		_, err = b.store.AssertUpper(x, &k, false)
	case boundLessEqual:
		_, err = b.store.AssertUpper(x, &k, strict)
	case boundGreaterEqual:
		_, err = b.store.AssertLower(x, &k, strict)
	}
	return err == nil, err
}

// CollectBounds scans the source goal once, retaining non-bounds in their
// original order in a fresh goal supplied by the caller. Both goals are
// borrowed; published bounds and the copied prefix survive a stopped call.
func (b *PolynomialBuilder) CollectBounds(input, saved *goaltactic.Goal) (found bool, err error) {
	if err = b.open(); err != nil {
		return false, err
	}
	if input == nil || saved == nil || input == saved {
		return false, arena.ErrInvalidSlot
	}
	if err = input.CheckOwner(); err != nil {
		return false, err
	}
	if err = saved.CheckOwner(); err != nil {
		return false, err
	}
	if input.Manager() != b.manager || saved.Manager() != b.manager {
		return false, ast.ErrASTHandle
	}
	if size, e := saved.Size(); e != nil || size != 0 {
		if e != nil {
			return false, e
		}
		return false, arena.ErrInvalidSlot
	}
	size, err := input.Size()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < size; i++ {
		formula, e := input.Form(i)
		if e != nil {
			return found, e
		}
		consumed, e := b.ProcessBound(formula)
		if e != nil {
			return found, e
		}
		if consumed {
			found = true
		} else if e = saved.Assert(formula); e != nil {
			return found, e
		}
	}
	return found, nil
}

// RestoreBounds emits the current source-order variable bounds after the
// caller has propagated the store. Exact equal non-strict bounds become one
// equality; ADD bounds implied by their terms are omitted.
func (b *PolynomialBuilder) RestoreBounds(saved *goaltactic.Goal) (err error) {
	if err = b.open(); err != nil {
		return err
	}
	if saved == nil {
		return arena.ErrInvalidSlot
	}
	if err = saved.CheckOwner(); err != nil {
		return err
	}
	if saved.Manager() != b.manager {
		return ast.ErrASTHandle
	}
	lower, upper := numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, b.store.manager.Del(&upper), b.store.manager.Del(&lower)) }()
	for i, term := range b.var2expr {
		foundLower, strictLower, _, e := b.store.Lower(uint32(i), &lower)
		if e != nil {
			return e
		}
		foundUpper, strictUpper, _, e := b.store.Upper(uint32(i), &upper)
		if e != nil {
			return e
		}
		if !foundLower && !foundUpper {
			continue
		}
		if foundLower && foundUpper && !strictLower && !strictUpper {
			cmp, e := b.store.manager.Compare(&lower, &upper)
			if e != nil {
				return e
			}
			if cmp == 0 {
				bound, e := b.boundNumeral(term, &lower)
				if e != nil {
					return e
				}
				formula, e := b.manager.MkEq(term, bound)
				if e != nil {
					return e
				}
				if e = saved.Assert(formula); e != nil {
					return e
				}
				continue
			}
		}
		if foundLower {
			subsumed, e := b.LowerSubsumed(term, &lower, strictLower)
			if e != nil {
				return e
			}
			if !subsumed {
				if e = b.emitBound(saved, term, &lower, true, strictLower); e != nil {
					return e
				}
			}
		}
		if foundUpper {
			subsumed, e := b.UpperSubsumed(term, &upper, strictUpper)
			if e != nil {
				return e
			}
			if !subsumed {
				if e = b.emitBound(saved, term, &upper, false, strictUpper); e != nil {
					return e
				}
			}
		}
	}
	return nil
}

func (b *PolynomialBuilder) boundNumeral(term ast.ASTHandle, value *numeral.Rational) (ast.ASTHandle, error) {
	sort, err := b.manager.GetSort(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := b.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	text, err := b.store.manager.String(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	service := b.manager.NumeralService()
	owned, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = service.SetString(&owned, text); err != nil {
		return ast.ASTHandle{}, errors.Join(err, service.Del(&owned))
	}
	h, err := b.manager.ArithmeticNumeral(&owned, kind == 1)
	return h, errors.Join(err, service.Del(&owned))
}

func (b *PolynomialBuilder) emitBound(saved *goaltactic.Goal, term ast.ASTHandle, value *numeral.Rational, lower, strict bool) error {
	bound, err := b.boundNumeral(term, value)
	if err != nil {
		return err
	}
	op := int32(2) // LE
	if lower != strict {
		op = 3 // GE
	}
	formula, err := b.manager.MkFamilyApp(b.family, op, nil, []ast.ASTHandle{term, bound})
	if err != nil {
		return err
	}
	if strict {
		formula, err = b.manager.MkNot(formula)
		if err != nil {
			return err
		}
	}
	return saved.Assert(formula)
}

// Apply executes the retained inequality pass. A newly allocated result is
// transferred to the caller, including on a stopped path. With no recognized
// bound, the source returns the original borrowed goal unchanged.
func (b *PolynomialBuilder) Apply(input *goaltactic.Goal) (_ *goaltactic.Goal, retErr error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	if input == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err := input.CheckOwner(); err != nil {
		return nil, err
	}
	if input.Manager() != b.manager {
		return nil, ast.ErrASTHandle
	}
	proofs, err := input.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	if proofs {
		return nil, goaltactic.NewTacticException("propagate-ineqs does not support proof production")
	}
	cores, err := input.UnsatCoresEnabled()
	if err != nil {
		return nil, err
	}
	if cores {
		return nil, goaltactic.NewTacticException("propagate-ineqs does not support unsat core production")
	}
	report, err := goaltactic.NewTacticReport("propagate-ineqs", input)
	if err != nil {
		return nil, err
	}
	defer func() { retErr = errors.Join(retErr, report.Close()) }()
	result, err := goaltactic.NewGoalConfigCopy(input)
	if err != nil {
		return nil, err
	}
	if err = result.BindInequalitySourceAllocation(); err != nil {
		return result, err
	}
	if err = result.IncDepth(); err != nil {
		return result, err
	}
	found, err := b.CollectBounds(input, result)
	if err != nil {
		return result, err
	}
	if !found {
		if err = result.Close(); err != nil {
			return result, err
		}
		return input, nil
	}
	if err = b.traceBeforePropagation(input); err != nil {
		return result, err
	}
	if err = b.store.Propagate(); err != nil {
		return result, err
	}
	propagations, falseAlarms, err := b.store.PropagationCounts()
	if err != nil {
		return result, err
	}
	if err = goaltactic.ReportTacticProgress(":bound-propagations", propagations); err != nil {
		return result, err
	}
	if err = goaltactic.ReportTacticProgress(":bound-false-alarms", falseAlarms); err != nil {
		return result, err
	}
	inconsistent, err := b.store.Inconsistent()
	if err != nil {
		return result, err
	}
	if inconsistent {
		if err = result.Reset(); err != nil {
			return result, err
		}
		falsehood, e := b.manager.MkFalse()
		if e != nil {
			return result, e
		}
		return result, result.Assert(falsehood)
	}
	if err = b.RestoreBounds(result); err != nil {
		return result, err
	}
	if err = b.traceAfterPropagation(); err != nil {
		return result, err
	}
	if err = b.traceResultGoal(result); err != nil {
		return result, err
	}
	return result, nil
}

// LowerSubsumed and UpperSubsumed are the concrete rational-buffer callers in
// propagate_ineqs_tactic.cpp:256-277. They reuse the same owned coefficient
// buffer as MkLinearPolynomial and borrow its current contents for one bound
// query; a non-addition has no polynomial subsumption path in the source.
func (b *PolynomialBuilder) LowerSubsumed(term ast.ASTHandle, k *numeral.Rational, strict bool) (bool, error) {
	return b.polynomialSubsumed(term, k, strict, true)
}

func (b *PolynomialBuilder) UpperSubsumed(term ast.ASTHandle, k *numeral.Rational, strict bool) (bool, error) {
	return b.polynomialSubsumed(term, k, strict, false)
}

func (b *PolynomialBuilder) polynomialSubsumed(term ast.ASTHandle, k *numeral.Rational, strict, lower bool) (subsumed bool, err error) {
	if err = b.open(); err != nil {
		return false, err
	}
	add, err := b.isArithmetic(term, 6)
	if err != nil || !add {
		return false, err
	}
	if err = b.coeff.Reset(); err != nil {
		return false, err
	}
	b.ids = b.ids[:0]
	arity, err := b.manager.GetNumArgs(term)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < arity; i++ {
		arg, e := b.manager.GetArg(term, i)
		if e != nil {
			return false, e
		}
		if e = b.pushTerm(arg); e != nil {
			return false, e
		}
	}
	values, err := b.coeff.BorrowedValues()
	if err != nil {
		return false, err
	}
	implied := numeral.NewRational()
	defer func() { err = errors.Join(err, b.store.manager.Del(&implied)) }()
	found, impliedStrict, err := b.store.PolynomialBound(values, b.ids, lower, &implied)
	if err != nil || !found {
		return false, err
	}
	cmp, err := b.store.manager.Compare(&implied, k)
	if err != nil {
		return false, err
	}
	if lower {
		return cmp > 0 || cmp == 0 && (!strict || impliedStrict), nil
	}
	return cmp < 0 || cmp == 0 && (!strict || impliedStrict), nil
}

func (b *PolynomialBuilder) Close() error {
	if b == nil || b.self != b {
		return arena.ErrInvalidSlot
	}
	if b.closed {
		return nil
	}
	if err := b.coeff.Close(); err != nil {
		return err
	}
	for i := len(b.refs) - 1; i >= 0; i-- {
		if err := b.manager.DelRef(&b.refs[i]); err != nil {
			return err
		}
	}
	b.refs, b.ids, b.var2expr, b.expr2var = nil, nil, nil, nil
	b.closed = true
	return nil
}
