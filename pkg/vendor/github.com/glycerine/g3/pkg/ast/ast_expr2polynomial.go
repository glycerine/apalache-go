package ast

// Source: src/ast/expr2polynomial.cpp:25-518; src/ast/expr2polynomial.h:29-110.

import (
	"errors"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var ErrExprNotPolynomial = errors.New("the given expression is not a polynomial")
var ErrExprPolynomialIntegerVariable = errors.New("integer variables are not allowed in the given polynomial")
var ErrExprPolynomialCanceled = errors.New("canceled")

type Expr2PolynomialConfig struct {
	AST           *ASTManager
	Polynomial    *polynomial.MultivariatePolynomialManager
	Mapping       *Expr2Var
	Variables     ExprPolynomialVariables
	UseVarIndices bool
}

// ExprPolynomialVariables is the concrete typed virtual surface used by the
// retained nlsat specialization. The default converter leaves it nil and uses
// the polynomial manager directly.
type ExprPolynomialVariables interface {
	MkExprPolynomialVariable(integer bool) (uint32, error)
	IsExprPolynomialInteger(variable uint32) (bool, error)
}

type exprPolynomialValue struct {
	polynomial  *polynomial.PolynomialRef
	denominator *unumeral.ScopedInt
}

func (v *exprPolynomialValue) close() error {
	if v == nil {
		return nil
	}
	var err error
	if v.polynomial != nil {
		err = errors.Join(err, v.polynomial.Close())
		v.polynomial = nil
	}
	if v.denominator != nil {
		err = errors.Join(err, v.denominator.Close())
		v.denominator = nil
	}
	return err
}

type exprPolynomialCacheEntry struct {
	handle ASTHandle
	domain ASTRef
	value  exprPolynomialValue
}

type Expr2Polynomial struct {
	self          *Expr2Polynomial
	ast           *ASTManager
	manager       *polynomial.MultivariatePolynomialManager
	core          *polynomial.MultivariatePolynomialCore
	integers      *unumeral.IntegerManager
	monomials     *polynomial.MonomialManager
	mapping       *Expr2Var
	variables     ExprPolynomialVariables
	ownMapping    bool
	useVarIndices bool
	var2expr      []ASTRef
	varIsInt      []bool
	cache         map[ASTHandle]int
	cached        []exprPolynomialCacheEntry
	canceled      bool
	closed        bool
}

func NewExpr2Polynomial(c Expr2PolynomialConfig) (*Expr2Polynomial, error) {
	if c.AST == nil || c.Polynomial == nil {
		return nil, arena.ErrConfig
	}
	if err := c.AST.lock(); err != nil {
		return nil, err
	}
	c.AST.mu.Unlock()
	core, err := c.Polynomial.ArithmeticCore()
	if err != nil {
		return nil, err
	}
	integers, err := core.IntegerManager()
	if err != nil {
		return nil, err
	}
	monomials, err := core.MonomialManager()
	if err != nil {
		return nil, err
	}
	x := &Expr2Polynomial{ast: c.AST, manager: c.Polynomial, core: core, integers: integers, monomials: monomials, variables: c.Variables, useVarIndices: c.UseVarIndices, cache: make(map[ASTHandle]int)}
	x.self = x
	if !c.UseVarIndices {
		x.mapping = c.Mapping
		if x.mapping == nil {
			x.mapping, err = NewExpr2Var(c.AST)
			x.ownMapping = err == nil
		} else if owner, e := x.mapping.Manager(); e != nil || owner != c.AST {
			if e != nil {
				err = e
			} else {
				err = arena.ErrConfig
			}
		}
	}
	if err != nil {
		if x.ownMapping {
			_ = x.mapping.Close()
		}
		return nil, err
	}
	return x, nil
}

func (x *Expr2Polynomial) open() error {
	if x == nil || x.self != x || x.closed || x.ast == nil || x.core == nil {
		return ErrASTHandle
	}
	if x.ast.Closed {
		return arena.ErrClosed
	}
	_, err := x.core.IntegerManager()
	return err
}

func (x *Expr2Polynomial) ASTManager() (*ASTManager, error) {
	if err := x.open(); err != nil {
		return nil, err
	}
	return x.ast, nil
}

func (x *Expr2Polynomial) PolynomialManager() (*polynomial.MultivariatePolynomialManager, error) {
	if err := x.open(); err != nil {
		return nil, err
	}
	return x.manager, nil
}

func (x *Expr2Polynomial) SetCancel(cancel bool) error {
	if err := x.open(); err != nil {
		return err
	}
	x.canceled = cancel
	return nil
}

func (x *Expr2Polynomial) checkpoint() error {
	if x.canceled {
		return ErrExprPolynomialCanceled
	}
	canceled, err := x.ast.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return ErrExprPolynomialCanceled
	}
	return nil
}

