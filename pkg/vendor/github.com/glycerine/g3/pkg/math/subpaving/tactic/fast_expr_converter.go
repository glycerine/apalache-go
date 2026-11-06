package tactic

// Source: src/math/subpaving/tactic/expr2subpaving.cpp:84-286, 332-363; concrete MPFF instantiation.

import (
	"errors"
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// FastExprConverter owns the source expr2var references created for
// arithmetic variables. Its AST manager, MPFF engine, and rational manager are borrowed.
type FastExprConverter struct {
	self                                 *FastExprConverter
	ast                                  *ast.ASTManager
	engine                               *FastEngine
	trace                                *diagnostic.TraceTags
	cooperation                          *cooperate.Cooperation
	qm                                   *numeral.RationalManager
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

func NewFastExprConverter(m *ast.ASTManager, engine *FastEngine, qm *numeral.RationalManager) (*FastExprConverter, error) {
	return NewFastExprConverterWithMapping(m, engine, qm, nil)
}

// NewFastExprConverterWithMapping borrows a supplied expr2var owner.
// A nil mapping creates a private mapping that the converter closes.
func NewFastExprConverterWithMapping(m *ast.ASTManager, engine *FastEngine, qm *numeral.RationalManager, mapping *ast.Expr2Var) (*FastExprConverter, error) {
	if m == nil || engine == nil || qm == nil {
		return nil, arena.ErrInvalidSlot
	}
	if _, err := engine.Shell(); err != nil {
		return nil, err
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
	c := &FastExprConverter{ast: m, engine: engine, qm: qm, family: family, cache: make(map[ast.ASTHandle]cachedTerm), cachedNumerators: ns, cachedDenominators: ds}
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

func (c *FastExprConverter) check() error {
	if c == nil || c.self != c {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return arena.ErrClosed
	}
	_, err := c.engine.Shell()
	return err
}

// BindCooperation borrows the source process owner for expression checkpoints.
func (c *FastExprConverter) BindCooperation(owner *cooperate.Cooperation) error {
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

// InternalizeTerm implements numeral and variable leaves, to_real, add, mul,
// and positive integral powers.
// The caller owns numerator and denominator.
// math.MaxUint32 is the source null_var value for constant terms.
func (c *FastExprConverter) InternalizeTerm(t ast.ASTHandle, numerator, denominator *numeral.Int) (uint32, error) {
	return c.process(t, numerator, denominator)
}

func (c *FastExprConverter) process(t ast.ASTHandle, numerator, denominator *numeral.Int) (variable uint32, err error) {
	if err := c.check(); err != nil {
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
	if family != c.family || (sortKind != 0 && sortKind != 1) {
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
			if err := c.qm.SetString(&r, value); err != nil {
				return 0, err
			}
			if err := c.qm.Numerator(&r, numerator); err != nil {
				return 0, err
			}
			if err := c.qm.Denominator(&r, denominator); err != nil {
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
				return c.process(arg, numerator, denominator)
			case 7, 8: // OP_SUB, OP_UMINUS
				return 0, ErrExprConverterNonSimplified
			case 1, 10, 11, 12, 13, 15: // irrational numeral, div/idiv/rem/mod/to_int
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

func (c *FastExprConverter) cachedTerm(t ast.ASTHandle, n, d *numeral.Int) (uint32, bool, error) {
	rc, err := c.ast.GetRefCount(t)
	if err != nil {
		return 0, false, err
	}
	if rc <= 1 {
		return 0, false, nil
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

// cacheResult follows the source's ref-count threshold. Publishing both deep
// copies and the retained AST key happens only after the entry is complete.
func (c *FastExprConverter) cacheResult(t ast.ASTHandle, variable uint32, n, d *numeral.Int) (err error) {
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

func (c *FastExprConverter) mkVarFor(t ast.ASTHandle, integer bool, numerator, denominator *numeral.Int) (uint32, error) {
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
	shell, err := c.engine.Shell()
	if err != nil {
		return 0, err
	}
	variable, err = shell.MkVar(integer)
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

// unsignedExponent follows rational::is_unsigned and get_unsigned, including
// the full uint32 range. The AST's detached decimal/fraction text is exact.
func (c *FastExprConverter) unsignedExponent(h ast.ASTHandle) (uint32, bool, error) {
	value, _, matched, err := c.ast.ArithmeticNumeralText(h)
	if err != nil || !matched {
		return 0, false, err
	}
	r, ok := new(big.Rat).SetString(value)
	if !ok || !r.IsInt() || r.Sign() < 0 || r.Num().BitLen() > 32 {
		return 0, false, nil
	}
	return uint32(r.Num().Uint64()), true, nil
}

func (c *FastExprConverter) asPower(h ast.ASTHandle) (ast.ASTHandle, uint32, error) {
	kind, err := c.ast.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return h, 1, err
	}
	family, err := c.ast.GetFamilyID(h)
	if err != nil || family != c.family {
		return h, 1, err
	}
	op, err := c.ast.GetDeclKind(h)
	if err != nil || op != 18 {
		return h, 1, err
	}
	exponent, err := c.ast.GetArg(h, 1)
	if err != nil {
		return h, 1, err
	}
	k, yes, err := c.unsignedExponent(exponent)
	if err != nil || !yes {
		return h, 1, err
	}
	base, err := c.ast.GetArg(h, 0)
	return base, k, err
}

func (c *FastExprConverter) processMul(t ast.ASTHandle, n, d *numeral.Int) (variable uint32, err error) {
	z := c.qm.Z
	count, err := c.ast.GetNumArgs(t)
	if err != nil {
		return 0, err
	}
	if count <= 1 {
		return 0, ErrExprConverterNonSimplified
	}
	first, err := c.ast.GetArg(t, 0)
	if err != nil {
		return 0, err
	}
	coefficientText, _, numeric, err := c.ast.ArithmeticNumeralText(first)
	if err != nil {
		return 0, err
	}
	m := t
	hasCoefficient := false
	_, parsed := new(big.Rat).SetString(coefficientText)
	if numeric && parsed {
		if count != 2 {
			return 0, ErrExprConverterNonSimplified
		}
		if err = copyExprCoefficient(c.qm, coefficientText, n, d); err != nil {
			return 0, err
		}
		m, err = c.ast.GetArg(t, 1)
		if err != nil {
			return 0, err
		}
		hasCoefficient = true
	}
	if !hasCoefficient {
		if err = z.SetInt32(n, 1); err != nil {
			return 0, err
		}
		if err = z.SetInt32(d, 1); err != nil {
			return 0, err
		}
	}
	args := []ast.ASTHandle{m}
	kind, e := c.ast.GetKind(m)
	if e != nil {
		return 0, e
	}
	if kind == ast.ASTApp {
		family, e := c.ast.GetFamilyID(m)
		if e != nil {
			return 0, e
		}
		operator, e := c.ast.GetDeclKind(m)
		if e != nil {
			return 0, e
		}
		if family == c.family && operator == 9 {
			length, e := c.ast.GetNumArgs(m)
			if e != nil {
				return 0, e
			}
			args = make([]ast.ASTHandle, length)
			for i := range args {
				args[i], e = c.ast.GetArg(m, uint32(i))
				if e != nil {
					return 0, e
				}
			}
		}
	}
	var argN, argD numeral.Int
	defer func() { err = errors.Join(err, z.Del(&argD), z.Del(&argN)) }()
	powers := make([]subpaving.FastPower, 0, len(args))
	for _, arg := range args {
		base, degree, e := c.asPower(arg)
		if e != nil {
			return 0, e
		}
		x, e := c.process(base, &argN, &argD)
		if e != nil {
			return 0, e
		}
		if e = z.Power(&argN, degree, &argN); e != nil {
			return 0, e
		}
		if e = z.Power(&argD, degree, &argD); e != nil {
			return 0, e
		}
		if e = z.Mul(n, &argN, n); e != nil {
			return 0, e
		}
		if e = z.Mul(d, &argD, d); e != nil {
			return 0, e
		}
		if x != math.MaxUint32 {
			powers = append(powers, subpaving.FastPower{Variable: x, Degree: degree})
		}
	}
	if len(powers) == 0 {
		variable = math.MaxUint32
		return variable, c.cacheResult(t, variable, n, d)
	}
	if len(powers) == 1 && powers[0].Degree == 1 {
		variable = powers[0].Variable
		return variable, c.cacheResult(t, variable, n, d)
	}
	shell, err := c.engine.Shell()
	if err != nil {
		return 0, err
	}
	variable, err = shell.MkMonomial(powers)
	if err != nil {
		return 0, err
	}
	return variable, c.cacheResult(t, variable, n, d)
}

func (c *FastExprConverter) processPower(t ast.ASTHandle, n, d *numeral.Int, integer bool) (variable uint32, err error) {
	z := c.qm.Z
	exponent, err := c.ast.GetArg(t, 1)
	if err != nil {
		return 0, err
	}
	k, yes, err := c.unsignedExponent(exponent)
	if err != nil {
		return 0, err
	}
	if !yes {
		return c.mkVarFor(t, integer, n, d)
	}
	base, err := c.ast.GetArg(t, 0)
	if err != nil {
		return 0, err
	}
	x, err := c.process(base, n, d)
	if err != nil {
		return 0, err
	}
	if x != math.MaxUint32 {
		shell, e := c.engine.Shell()
		if e != nil {
			return 0, e
		}
		x, err = shell.MkMonomial([]subpaving.FastPower{{Variable: x, Degree: k}})
		if err != nil {
			return 0, err
		}
	}
	if err = z.Power(n, k, n); err != nil {
		return 0, err
	}
	if err = z.Power(d, k, d); err != nil {
		return 0, err
	}
	return x, c.cacheResult(t, x, n, d)
}

// processAdd uses the two live unsynch_mpz_manager buffer instantiations from
// src/util/scoped_numeral_buffer.h. It computes the common denominator before
// handing exact integer coefficients to the MPFF wrapper.
func (c *FastExprConverter) processAdd(t ast.ASTHandle, n, d *numeral.Int) (variable uint32, err error) {
	qm := c.qm
	z := qm.Z
	var ns, ds numeral.ScopedIntegerBuffer16
	if err = ns.Construct(z); err != nil {
		return 0, err
	}
	if err = ds.Construct(z); err != nil {
		_ = ns.Cleanup()
		return 0, err
	}
	constant, argConstant := numeral.NewRational(), numeral.NewRational()
	var argN, argD, factor, sumConstant numeral.Int
	defer func() {
		// A stopped buffer destructor retains its roots for explicit cleanup.
		for _, b := range []*numeral.ScopedIntegerBuffer16{&ds, &ns} {
			e := b.SourceClose()
			err = errors.Join(err, e)
			if err != nil {
				err = errors.Join(err, b.Cleanup())
			}
		}
		err = errors.Join(err, z.Del(&sumConstant), z.Del(&factor), z.Del(&argD), z.Del(&argN), qm.Del(&argConstant), qm.Del(&constant))
	}()
	count, err := c.ast.GetNumArgs(t)
	if err != nil {
		return 0, err
	}
	vars := make([]uint32, 0, count)
	for i := uint32(0); i < count; i++ {
		arg, e := c.ast.GetArg(t, i)
		if e != nil {
			return 0, e
		}
		x, e := c.process(arg, &argN, &argD)
		if e != nil {
			return 0, e
		}
		if x == math.MaxUint32 {
			if e = qm.SetParts(&argConstant, &argN, &argD); e != nil {
				return 0, e
			}
			if e = qm.Add(&constant, &argConstant, &constant); e != nil {
				return 0, e
			}
		} else {
			vars = append(vars, x)
			if e = ns.Push(&argN); e != nil {
				return 0, e
			}
			if e = ds.Push(&argD); e != nil {
				return 0, e
			}
		}
	}
	if err = qm.Denominator(&constant, d); err != nil {
		return 0, err
	}
	for i := range vars {
		den, e := ds.At(uint32(i))
		if e != nil {
			return 0, e
		}
		if e = z.LCM(d, den, d); e != nil {
			return 0, e
		}
	}
	if err = z.Div(d, &constant.Den, &factor); err != nil {
		return 0, err
	}
	if err = z.Mul(&constant.Num, &factor, &sumConstant); err != nil {
		return 0, err
	}
	for i := range vars {
		den, e := ds.At(uint32(i))
		if e != nil {
			return 0, e
		}
		if e = z.Div(d, den, &factor); e != nil {
			return 0, e
		}
		if e = ns.MultiplyAt(uint32(i), &factor); e != nil {
			return 0, e
		}
	}
	if len(vars) == 0 {
		if err = z.Copy(n, &sumConstant); err != nil {
			return 0, err
		}
		return math.MaxUint32, c.cacheResult(t, math.MaxUint32, n, d)
	}
	coefficients := make([]*numeral.Int, len(vars))
	for i := range coefficients {
		coefficients[i], err = ns.At(uint32(i))
		if err != nil {
			return 0, err
		}
	}
	shell, err := c.engine.Shell()
	if err != nil {
		return 0, err
	}
	variable, err = shell.MkSum(&sumConstant, coefficients, vars)
	if err != nil {
		return 0, err
	}
	if err = z.SetInt32(n, 1); err != nil {
		return 0, err
	}
	return variable, c.cacheResult(t, variable, n, d)
}

func (c *FastExprConverter) IsVar(t ast.ASTHandle) (bool, error) {
	if err := c.check(); err != nil {
		return false, err
	}
	return c.mapping.IsVar(t)
}

func (c *FastExprConverter) Close() error {
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
