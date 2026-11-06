package tactic

// Source: src/math/subpaving/tactic/expr2subpaving.cpp:84-111,286-363;
// src/math/subpaving/tactic/subpaving_tactic.cpp:128-190, concrete MPQ branch.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	goaltactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// RationalExprConverter owns variable and cached-expression AST references,
// and cached integer coefficients. Its AST manager, engine and numeral manager
// are borrowed.
type RationalExprConverter struct {
	self                                 *RationalExprConverter
	ast                                  *ast.ASTManager
	engine                               *RationalEngine
	trace                                *diagnostic.TraceTags
	qm                                   *numeral.RationalManager
	cooperation                          *cooperate.Cooperation
	family                               ast.FamilyID
	mapping                              *ast.Expr2Var
	ownsMapping                          bool
	order                                []ast.ASTHandle
	displayOrder                         []ast.ASTHandle
	cache                                map[ast.ASTHandle]cachedTerm
	cachedOrder                          []ast.ASTHandle
	cachedNumerators, cachedDenominators *numeral.ScopedIntVector
	closed                               bool
}

func NewRationalExprConverter(m *ast.ASTManager, engine *RationalEngine, qm *numeral.RationalManager) (*RationalExprConverter, error) {
	return NewRationalExprConverterWithMapping(m, engine, qm, nil)
}