func (x *Expr2Polynomial) isIntReal(h ASTHandle) (isInt, matched bool, err error) {
	sort, err := x.ast.GetSort(h)
	if err != nil {
		return false, false, err
	}
	if x.ast.arithmeticPlugin == nil {
		return false, false, nil
	}
	if sort == x.ast.external(x.ast.arithmeticPlugin.fields[1]) {
		return true, true, nil
	}
	return false, sort == x.ast.external(x.ast.arithmeticPlugin.fields[0]), nil
}

func (x *Expr2Polynomial) newValue(p polynomial.Polynomial, d *unumeral.Int) (v *exprPolynomialValue, err error) {
	v = &exprPolynomialValue{}
	defer func() {
		if err != nil {
			err = errors.Join(err, v.close())
			v = nil
		}
	}()
	v.polynomial, err = polynomial.NewPolynomialRef(x.core)
	if err != nil {
		return nil, err
	}
	if err = v.polynomial.Assign(p); err != nil {
		return nil, err
	}
	v.denominator, err = unumeral.NewScopedInt(x.integers)
	if err != nil {
		return nil, err
	}
	if d == nil {
		err = v.denominator.SetInt64(1)
	} else {
		err = v.denominator.Set(d)
	}
	return v, err
}

func (x *Expr2Polynomial) cloneValue(source *exprPolynomialValue) (v *exprPolynomialValue, err error) {
	p, err := source.polynomial.Get()
	if err != nil {
		return nil, err
	}
	d, err := source.denominator.Get()
	if err != nil {
		return nil, err
	}
	return x.newValue(p, d)
}

func (x *Expr2Polynomial) cacheValue(h ASTHandle, v *exprPolynomialValue) error {
	refs, err := x.ast.GetRefCount(h)
	if err != nil || refs <= 1 {
		return err
	}
	if _, exists := x.cache[h]; exists {
		return ErrASTDomain
	}
	copyValue, err := x.cloneValue(v)
	if err != nil {
		return err
	}
	domain, err := x.ast.NewRef(h)
	if err != nil {
		return errors.Join(err, copyValue.close())
	}
	x.cache[h] = len(x.cached)
	x.cached = append(x.cached, exprPolynomialCacheEntry{handle: h, domain: domain, value: *copyValue})
	return nil
}

func (x *Expr2Polynomial) publishCache(h ASTHandle, v *exprPolynomialValue) error {
	if err := x.cacheValue(h, v); err != nil {
		return errors.Join(err, v.close())
	}
	return nil
}

