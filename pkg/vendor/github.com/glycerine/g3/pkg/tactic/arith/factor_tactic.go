package arith

// Source: src/tactic/arith/factor_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	primegen "github.com/glycerine/g3/pkg/util/prime"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

const (
	factorLE int32 = 2
	factorGE int32 = 3
	factorLT int32 = 4
	factorGT int32 = 5
)

type factorInvocation struct {
	owner       *FactorTactic
	polynomials *polynomial.MultivariatePolynomialManager
	core        *polynomial.MultivariatePolynomialCore
	expressions *ast.Expr2Polynomial
	primes      *primegen.Generator
	rationals   *unumeral.RationalManager
}

func (i *factorInvocation) close() (err error) {
	if i.expressions != nil {
		err = errors.Join(err, i.expressions.Close())
		i.expressions = nil
	}
	if i.polynomials != nil {
		err = errors.Join(err, i.polynomials.CloseChecked())
		i.polynomials = nil
	}
	if i.primes != nil {
		err = errors.Join(err, i.primes.Close())
		i.primes = nil
	}
	return err
}

type factorRewriteConfig struct {
	rewriter.DefaultConfiguration
	invocation *factorInvocation
}

func (c *factorRewriteConfig) ReduceApp(decl ast.ASTHandle, arguments ast.AppArguments, out, _ ast.ASTRef) (rewriter.RewriteStatus, error) {
	if arguments.Count() != 2 {
		return rewriter.RewriteFailed, nil
	}
	m := c.invocation.owner.manager
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	op, err := m.GetDeclKind(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	comparison := family == c.invocation.owner.arithmetic && (op == factorLT || op == factorGT || op == factorLE || op == factorGE)
	equality := family == m.BasicFamilyID() && op == ast.ASTOpEq
	if !comparison && !equality {
		return rewriter.RewriteFailed, nil
	}
	left, err := arguments.At(0)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	right, err := arguments.At(1)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if equality {
		sort, e := m.GetSort(left)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		sortFamily, e := m.GetFamilyID(sort)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if sortFamily != c.invocation.owner.arithmetic {
			return rewriter.RewriteFailed, nil
		}
	}
	changed, result, err := c.invocation.factor(op, equality, left, right)
	if err != nil || !changed {
		return rewriter.RewriteFailed, err
	}
	defer func() { _ = m.DelRef(&result) }()
	handle, err := m.RefGet(result)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	return rewriter.RewriteDone, m.SetRef(out, handle)
}

func (i *factorInvocation) difference(left, right ast.ASTHandle) (_ *polynomial.PolynomialRef, matched bool, err error) {
	p1, d1, matched, err := i.expressions.ToPolynomial(left)
	if err != nil || !matched {
		return nil, matched, err
	}
	defer func() { err = errors.Join(err, p1.Close(), d1.Close()) }()
	p2, d2, matched, err := i.expressions.ToPolynomial(right)
	if err != nil || !matched {
		return nil, matched, err
	}
	defer func() { err = errors.Join(err, p2.Close(), d2.Close()) }()
	integers := i.rationals.Integers()
	leftDenominator, err := d1.Get()
	if err != nil {
		return nil, false, err
	}
	rightDenominator, err := d2.Get()
	if err != nil {
		return nil, false, err
	}
	lcm, leftScale, rightScale := unumeral.NewInt(0), unumeral.NewInt(0), unumeral.NewInt(0)
	defer func() {
		err = errors.Join(err, integers.Del(&rightScale), integers.Del(&leftScale), integers.Del(&lcm))
	}()
	if err = integers.LCM(leftDenominator, rightDenominator, &lcm); err != nil {
		return nil, false, err
	}
	if err = integers.Div(&lcm, leftDenominator, &leftScale); err != nil {
		return nil, false, err
	}
	if err = integers.Div(&lcm, rightDenominator, &rightScale); err != nil {
		return nil, false, err
	}
	if err = integers.Neg(&rightScale); err != nil {
		return nil, false, err
	}
	monomials, err := i.core.MonomialManager()
	if err != nil {
		return nil, false, err
	}
	unit, err := monomials.Unit()
	if err != nil {
		return nil, false, err
	}
	leftPolynomial, err := p1.Get()
	if err != nil {
		return nil, false, err
	}
	rightPolynomial, err := p2.Get()
	if err != nil {
		return nil, false, err
	}
	difference, err := i.core.AddMul(&leftScale, unit, leftPolynomial, &rightScale, unit, rightPolynomial)
	if err != nil {
		return nil, false, err
	}
	owned, err := polynomial.NewPolynomialRef(i.core)
	if err != nil {
		return nil, false, err
	}
	if err = owned.Assign(difference); err != nil {
		return nil, false, errors.Join(err, owned.Close())
	}
	return owned, true, nil
}

type factorExpressions struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
	values  []ast.ASTHandle
}