// NewRationalExprConverterWithMapping borrows a supplied expr2var owner.
// A nil mapping creates a private mapping that the converter closes.
func NewRationalExprConverterWithMapping(m *ast.ASTManager, engine *RationalEngine, qm *numeral.RationalManager, mapping *ast.Expr2Var) (*RationalExprConverter, error) {
	if m == nil || engine == nil || qm == nil {
		return nil, arena.ErrInvalidSlot
	}
	if _, err := engine.Context(); err != nil {
		return nil, err
	}
	if engine.numbers != qm {
		return nil, arena.ErrConfig
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	ns, err := numeral.NewScopedIntVector(qm.Z)
	if err != nil {
		return nil, err
	}
	ds, err := numeral.NewScopedIntVector(qm.Z)
	if err != nil {
		_ = ns.Close()
		return nil, err
	}
	c := &RationalExprConverter{ast: m, engine: engine, qm: qm, family: family, cache: make(map[ast.ASTHandle]cachedTerm), cachedNumerators: ns, cachedDenominators: ds}
	c.ownsMapping = mapping == nil
	if mapping == nil {
		mapping, err = ast.NewExpr2Var(m)
	} else {
		var owner *ast.ASTManager
		owner, err = mapping.Manager()
		if err == nil && owner != m {
			err = ast.ErrASTHandle
		}
	}
	if err != nil {
		return nil, errors.Join(err, ds.Close(), ns.Close())
	}
	c.mapping = mapping
	c.self = c
	return c, nil
}

func (c *RationalExprConverter) check() error {
	if c == nil || c.self != c {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return arena.ErrClosed
	}
	_, err := c.engine.Context()
	return err
}

func (c *RationalExprConverter) BindCooperation(owner *cooperate.Cooperation) error {
	if err := c.check(); err != nil {
		return err
	}
	if owner == nil {
		return arena.ErrConfig
	}
	if _, err := owner.Enabled(); err != nil {
		return err
	}
	c.cooperation = owner
	return nil
}

// InternalizeTerm returns the source null_var for a numeral and an owned
// context variable for a leaf or a concrete sum/product definition.
func (c *RationalExprConverter) InternalizeTerm(t ast.ASTHandle, numerator, denominator *numeral.Int) (variable uint32, err error) {
	if err = c.check(); err != nil {
		return 0, err
	}
	if numerator == nil || denominator == nil {
		return 0, arena.ErrInvalidSlot
	}
	canceled, err := c.ast.Canceled()
	if err != nil {
		return 0, err
	}
	if canceled {
		return 0, ErrExprConverterCanceled
	}
	if c.cooperation != nil {
		if err := c.cooperation.Cooperate("expr2subpaving"); err != nil {
			return 0, err
		}
	}
	kind, err := c.ast.GetKind(t)
	if err != nil {
		return 0, err
	}
	if kind != ast.ASTApp && kind != ast.ASTVar {
		return 0, ast.ErrASTDomain
	}
	sort, err := c.ast.GetSort(t)
	if err != nil {
		return 0, err
	}
	family, err := c.ast.GetFamilyID(sort)
	if err != nil {
		return 0, err
	}
	sortKind, err := c.ast.GetDeclKind(sort)
	if err != nil {
		return 0, err
	}
	if family != c.family || sortKind != 0 && sortKind != 1 {
		return 0, ast.ErrASTDomain
	}
	if cached, ok, err := c.cachedTerm(t, numerator, denominator); err != nil || ok {
		return cached, err
	}
	if kind == ast.ASTApp {
		value, _, matched, err := c.ast.ArithmeticNumeralText(t)
		if err != nil {
			return 0, err
		}
		if matched {
			r := numeral.NewRational()
			defer func() { err = errors.Join(err, c.qm.Del(&r)) }()
			if err = c.qm.SetString(&r, value); err != nil {
				return 0, err
			}
			if err = c.qm.Numerator(&r, numerator); err != nil {
				return 0, err
			}
			if err = c.qm.Denominator(&r, denominator); err != nil {
				return 0, err
			}
			return math.MaxUint32, nil
		}
		declFamily, err := c.ast.GetFamilyID(t)
		if err != nil {
			return 0, err
		}
		if declFamily == c.family {
			operator, err := c.ast.GetDeclKind(t)
			if err != nil {
				return 0, err
			}
			switch operator {
			case 6: // OP_ADD
				return c.processAdd(t, numerator, denominator)
			case 9: // OP_MUL
				return c.processMul(t, numerator, denominator)
			case 18: // OP_POWER
				return c.processPower(t, numerator, denominator, sortKind == 1)
			case 14: // OP_TO_REAL
				arg, err := c.ast.GetArg(t, 0)
				if err != nil {
					return 0, err
				}
				return c.InternalizeTerm(arg, numerator, denominator)
			case 7, 8: // OP_SUB, OP_UMINUS
				return 0, ErrExprConverterNonSimplified
			case 1, 10, 11, 12, 13, 15:
				return 0, ErrExprConverterPurifier
			case 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30:
				return 0, ErrExprConverterTranscendental
			default:
				return 0, ErrExprConverterUnsupported
			}
		}
	}
	return c.mkVarFor(t, sortKind == 1, numerator, denominator)
}

func (c *RationalExprConverter) mkVarFor(t ast.ASTHandle, integer bool, numerator, denominator *numeral.Int) (uint32, error) {
	if err := c.qm.Z.SetInt32(numerator, 1); err != nil {
		return 0, err
	}
	if err := c.qm.Z.SetInt32(denominator, 1); err != nil {
		return 0, err
	}
	variable, err := c.mapping.ToVar(t)
	if err != nil || variable != math.MaxUint32 {
		return variable, err
	}
	context, err := c.engine.Context()
	if err != nil {
		return 0, err
	}
	variable, err = context.NewVariable(integer)
	if err != nil {
		return 0, err
	}
	if err := c.mapping.Insert(t, variable); err != nil {
		return 0, err
	}
	// m_var2expr owns a second reference, independently of expr2var.
	if err := c.ast.IncRef(t); err != nil {
		return 0, err
	}
	c.order = append(c.order, t)
	return variable, nil
}

func (c *RationalExprConverter) cachedTerm(t ast.ASTHandle, n, d *numeral.Int) (uint32, bool, error) {
	rc, err := c.ast.GetRefCount(t)
	if err != nil || rc <= 1 {
		return 0, false, err
	}
	entry, ok := c.cache[t]
	if !ok {
		return 0, false, nil
	}
	if err := c.cachedNumerators.Read(entry.index, n); err != nil {
		return 0, true, err
	}
	if err := c.cachedDenominators.Read(entry.index, d); err != nil {
		return 0, true, err
	}
	return entry.variable, true, nil
}

func (c *RationalExprConverter) cacheResult(t ast.ASTHandle, variable uint32, n, d *numeral.Int) (err error) {
	rc, err := c.ast.GetRefCount(t)
	if err != nil || rc <= 1 {
		return err
	}
	if _, exists := c.cache[t]; exists {
		return ast.ErrASTDomain
	}
	index := c.cachedNumerators.Size()
	if err := c.ast.IncRef(t); err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		err = errors.Join(err, c.cachedDenominators.Shrink(index), c.cachedNumerators.Shrink(index))
		_, e := c.ast.DecRefConsumed(t)
		err = errors.Join(err, e)
	}()
	if err = c.cachedNumerators.Push(n); err != nil {
		return err
	}
	if err = c.cachedDenominators.Push(d); err != nil {
		return err
	}
	c.cache[t] = cachedTerm{index: index, variable: variable}
	c.cachedOrder = append(c.cachedOrder, t)
	return nil
}