func (x *Expr2Polynomial) variableValue(h ASTHandle, integer bool) (*exprPolynomialValue, error) {
	var variable uint32
	if x.useVarIndices {
		if integer {
			return nil, ErrExprPolynomialIntegerVariable
		}
		kind, err := x.ast.GetKind(h)
		if err != nil {
			return nil, err
		}
		if kind != ASTVar {
			return nil, ErrExprNotPolynomial
		}
		variable, err = x.ast.GetVarIndex(h)
		if err != nil {
			return nil, err
		}
		for {
			n, err := x.core.NumVars()
			if err != nil {
				return nil, err
			}
			if n > variable {
				break
			}
			if _, err = x.core.MkVar(); err != nil {
				return nil, err
			}
		}
	} else {
		var err error
		variable, err = x.mapping.ToVar(h)
		if err != nil {
			return nil, err
		}
		if variable == math.MaxUint32 {
			if x.variables == nil {
				variable, err = x.core.MkVar()
			} else {
				variable, err = x.variables.MkExprPolynomialVariable(integer)
			}
			if err != nil {
				return nil, err
			}
			if err = x.mapping.Insert(h, variable); err != nil {
				return nil, err
			}
		}
		for uint64(len(x.var2expr)) <= uint64(variable) {
			x.var2expr = append(x.var2expr, ASTRef{})
			x.varIsInt = append(x.varIsInt, false)
		}
		if x.var2expr[variable] == (ASTRef{}) {
			x.var2expr[variable], err = x.ast.NewRef(h)
			if err != nil {
				return nil, err
			}
			x.varIsInt[variable] = integer
		}
	}
	p, err := x.core.MkVariable(variable, 1)
	if err != nil {
		return nil, err
	}
	return x.newValue(p, nil)
}

func (x *Expr2Polynomial) numeralValue(h ASTHandle) (*exprPolynomialValue, error) {
	text, _, matched, err := x.ast.ArithmeticNumeralText(h)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ErrASTDomain
	}
	numerator, denominator := text, "1"
	if left, right, found := strings.Cut(text, "/"); found {
		numerator, denominator = left, right
	}
	n := unumeral.NewInt(0)
	if err = x.integers.SetString(&n, numerator); err != nil {
		return nil, err
	}
	defer func() { _ = x.integers.Del(&n) }()
	d := unumeral.NewInt(0)
	if err = x.integers.SetString(&d, denominator); err != nil {
		return nil, err
	}
	defer func() { _ = x.integers.Del(&d) }()
	p, err := x.core.MkConstantTakeCopy(&n)
	if err != nil {
		return nil, err
	}
	return x.newValue(p, &d)
}

func (x *Expr2Polynomial) visit(h ASTHandle) (value *exprPolynomialValue, op int32, frame bool, err error) {
	if index, ok := x.cache[h]; ok {
		value, err = x.cloneValue(&x.cached[index].value)
		return
	}
	integer, matched, err := x.isIntReal(h)
	if err != nil || !matched {
		return nil, 0, false, err
	}
	kind, err := x.ast.GetKind(h)
	if err != nil {
		return nil, 0, false, err
	}
	if kind == ASTVar {
		v, err := x.variableValue(h, integer)
		if err == nil {
			err = x.publishCache(h, v)
		}
		return v, 0, false, err
	}
	if kind != ASTApp {
		return nil, 0, false, ErrExprNotPolynomial
	}
	family, err := x.ast.GetFamilyID(h)
	if err != nil {
		return nil, 0, false, err
	}
	if x.ast.arithmeticPlugin == nil || family != x.ast.arithmeticPlugin.family {
		if x.useVarIndices {
			return nil, 0, false, ErrExprNotPolynomial
		}
		v, err := x.variableValue(h, integer)
		if err == nil {
			err = x.publishCache(h, v)
		}
		return v, 0, false, err
	}
	op, err = x.ast.GetDeclKind(h)
	if err != nil {
		return nil, 0, false, err
	}
	if op == 0 {
		v, err := x.numeralValue(h)
		if err == nil {
			err = x.publishCache(h, v)
		}
		return v, 0, false, err
	}
	if op != 6 && op != 7 && op != 8 && op != 9 && op != 14 && op != 18 {
		if x.useVarIndices {
			return nil, 0, false, ErrExprNotPolynomial
		}
		v, err := x.variableValue(h, integer)
		if err == nil {
			err = x.publishCache(h, v)
		}
		return v, 0, false, err
	}
	if op == 18 {
		exponent, e := x.ast.GetArg(h, 1)
		if e != nil {
			return nil, 0, false, e
		}
		text, _, numeral, e := x.ast.ArithmeticNumeralText(exponent)
		valid := e == nil && numeral && !strings.Contains(text, "/") && !strings.HasPrefix(text, "-")
		if valid {
			_, e = strconv.ParseUint(text, 10, 32)
			valid = e == nil
		}
		if !valid {
			if e != nil && numeral {
				return nil, 0, false, e
			}
			if x.useVarIndices {
				return nil, 0, false, ErrExprNotPolynomial
			}
			v, e := x.variableValue(h, integer)
			if e == nil {
				e = x.publishCache(h, v)
			}
			return v, 0, false, e
		}
	}
	return nil, op, true, nil
}