func (e *factorExpressions) add(ref ast.ASTRef) error {
	value, err := e.manager.RefGet(ref)
	if err != nil {
		return err
	}
	e.refs = append(e.refs, ref)
	e.values = append(e.values, value)
	return nil
}

func (e *factorExpressions) hold(value ast.ASTHandle) error {
	ref, err := e.manager.NewRef(value)
	if err != nil {
		return err
	}
	return e.add(ref)
}

func (e *factorExpressions) close() (err error) {
	for j := len(e.refs) - 1; j >= 0; j-- {
		err = errors.Join(err, e.manager.DelRef(&e.refs[j]))
	}
	return err
}

func (i *factorInvocation) numeralFor(term ast.ASTHandle, text string) (ast.ASTHandle, error) {
	m := i.owner.manager
	sort, err := m.GetSort(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	integer, err := m.ArithmeticIntegerNumeralText(text)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	integerSort, err := m.GetSort(integer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sort == integerSort {
		return integer, nil
	}
	return m.ArithmeticRealNumeralText(text)
}

func (i *factorInvocation) product(values []ast.ASTHandle, refs *factorExpressions) (ast.ASTHandle, error) {
	if len(values) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	result := values[0]
	for _, value := range values[1:] {
		product, err := i.owner.manager.MkArithmeticMul(result, value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = refs.hold(product); err != nil {
			return ast.ASTHandle{}, err
		}
		result = product
	}
	return result, nil
}

func flipFactorComparison(op int32) (int32, error) {
	switch op {
	case factorLT:
		return factorGT, nil
	case factorLE:
		return factorGE, nil
	case factorGT:
		return factorLT, nil
	case factorGE:
		return factorLE, nil
	default:
		return 0, ast.ErrASTDomain
	}
}

func (i *factorInvocation) factor(op int32, equality bool, left, right ast.ASTHandle) (changed bool, result ast.ASTRef, err error) {
	difference, matched, err := i.difference(left, right)
	if err != nil || !matched {
		return false, ast.ASTRef{}, err
	}
	defer func() { err = errors.Join(err, difference.Close()) }()
	p, err := difference.Get()
	if err != nil {
		return false, ast.ASTRef{}, err
	}
	constant, err := i.core.IsConstant(p)
	if err != nil || constant {
		return false, ast.ASTRef{}, err
	}
	factors, err := polynomial.NewPolynomialFactors(i.polynomials)
	if err != nil {
		return false, ast.ASTRef{}, err
	}
	defer func() { err = errors.Join(err, factors.CloseChecked()) }()
	if err = i.polynomials.Factor(p, factors, i.owner.factorParams); err != nil {
		return false, ast.ASTRef{}, err
	}
	count, err := factors.Distinct()
	if err != nil || count == 0 {
		return false, ast.ASTRef{}, errors.Join(polynomial.ErrPolynomialPartial, err)
	}
	degree, err := factors.Degree(0)
	if err != nil {
		return false, ast.ASTRef{}, err
	}
	if count == 1 && degree == 1 {
		return false, ast.ASTRef{}, nil
	}
	constantFactor, err := factors.Constant()
	if err != nil {
		return false, ast.ASTRef{}, err
	}
	negative, err := i.rationals.Integers().IsNeg(constantFactor)
	if err != nil {
		return false, ast.ASTRef{}, err
	}
	if !equality && negative {
		op, err = flipFactorComparison(op)
		if err != nil {
			return false, ast.ASTRef{}, err
		}
	}
	expressions := factorExpressions{manager: i.owner.manager}
	defer func() { err = errors.Join(err, expressions.close()) }()
	factorValues := make([]ast.ASTHandle, count)
	degrees := make([]uint32, count)
	for j := uint32(0); j < count; j++ {
		piece, e := factors.Factor(j)
		if e != nil {
			return false, ast.ASTRef{}, e
		}
		ref, e := i.expressions.ToExpr(piece, true)
		if e != nil {
			return false, ast.ASTRef{}, e
		}
		if e = expressions.add(ref); e != nil {
			_ = i.owner.manager.DelRef(&ref)
			return false, ast.ASTRef{}, e
		}
		factorValues[j] = expressions.values[len(expressions.values)-1]
		degrees[j], err = factors.Degree(j)
		if err != nil {
			return false, ast.ASTRef{}, err
		}
	}
	var rewritten ast.ASTHandle
	if equality {
		changed, rewritten, err = i.makeEquality(factorValues, &expressions)
	} else {
		changed, rewritten, err = i.makeComparison(op, factorValues, degrees, &expressions)
	}
	if err != nil || !changed {
		return changed, ast.ASTRef{}, err
	}
	result, err = i.owner.manager.NewRef(rewritten)
	return changed, result, err
}

func (i *factorInvocation) makeEquality(values []ast.ASTHandle, refs *factorExpressions) (bool, ast.ASTHandle, error) {
	m := i.owner.manager
	if i.owner.splitFactors {
		equalities := make([]ast.ASTHandle, len(values))
		for j, value := range values {
			zero, err := i.numeralFor(value, "0")
			if err != nil {
				return false, ast.ASTHandle{}, err
			}
			equalities[j], err = m.MkEq(value, zero)
			if err != nil {
				return false, ast.ASTHandle{}, err
			}
			if err = refs.hold(equalities[j]); err != nil {
				return false, ast.ASTHandle{}, err
			}
		}
		if len(equalities) == 1 {
			return true, equalities[0], nil
		}
		result, err := m.MkOrArgs(equalities)
		return true, result, err
	}
	product, err := i.product(values, refs)
	if err != nil {
		return false, ast.ASTHandle{}, err
	}
	zero, err := i.numeralFor(product, "0")
	if err != nil {
		return false, ast.ASTHandle{}, err
	}
	result, err := m.MkEq(product, zero)
	return true, result, err
}

func (i *factorInvocation) comparison(op int32, left, right ast.ASTHandle) (ast.ASTHandle, error) {
	return i.owner.manager.MkFamilyApp(i.owner.arithmetic, op, nil, []ast.ASTHandle{left, right})
}

func (i *factorInvocation) makeComparison(op int32, values []ast.ASTHandle, degrees []uint32, refs *factorExpressions) (bool, ast.ASTHandle, error) {
	m := i.owner.manager
	if !i.owner.splitFactors {
		terms := make([]ast.ASTHandle, len(values))
		copy(terms, values)
		for j := range terms {
			if degrees[j]%2 == 0 {
				two, err := i.numeralFor(terms[j], "2")
				if err != nil {
					return false, ast.ASTHandle{}, err
				}
				terms[j], err = m.MkArithmeticPower(terms[j], two)
				if err != nil {
					return false, ast.ASTHandle{}, err
				}
				if err = refs.hold(terms[j]); err != nil {
					return false, ast.ASTHandle{}, err
				}
			}
		}
		product, err := i.product(terms, refs)
		if err != nil {
			return false, ast.ASTHandle{}, err
		}
		zero, err := i.numeralFor(product, "0")
		if err != nil {
			return false, ast.ASTHandle{}, err
		}
		result, err := i.comparison(op, product, zero)
		return true, result, err
	}
	strict := op == factorLT || op == factorGT
	clauses := make([]ast.ASTHandle, 0, len(values)+1)
	odd := make([]ast.ASTHandle, 0, len(values))
	for j, value := range values {
		if degrees[j]%2 != 0 {
			odd = append(odd, value)
			continue
		}
		zero, err := i.numeralFor(value, "0")
		if err != nil {
			return false, ast.ASTHandle{}, err
		}
		equality, err := m.MkEq(value, zero)
		if err != nil {
			return false, ast.ASTHandle{}, err
		}
		if err = refs.hold(equality); err != nil {
			return false, ast.ASTHandle{}, err
		}
		if strict {
			equality, err = m.MkNot(equality)
			if err != nil {
				return false, ast.ASTHandle{}, err
			}
			if err = refs.hold(equality); err != nil {
				return false, ast.ASTHandle{}, err
			}
		}
		clauses = append(clauses, equality)
	}
	if len(odd) == 0 {
		if op == factorLT {
			result, err := m.MkFalse()
			return true, result, err
		}
		if op == factorGE {
			result, err := m.MkTrue()
			return true, result, err
		}
	} else {
		product, err := i.product(odd, refs)
		if err != nil {
			return false, ast.ASTHandle{}, err
		}
		zero, err := i.numeralFor(product, "0")
		if err != nil {
			return false, ast.ASTHandle{}, err
		}
		comparison, err := i.comparison(op, product, zero)
		if err != nil {
			return false, ast.ASTHandle{}, err
		}
		if err = refs.hold(comparison); err != nil {
			return false, ast.ASTHandle{}, err
		}
		clauses = append(clauses, comparison)
	}
	if len(clauses) == 1 {
		return true, clauses[0], nil
	}
	var result ast.ASTHandle
	var err error
	if strict {
		result, err = m.MkAndArgs(clauses)
	} else {
		result, err = m.MkOrArgs(clauses)
	}
	return true, result, err
}

type FactorTactic struct {
	self         *FactorTactic
	manager      *ast.ASTManager
	context      *uparams.ParamsContext
	globals      *uparams.GlobalParamsService
	services     model.ModelEvaluatorConfig
	params       uparams.ParamsRef
	factorParams polynomial.UPolynomialFactorParams
	arithmetic   ast.FamilyID
	splitFactors bool
	refs         uint32
	ownerBytes   uint64
	closed       bool
}

var _ tactic.Tactic = (*FactorTactic)(nil)

func NewFactorTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*FactorTactic, error) {
	if m == nil || context == nil || globals == nil || globals.Context != context || services.Context != context || services.Globals != globals || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	params, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(FactorTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, context.Del(&params))
	}
	t := &FactorTactic{manager: m, context: context, globals: globals, services: services, params: params, ownerBytes: amount}
	t.self = t
	if p != nil {
		err = context.Assign(&t.params, p)
	}
	if err == nil {
		t.arithmetic, err = m.RegisterArithmeticPlugin()
	}
	if err == nil {
		err = t.readPolicy(&t.params)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func MakeFactorTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewFactorTactic(m, context, globals, p, services)
	if err != nil {
		return nil, err
	}
	clean, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return clean, nil
}

func (t *FactorTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}

func (t *FactorTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *FactorTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}

func (t *FactorTactic) readPolicy(p *uparams.ParamsRef) (err error) {
	name := "split_factors"
	if t.splitFactors, err = t.context.GetBoolName(p, &name, true); err != nil {
		return err
	}
	params := polynomial.DefaultUPolynomialFactorParams()
	name = "max_prime"
	if params.MaxPrime, err = t.context.GetUIntName(p, &name, params.MaxPrime); err != nil {
		return err
	}
	name = "num_primes"
	if params.PrimeTrials, err = t.context.GetUIntName(p, &name, params.PrimeTrials); err != nil {
		return err
	}
	name = "max_search_size"
	params.MaxSearchSize, err = t.context.GetUIntName(p, &name, params.MaxSearchSize)
	t.factorParams = params
	return err
}

func (t *FactorTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.readPolicy(p)
}

func (t *FactorTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	entries := []struct{ name, text string }{
		{"split_factors", "(default: true) apply simplifications such as (= (* p1 p2) 0) --> (or (= p1 0) (= p2 0))."},
		{"max_search_size", "(default: infty) limit the polynomial factorization search space."},
		{"max_prime", "(default: infty) maximum prime used by polynomial factorization."},
		{"num_primes", "(default: 1) number of finite factorizations considered before lifting and searching."},
	}
	for j := range entries {
		kind := uparams.ParamUInt
		if j == 0 {
			kind = uparams.ParamBool
		}
		if err := out.InsertName(entries[j].name, kind, &entries[j].text, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func (t *FactorTactic) CollectStatistics() map[string]float64  { return nil }
func (t *FactorTactic) ResetStatistics() error                 { return t.open() }
func (t *FactorTactic) Cleanup() error                         { return t.open() }
func (t *FactorTactic) Reset() error                           { return t.Cleanup() }
func (t *FactorTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *FactorTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *FactorTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewFactorTactic(m, t.context, t.globals, &t.params, t.services)
}
func (t *FactorTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	err := t.context.Del(&t.params)
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return err
}

func (t *FactorTactic) newInvocation() (*factorInvocation, error) {
	i := &factorInvocation{owner: t}
	var err error
	algebraic, err := t.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return nil, err
	}
	i.rationals, err = algebraic.RationalManager()
	if err != nil {
		return nil, err
	}
	i.primes, err = primegen.NewGenerator(t.manager.Budget())
	if err != nil {
		return nil, err
	}
	i.polynomials, err = polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core:          polynomial.MultivariatePolynomialConfig{Integer: i.rationals.Integers(), Rationals: i.rationals, Checkpoint: t.manager.Limit.Inc, Cooperation: func() error { return t.services.Cooperation.Cooperate("factor") }},
		Factorization: polynomial.UPolynomialFactorizationConfig{Primes: i.primes}, Trace: t.services.Trace,
	})
	if err != nil {
		return nil, errors.Join(err, i.close())
	}
	i.core, err = i.polynomials.ArithmeticCore()
	if err == nil {
		i.expressions, err = ast.NewExpr2Polynomial(ast.Expr2PolynomialConfig{AST: t.manager, Polynomial: i.polynomials})
	}
	if err != nil {
		return nil, errors.Join(err, i.close())
	}
	return i, nil
}

func (t *FactorTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("factor", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	invocation, err := t.newInvocation()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, invocation.close()) }()
	config := &factorRewriteConfig{invocation: invocation}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	var traversal *rewriter.Traversal
	if proofs {
		traversal, err = rewriter.NewProofTraversal(t.manager, config)
	} else {
		traversal, err = rewriter.NewTraversal(t.manager, config)
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, traversal.Close()) }()
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for j := uint32(0); j < size; j++ {
		current, e := goal.Form(j)
		if e != nil {
			return e
		}
		newCurrent, e := t.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		newProof, e := t.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			_ = t.manager.DelRef(&newCurrent)
			return e
		}
		if proofs {
			e = traversal.RewriteWithProof(current, newCurrent, newProof)
		} else {
			e = traversal.Rewrite(current, newCurrent)
		}
		if e == nil {
			rewritten, x := t.manager.RefGet(newCurrent)
			if x != nil {
				e = x
			} else {
				proof := ast.ASTHandle{}
				if proofs {
					oldProof, x := goal.Proof(j)
					if x != nil {
						e = x
					} else if rewriteProof, x := t.manager.RefGet(newProof); x != nil {
						e = x
					} else {
						proof, e = t.manager.MkProofModusPonens(oldProof, rewriteProof)
					}
				}
				if e == nil {
					if dependency, x := goal.Dependency(j); x != nil {
						e = x
					} else {
						e = goal.Update(j, rewritten, proof, dependency)
					}
				}
			}
		}
		e = errors.Join(e, t.manager.DelRef(&newProof), t.manager.DelRef(&newCurrent))
		if e != nil {
			return e
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}