// InternalizeInequality follows mk_ineq's source order: strip negations,
// parse the RHS, internalize the LHS, scale the threshold, then choose sign.
func (c *RationalExprConverter) InternalizeInequality(h ast.ASTHandle) (atom *subpaving.RationalInequality, err error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	neg := false
	for {
		yes, child, e := c.ast.IsNot(h)
		if e != nil {
			return nil, e
		}
		if !yes {
			break
		}
		neg = !neg
		h = child
	}
	kind, err := c.ast.GetKind(h)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTApp {
		return nil, ErrFastUnsupportedAtom
	}
	family, err := c.ast.GetFamilyID(h)
	if err != nil {
		return nil, err
	}
	op, err := c.ast.GetDeclKind(h)
	if err != nil {
		return nil, err
	}
	if family != c.family || op != 2 && op != 3 {
		return nil, ErrFastUnsupportedAtom
	}
	lower, open := op == 3, false
	if neg {
		lower, open = !lower, true
	}
	rhs, err := c.ast.GetArg(h, 1)
	if err != nil {
		return nil, err
	}
	value, _, matched, err := c.ast.ArithmeticNumeralText(rhs)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ErrFastArithmeticLHS
	}
	k := numeral.NewRational()
	var n, d numeral.Int
	defer func() { err = errors.Join(err, c.qm.Z.Del(&d), c.qm.Z.Del(&n), c.qm.Del(&k)) }()
	if err = c.qm.SetString(&k, value); err != nil {
		return nil, err
	}
	lhs, err := c.ast.GetArg(h, 0)
	if err != nil {
		return nil, err
	}
	x, err := c.InternalizeTerm(lhs, &n, &d)
	if err != nil {
		return nil, err
	}
	if err = c.qm.MulInt(&d, &k, &k); err != nil {
		return nil, err
	}
	if err = c.qm.DivInt(&k, &n, &k); err != nil {
		return nil, err
	}
	sign, err := c.qm.Z.Sign(&n)
	if err != nil {
		return nil, err
	}
	if sign < 0 {
		lower = !lower
	}
	if err = emitGoalInequalityTrace(c.trace, c.qm, x, &k, lower, open); err != nil {
		return nil, err
	}
	shell, err := c.engine.Shell()
	if err != nil {
		return nil, err
	}
	return shell.MkInequality(x, &k, lower, open)
}

func (c *RationalExprConverter) InternalizeClause(h ast.ASTHandle) (err error) {
	if err := c.check(); err != nil {
		return err
	}
	args := []ast.ASTHandle{h}
	kind, err := c.ast.GetKind(h)
	if err != nil {
		return err
	}
	if kind == ast.ASTApp {
		family, err := c.ast.GetFamilyID(h)
		if err != nil {
			return err
		}
		op, err := c.ast.GetDeclKind(h)
		if err != nil {
			return err
		}
		if family == c.ast.BasicFamilyID() && op == ast.ASTOpOr {
			count, err := c.ast.GetNumArgs(h)
			if err != nil {
				return err
			}
			args = make([]ast.ASTHandle, count)
			for i := range args {
				args[i], err = c.ast.GetArg(h, uint32(i))
				if err != nil {
					return err
				}
			}
		}
	}
	atoms := make([]*subpaving.RationalInequality, 0, len(args))
	defer func() {
		for _, atom := range atoms {
			if cleanup := c.engine.context.DiscardUnreferencedInequality(atom); cleanup != nil {
				err = errors.Join(err, cleanup)
				break
			}
		}
	}()
	for _, literal := range args {
		atom, err := c.InternalizeInequality(literal)
		if err != nil {
			return err
		}
		atoms = append(atoms, atom)
	}
	context, err := c.engine.Context()
	if err != nil {
		return err
	}
	_, err = context.AddClause(atoms)
	return err
}

func (c *RationalExprConverter) InternalizeGoal(g *goaltactic.Goal) error {
	if err := c.check(); err != nil {
		return err
	}
	if g == nil {
		return arena.ErrInvalidSlot
	}
	count, err := g.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		formula, err := g.Form(i)
		if err != nil {
			return err
		}
		if err := c.InternalizeClause(formula); err != nil {
			return err
		}
	}
	return nil
}

func (c *RationalExprConverter) SearchGoal(g *goaltactic.Goal) error {
	if err := c.InternalizeGoal(g); err != nil {
		return err
	}
	context, err := c.engine.Context()
	if err != nil {
		return err
	}
	return context.Search()
}

func (c *RationalExprConverter) IsVar(t ast.ASTHandle) (bool, error) {
	if err := c.check(); err != nil {
		return false, err
	}
	return c.mapping.IsVar(t)
}

func (c *RationalExprConverter) Close() error {
	if c == nil || c.self != c {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return nil
	}
	for _, h := range c.cachedOrder {
		if _, ok := c.cache[h]; !ok {
			continue
		}
		consumed, err := c.ast.DecRefConsumed(h)
		if consumed {
			delete(c.cache, h)
		}
		if err != nil {
			return err
		}
	}
	if err := c.cachedNumerators.Close(); err != nil {
		return err
	}
	if err := c.cachedDenominators.Close(); err != nil {
		return err
	}
	if c.ownsMapping {
		if err := c.mapping.Close(); err != nil {
			return err
		}
	}
	if err := releaseExprPins(c.ast, &c.displayOrder); err != nil {
		return err
	}
	for i, h := range c.order {
		if h.IsNull() {
			continue
		}
		consumed, err := c.ast.DecRefConsumed(h)
		if consumed {
			c.order[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
	}
	c.closed = true
	return nil
}