type exprPolynomialFrame struct {
	expression ASTHandle
	op         int32
	next       uint32
	count      uint32
	args       []*exprPolynomialValue
}

func closeExprPolynomialFrames(frames []exprPolynomialFrame) error {
	var err error
	for i := range frames {
		for _, value := range frames[i].args {
			err = errors.Join(err, value.close())
		}
	}
	return err
}

func (x *Expr2Polynomial) traceFrame(frame *exprPolynomialFrame) error {
	tags := x.ast.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("expr2polynomial") {
		return nil
	}
	return tags.EmitConditional("expr2polynomial", "to_polynomial", "src/ast/expr2polynomial.cpp", 372,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "processing: "+strconv.FormatUint(uint64(frame.next), 10)+"\n"); err != nil {
				return err
			}
			g := uparams.BorrowProcessGlobalParamsService()
			if g == nil || g.Context == nil {
				return uparams.ErrGlobalParamsOwner
			}
			format, err := x.ast.SMT2PPExprTree(frame.expression, nil)
			if err != nil {
				return err
			}
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: x.ast.BudgetField})
			if err != nil {
				return err
			}
			empty := g.Context.EmptyValue()
			if err = x.ast.PrintFormat(out, format, g.Context, &empty, g); err != nil {
				return errors.Join(err, out.Close())
			}
			body, err := out.String()
			if err != nil {
				return errors.Join(err, out.Close())
			}
			_, err = io.WriteString(w, body+"\n")
			return errors.Join(err, out.Close())
		})
}

func (x *Expr2Polynomial) translate(root ASTHandle) (result *exprPolynomialValue, err error) {
	value, op, frame, err := x.visit(root)
	if err != nil || !frame {
		return value, err
	}
	count, err := x.ast.GetNumArgs(root)
	if err != nil {
		return nil, err
	}
	frames := []exprPolynomialFrame{{expression: root, op: op, count: count}}
	defer func() {
		if err != nil {
			err = errors.Join(err, closeExprPolynomialFrames(frames))
		}
	}()
	for len(frames) != 0 {
		if err = x.checkpoint(); err != nil {
			return nil, err
		}
		current := &frames[len(frames)-1]
		if err = x.traceFrame(current); err != nil {
			return nil, err
		}
		descended := false
		for current.next < current.count {
			child, e := x.ast.GetArg(current.expression, current.next)
			if e != nil {
				return nil, e
			}
			current.next++
			childValue, childOp, childFrame, e := x.visit(child)
			if e != nil {
				return nil, e
			}
			if childFrame {
				childCount, e := x.ast.GetNumArgs(child)
				if e != nil {
					return nil, e
				}
				frames = append(frames, exprPolynomialFrame{expression: child, op: childOp, count: childCount})
				descended = true
				break
			}
			current.args = append(current.args, childValue)
		}
		if descended {
			continue
		}
		value, e := x.process(current.expression, current.op, current.args)
		if e == nil {
			e = x.publishCache(current.expression, value)
		}
		for _, arg := range current.args {
			e = errors.Join(e, arg.close())
		}
		current.args = nil
		if e != nil {
			return nil, e
		}
		frames = frames[:len(frames)-1]
		if len(frames) == 0 {
			return value, nil
		}
		parent := &frames[len(frames)-1]
		parent.args = append(parent.args, value)
	}
	return nil, ErrASTDomain
}

func (x *Expr2Polynomial) process(h ASTHandle, op int32, args []*exprPolynomialValue) (*exprPolynomialValue, error) {
	if op == 14 {
		return x.cloneValue(args[0])
	}
	if op == 8 {
		p, err := args[0].polynomial.Get()
		if err != nil {
			return nil, err
		}
		p, err = x.core.Neg(p)
		if err != nil {
			return nil, err
		}
		d, err := args[0].denominator.Get()
		if err != nil {
			return nil, err
		}
		return x.newValue(p, d)
	}
	if op == 18 {
		exponentHandle, err := x.ast.GetArg(h, 1)
		if err != nil {
			return nil, err
		}
		text, _, matched, err := x.ast.ArithmeticNumeralText(exponentHandle)
		if err != nil || !matched {
			return nil, errors.Join(ErrExprNotPolynomial, err)
		}
		k64, err := strconv.ParseUint(text, 10, 32)
		if err != nil {
			return nil, ErrExprNotPolynomial
		}
		base, err := args[0].polynomial.Get()
		if err != nil {
			return nil, err
		}
		ref, err := polynomial.NewPolynomialRef(x.core)
		if err != nil {
			return nil, err
		}
		if err = x.core.Power(base, uint32(k64), ref); err != nil {
			return nil, errors.Join(err, ref.Close())
		}
		if _, err = ref.Get(); err != nil {
			return nil, errors.Join(err, ref.Close())
		}
		d, err := args[0].denominator.Clone()
		if err != nil {
			return nil, errors.Join(err, ref.Close())
		}
		dv, _ := d.Get()
		if err = x.integers.Power(dv, uint32(k64), dv); err != nil {
			return nil, errors.Join(err, d.Close(), ref.Close())
		}
		return &exprPolynomialValue{polynomial: ref, denominator: d}, nil
	}
	if len(args) == 0 {
		return nil, ErrASTDomain
	}
	if op == 9 {
		one := unumeral.NewInt(1)
		p, err := x.core.MkConstantTakeCopy(&one)
		if err != nil {
			return nil, err
		}
		accumulator, err := polynomial.NewPolynomialRef(x.core)
		if err != nil {
			return nil, err
		}
		if err = accumulator.Assign(p); err != nil {
			return nil, errors.Join(err, accumulator.Close())
		}
		d := unumeral.NewInt(1)
		for _, arg := range args {
			ap, e := arg.polynomial.Get()
			if e != nil {
				return nil, errors.Join(e, accumulator.Close())
			}
			current, e := accumulator.Get()
			if e != nil {
				return nil, errors.Join(e, accumulator.Close())
			}
			p, e = x.core.Mul(current, ap)
			if e != nil {
				return nil, errors.Join(e, accumulator.Close())
			}
			if e = accumulator.Assign(p); e != nil {
				return nil, errors.Join(e, accumulator.Close())
			}
			ad, e := arg.denominator.Get()
			if e != nil {
				return nil, errors.Join(e, accumulator.Close())
			}
			if e = x.integers.Mul(&d, ad, &d); e != nil {
				return nil, errors.Join(e, accumulator.Close())
			}
		}
		defer x.integers.Del(&d)
		scoped, err := unumeral.NewScopedInt(x.integers)
		if err != nil {
			return nil, errors.Join(err, accumulator.Close())
		}
		if err = scoped.Set(&d); err != nil {
			return nil, errors.Join(err, scoped.Close(), accumulator.Close())
		}
		return &exprPolynomialValue{polynomial: accumulator, denominator: scoped}, nil
	}
	denominator := unumeral.NewInt(1)
	defer x.integers.Del(&denominator)
	for _, arg := range args {
		d, err := arg.denominator.Get()
		if err != nil {
			return nil, err
		}
		if err = x.integers.LCM(&denominator, d, &denominator); err != nil {
			return nil, err
		}
	}
	unit, err := x.monomials.Unit()
	if err != nil {
		return nil, err
	}
	accumulator, err := polynomial.NewPolynomialRef(x.core)
	if err != nil {
		return nil, err
	}
	for i, arg := range args {
		d, e := arg.denominator.Get()
		if e != nil {
			return nil, e
		}
		scale := unumeral.NewInt(0)
		if e = x.integers.Div(&denominator, d, &scale); e != nil {
			return nil, e
		}
		ap, e := arg.polynomial.Get()
		if e != nil {
			_ = x.integers.Del(&scale)
			return nil, e
		}
		scaled, e := x.core.Scale(&scale, unit, ap)
		_ = x.integers.Del(&scale)
		if e != nil {
			return nil, errors.Join(e, accumulator.Close())
		}
		scaledRef, e := polynomial.NewPolynomialRef(x.core)
		if e != nil {
			return nil, errors.Join(e, accumulator.Close())
		}
		if e = scaledRef.Assign(scaled); e != nil {
			return nil, errors.Join(e, scaledRef.Close(), accumulator.Close())
		}
		if i == 0 {
			e = accumulator.Assign(scaled)
		} else if op == 6 {
			current, readErr := accumulator.Get()
			if readErr != nil {
				e = readErr
			} else {
				var result polynomial.Polynomial
				result, e = x.core.Add(current, scaled)
				if e == nil {
					e = accumulator.Assign(result)
				}
			}
		} else {
			current, readErr := accumulator.Get()
			if readErr != nil {
				e = readErr
			} else {
				var result polynomial.Polynomial
				result, e = x.core.Sub(current, scaled)
				if e == nil {
					e = accumulator.Assign(result)
				}
			}
		}
		e = errors.Join(e, scaledRef.Close())
		if e != nil {
			return nil, errors.Join(e, accumulator.Close())
		}
	}
	d, err := unumeral.NewScopedInt(x.integers)
	if err != nil {
		return nil, errors.Join(err, accumulator.Close())
	}
	if err = d.Set(&denominator); err != nil {
		return nil, errors.Join(err, d.Close(), accumulator.Close())
	}
	return &exprPolynomialValue{polynomial: accumulator, denominator: d}, nil
}

// ToPolynomial returns owning polynomial and denominator results. The caller
// closes both. A false match is reserved for non-Int/Real expressions.
func (x *Expr2Polynomial) ToPolynomial(h ASTHandle) (*polynomial.PolynomialRef, *unumeral.ScopedInt, bool, error) {
	if err := x.open(); err != nil {
		return nil, nil, false, err
	}
	_, matched, err := x.isIntReal(h)
	if err != nil || !matched {
		return nil, nil, false, err
	}
	v, err := x.translate(h)
	if err != nil {
		return nil, nil, true, err
	}
	return v.polynomial, v.denominator, true, nil
}

func (x *Expr2Polynomial) isIntegerPolynomial(p polynomial.Polynomial) (bool, error) {
	info, err := x.core.Info(p)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < info.Size; i++ {
		term, err := x.core.TermMonomial(p, i)
		if err != nil {
			return false, err
		}
		mi, err := x.monomials.Info(term)
		if err != nil {
			return false, err
		}
		for j := uint32(0); j < mi.Size; j++ {
			power, err := x.monomials.ReadPower(term, j)
			if err != nil {
				return false, err
			}
			if x.useVarIndices {
				return false, nil
			}
			if x.variables != nil {
				integer, err := x.variables.IsExprPolynomialInteger(power.Var)
				if err != nil || !integer {
					return false, err
				}
			} else if uint64(power.Var) >= uint64(len(x.varIsInt)) || !x.varIsInt[power.Var] {
				return false, nil
			}
		}
	}
	return true, nil
}

func (x *Expr2Polynomial) makeIntegerNumeral(value *unumeral.Int, integer bool) (ASTHandle, error) {
	text, err := x.integers.String(value)
	if err != nil {
		return ASTHandle{}, err
	}
	n, err := x.ast.numerals.NewValue()
	if err != nil {
		return ASTHandle{}, err
	}
	if err = x.ast.numerals.SetString(&n, text); err != nil {
		return ASTHandle{}, errors.Join(err, x.ast.numerals.Del(&n))
	}
	h, err := x.ast.ArithmeticNumeral(&n, integer)
	return h, errors.Join(err, x.ast.numerals.Del(&n))
}

func (x *Expr2Polynomial) makeSmallNumeral(value uint32, integer bool) (ASTHandle, error) {
	n := unumeral.NewInt(0)
	if err := x.integers.SetString(&n, strconv.FormatUint(uint64(value), 10)); err != nil {
		return ASTHandle{}, err
	}
	h, err := x.makeIntegerNumeral(&n, integer)
	return h, errors.Join(err, x.integers.Del(&n))
}

func (x *Expr2Polynomial) variableExpression(variable uint32, polynomialInteger bool) (ASTHandle, error) {
	if x.useVarIndices {
		return x.ast.MkVar(variable, x.ast.external(x.ast.arithmeticPlugin.fields[0]))
	}
	if uint64(variable) >= uint64(len(x.var2expr)) || x.var2expr[variable] == (ASTRef{}) {
		return ASTHandle{}, ErrASTDomain
	}
	h, err := x.ast.RefGet(x.var2expr[variable])
	if err != nil {
		return ASTHandle{}, err
	}
	if x.varIsInt[variable] && !polynomialInteger {
		return x.ast.MkFamilyApp(x.ast.arithmeticPlugin.family, 14, nil, []ASTHandle{h})
	}
	return h, nil
}

// ToExpr reconstructs a polynomial as an owning AST reference. It uses power
// nodes when requested and repeats factors otherwise, matching the source.
func (x *Expr2Polynomial) ToExpr(p polynomial.Polynomial, usePower bool) (out ASTRef, err error) {
	if err = x.open(); err != nil {
		return ASTRef{}, err
	}
	if x.ast.arithmeticPlugin == nil {
		return ASTRef{}, ErrASTDomain
	}
	integer, err := x.isIntegerPolynomial(p)
	if err != nil {
		return ASTRef{}, err
	}
	var held []ASTRef
	hold := func(h ASTHandle) (ASTHandle, error) {
		r, e := x.ast.NewRef(h)
		if e == nil {
			held = append(held, r)
		}
		return h, e
	}
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			err = errors.Join(err, x.ast.DelRef(&held[i]))
		}
		if err != nil && out != (ASTRef{}) {
			err = errors.Join(err, x.ast.DelRef(&out))
		}
	}()
	info, err := x.core.Info(p)
	if err != nil {
		return ASTRef{}, err
	}
	terms := make([]ASTHandle, 0, info.Size)
	for i := uint32(0); i < info.Size; i++ {
		coefficient, e := x.core.Coefficient(p, i)
		if e != nil {
			return ASTRef{}, e
		}
		one, e := x.integers.IsOne(&coefficient)
		if e != nil {
			return ASTRef{}, e
		}
		factors := make([]ASTHandle, 0, 4)
		if !one {
			h, e := x.makeIntegerNumeral(&coefficient, integer)
			if e != nil {
				return ASTRef{}, e
			}
			h, e = hold(h)
			if e != nil {
				return ASTRef{}, e
			}
			factors = append(factors, h)
		}
		term, e := x.core.TermMonomial(p, i)
		if e != nil {
			return ASTRef{}, e
		}
		mi, e := x.monomials.Info(term)
		if e != nil {
			return ASTRef{}, e
		}
		for j := uint32(0); j < mi.Size; j++ {
			power, e := x.monomials.ReadPower(term, j)
			if e != nil {
				return ASTRef{}, e
			}
			variable, e := x.variableExpression(power.Var, integer)
			if e != nil {
				return ASTRef{}, e
			}
			variable, e = hold(variable)
			if e != nil {
				return ASTRef{}, e
			}
			if usePower && power.Degree > 1 {
				exponent, e := x.makeSmallNumeral(power.Degree, integer)
				if e != nil {
					return ASTRef{}, e
				}
				exponent, e = hold(exponent)
				if e != nil {
					return ASTRef{}, e
				}
				variable, e = x.ast.MkFamilyApp(x.ast.arithmeticPlugin.family, 18, nil, []ASTHandle{variable, exponent})
				if e != nil {
					return ASTRef{}, e
				}
				variable, e = hold(variable)
				if e != nil {
					return ASTRef{}, e
				}
				factors = append(factors, variable)
			} else {
				for k := uint32(0); k < power.Degree; k++ {
					factors = append(factors, variable)
				}
			}
		}
		var expression ASTHandle
		switch len(factors) {
		case 0:
			expression, err = x.makeSmallNumeral(1, integer)
		case 1:
			expression = factors[0]
		default:
			expression, err = x.ast.MkFamilyApp(x.ast.arithmeticPlugin.family, 9, nil, factors)
		}
		if err != nil {
			return ASTRef{}, err
		}
		expression, err = hold(expression)
		if err != nil {
			return ASTRef{}, err
		}
		terms = append(terms, expression)
	}
	var root ASTHandle
	switch len(terms) {
	case 0:
		root, err = x.makeSmallNumeral(0, integer)
	case 1:
		root = terms[0]
	default:
		root, err = x.ast.MkFamilyApp(x.ast.arithmeticPlugin.family, 6, nil, terms)
	}
	if err != nil {
		return ASTRef{}, err
	}
	out, err = x.ast.NewRef(root)
	return out, err
}

func (x *Expr2Polynomial) IsVar(h ASTHandle) (bool, error) {
	if err := x.open(); err != nil {
		return false, err
	}
	if x.useVarIndices {
		return false, ErrASTDomain
	}
	return x.mapping.IsVar(h)
}

func (x *Expr2Polynomial) Mapping() (*Expr2Var, error) {
	if err := x.open(); err != nil {
		return nil, err
	}
	if x.useVarIndices {
		return nil, ErrASTDomain
	}
	return x.mapping, nil
}

func (x *Expr2Polynomial) ResetCache() error {
	if err := x.open(); err != nil {
		return err
	}
	return x.resetCache(false)
}

func (x *Expr2Polynomial) resetCache(astClosed bool) error {
	for len(x.cached) != 0 {
		i := len(x.cached) - 1
		entry := &x.cached[i]
		if err := entry.value.close(); err != nil {
			return err
		}
		if !astClosed {
			if err := x.ast.DelRef(&entry.domain); err != nil {
				return err
			}
		}
		delete(x.cache, entry.handle)
		x.cached = x.cached[:i]
	}
	return nil
}

func (x *Expr2Polynomial) Close() error {
	if x == nil || x.self != x {
		return ErrASTHandle
	}
	if x.closed {
		return nil
	}
	astClosed := x.ast.Closed
	if err := x.resetCache(astClosed); err != nil {
		return err
	}
	if !astClosed {
		for i := range x.var2expr {
			if x.var2expr[i] != (ASTRef{}) {
				if err := x.ast.DelRef(&x.var2expr[i]); err != nil {
					return err
				}
			}
		}
	}
	if x.ownMapping {
		if err := x.mapping.Close(); err != nil {
			return err
		}
	}
	x.cache = nil
	x.cached = nil
	x.var2expr = nil
	x.varIsInt = nil
	x.closed = true
	return nil
}
