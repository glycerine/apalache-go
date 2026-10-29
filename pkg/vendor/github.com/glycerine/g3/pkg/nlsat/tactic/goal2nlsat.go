package tactic

// Source: src/nlsat/tactic/goal2nlsat.cpp:27-253;
// src/nlsat/tactic/goal2nlsat.h:27-58.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/nlsat"
	"github.com/glycerine/g3/pkg/sat"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var (
	ErrGoalNotCNF                = errors.New("convert goal into cnf before applying nlsat")
	ErrGoalNeedsSimplify         = errors.New("apply simplify before applying nlsat")
	ErrGoalNeedsDistinctElim     = errors.New("eliminate distinct before applying nlsat")
	ErrGoalNeedsArithmeticPurify = errors.New("apply purify-arith before applying nlsat")
)

// DependencyAttachment records the borrowed goal dependency represented by an
// opaque NLSAT assumption attachment. The goal must outlive solver checking and
// core extraction.
type DependencyAttachment struct {
	Attachment *nlsat.AssumptionAttachment
	Dependency ast.ASTDependency
}

// GoalCompiler compiles a preprocessed CNF goal into the retained NLSAT solver.
// It borrows all constructor arguments and owns its expression mappings.
type GoalCompiler struct {
	manager     *ast.ASTManager
	solver      *nlsat.Solver
	polynomials *polynomial.MultivariatePolynomialManager
	atoms       *ast.Expr2Var
	terms       *ast.Expr2Var
	expressions *ast.Expr2Polynomial
	arithmetic  ast.FamilyID
	attachments []DependencyAttachment
	factor      bool
	closed      bool
}

// SetFactor selects goal2nlsat's source polynomial-factorization branch.
func (c *GoalCompiler) SetFactor(factor bool) error {
	if c == nil || c.closed {
		return arena.ErrClosed
	}
	c.factor = factor
	return nil
}

func NewGoalCompiler(manager *ast.ASTManager, solver *nlsat.Solver) (compiler *GoalCompiler, err error) {
	if manager == nil || solver == nil {
		return nil, arena.ErrConfig
	}
	compiler = &GoalCompiler{manager: manager, solver: solver}
	defer func() {
		if err != nil {
			err = errors.Join(err, compiler.Close())
			compiler = nil
		}
	}()
	compiler.polynomials, err = solver.PolynomialManager()
	if err != nil {
		return nil, err
	}
	compiler.arithmetic, err = manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	compiler.atoms, err = ast.NewExpr2Var(manager)
	if err != nil {
		return nil, err
	}
	compiler.terms, err = ast.NewExpr2Var(manager)
	if err != nil {
		return nil, err
	}
	compiler.expressions, err = ast.NewExpr2Polynomial(ast.Expr2PolynomialConfig{
		AST: manager, Polynomial: compiler.polynomials, Mapping: compiler.terms, Variables: solver,
	})
	if err != nil {
		return nil, err
	}
	return compiler, nil
}

func (c *GoalCompiler) processBooleanAtom(expression ast.ASTHandle) (sat.Literal, error) {
	variable, err := c.atoms.ToVar(expression)
	if err != nil {
		return 0, err
	}
	if variable == ^uint32(0) {
		created, cause := c.solver.MkBoolVar()
		if cause != nil {
			return 0, cause
		}
		variable = uint32(created)
		if err = c.atoms.Insert(expression, variable); err != nil {
			return 0, err
		}
	}
	return sat.MakeLiteral(sat.BoolVar(variable), false), nil
}

func (c *GoalCompiler) processArithmeticAtom(expression ast.ASTHandle, kind nlsat.AtomKind) (literal sat.Literal, err error) {
	if count, cause := c.manager.GetNumArgs(expression); cause != nil {
		return 0, cause
	} else if count != 2 {
		return 0, ast.ErrASTDomain
	}
	left, err := c.manager.GetArg(expression, 0)
	if err != nil {
		return 0, err
	}
	right, err := c.manager.GetArg(expression, 1)
	if err != nil {
		return 0, err
	}
	p1, d1, matched, err := c.expressions.ToPolynomial(left)
	if err != nil || !matched {
		return 0, errors.Join(ast.ErrExprNotPolynomial, err)
	}
	defer func() { err = errors.Join(err, p1.Close(), d1.Close()) }()
	p2, d2, matched, err := c.expressions.ToPolynomial(right)
	if err != nil || !matched {
		return 0, errors.Join(ast.ErrExprNotPolynomial, err)
	}
	defer func() { err = errors.Join(err, p2.Close(), d2.Close()) }()

	core, err := c.polynomials.ArithmeticCore()
	if err != nil {
		return 0, err
	}
	integers, err := core.IntegerManager()
	if err != nil {
		return 0, err
	}
	monomials, err := core.MonomialManager()
	if err != nil {
		return 0, err
	}
	leftDenominator, err := d1.Get()
	if err != nil {
		return 0, err
	}
	rightDenominator, err := d2.Get()
	if err != nil {
		return 0, err
	}
	lcm := unumeral.NewInt(0)
	leftScale := unumeral.NewInt(0)
	rightScale := unumeral.NewInt(0)
	defer func() {
		err = errors.Join(err, integers.Del(&rightScale), integers.Del(&leftScale), integers.Del(&lcm))
	}()
	if err = integers.LCM(leftDenominator, rightDenominator, &lcm); err != nil {
		return 0, err
	}
	if err = integers.Div(&lcm, leftDenominator, &leftScale); err != nil {
		return 0, err
	}
	if err = integers.Div(&lcm, rightDenominator, &rightScale); err != nil {
		return 0, err
	}
	if err = integers.Neg(&rightScale); err != nil {
		return 0, err
	}
	unit, err := monomials.Unit()
	if err != nil {
		return 0, err
	}
	leftPolynomial, err := p1.Get()
	if err != nil {
		return 0, err
	}
	rightPolynomial, err := p2.Get()
	if err != nil {
		return 0, err
	}
	difference, err := core.AddMul(&leftScale, unit, leftPolynomial, &rightScale, unit, rightPolynomial)
	if err != nil {
		return 0, err
	}
	owned, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return 0, err
	}
	if err = owned.Assign(difference); err != nil {
		return 0, errors.Join(err, owned.Close())
	}
	defer func() { err = errors.Join(err, owned.Close()) }()
	maxVariable, err := core.MaxVariable(difference)
	if err != nil {
		return 0, err
	}
	if !c.factor || maxVariable == nlsat.NullVariable {
		return c.solver.MkInequality(kind, []polynomial.Polynomial{difference}, []bool{false}, false)
	}
	factors, err := polynomial.NewPolynomialFactors(c.polynomials)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, factors.CloseChecked()) }()
	if err = c.polynomials.Factor(difference, factors, polynomial.DefaultUPolynomialFactorParams()); err != nil {
		return 0, err
	}
	count, err := factors.Distinct()
	if err != nil || count == 0 {
		return 0, errors.Join(polynomial.ErrPolynomialPartial, err)
	}
	constant, err := factors.Constant()
	if err != nil {
		return 0, err
	}
	negative, err := integers.IsNeg(constant)
	if err != nil {
		return 0, err
	}
	if negative {
		kind, err = kind.Flip()
		if err != nil {
			return 0, err
		}
	}
	pieces := make([]polynomial.Polynomial, count)
	even := make([]bool, count)
	for i := uint32(0); i < count; i++ {
		pieces[i], err = factors.Factor(i)
		if err != nil {
			return 0, err
		}
		degree, cause := factors.Degree(i)
		if cause != nil {
			return 0, cause
		}
		even[i] = degree%2 == 0
	}
	return c.solver.MkInequality(kind, pieces, even, false)
}

func (c *GoalCompiler) processAtom(expression ast.ASTHandle) (sat.Literal, error) {
	family, err := c.manager.GetFamilyID(expression)
	if err != nil {
		return 0, err
	}
	op, err := c.manager.GetDeclKind(expression)
	if err != nil {
		return 0, err
	}
	if family == c.manager.BasicFamilyID() {
		switch op {
		case ast.ASTOpEq:
			left, cause := c.manager.GetArg(expression, 0)
			if cause != nil {
				return 0, cause
			}
			sort, cause := c.manager.GetSort(left)
			if cause != nil {
				return 0, cause
			}
			sortFamily, cause := c.manager.GetFamilyID(sort)
			if cause != nil {
				return 0, cause
			}
			if sortFamily == c.arithmetic {
				return c.processArithmeticAtom(expression, nlsat.AtomEQ)
			}
			return c.processBooleanAtom(expression)
		case ast.ASTOpTrue, ast.ASTOpFalse:
			return 0, ErrGoalNeedsSimplify
		case ast.ASTOpDistinct:
			return 0, ErrGoalNeedsDistinctElim
		case ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpIff, ast.ASTOpXor, ast.ASTOpNot, ast.ASTOpImplies:
			return 0, ErrGoalNotCNF
		default:
			return 0, ast.ErrASTDomain
		}
	}
	if family == c.arithmetic {
		switch op {
		case 2:
			literal, cause := c.processArithmeticAtom(expression, nlsat.AtomGT)
			return literal.Negated(), cause
		case 3:
			literal, cause := c.processArithmeticAtom(expression, nlsat.AtomLT)
			return literal.Negated(), cause
		default:
			return 0, ErrGoalNeedsArithmeticPurify
		}
	}
	return c.processBooleanAtom(expression)
}

func (c *GoalCompiler) processLiteral(expression ast.ASTHandle) (sat.Literal, error) {
	negative := false
	for {
		matched, child, err := c.manager.IsNot(expression)
		if err != nil {
			return 0, err
		}
		if !matched {
			break
		}
		negative = !negative
		expression = child
	}
	literal, err := c.processAtom(expression)
	if negative {
		literal = literal.Negated()
	}
	return literal, err
}

func (c *GoalCompiler) processFormula(formula ast.ASTHandle, dependency ast.ASTDependency) error {
	literals := []ast.ASTHandle{formula}
	if family, err := c.manager.GetFamilyID(formula); err != nil {
		return err
	} else if op, cause := c.manager.GetDeclKind(formula); cause != nil {
		return cause
	} else if family == c.manager.BasicFamilyID() && op == ast.ASTOpOr {
		count, cause := c.manager.GetNumArgs(formula)
		if cause != nil {
			return cause
		}
		literals = make([]ast.ASTHandle, count)
		for i := range literals {
			literals[i], cause = c.manager.GetArg(formula, uint32(i))
			if cause != nil {
				return cause
			}
		}
	}
	compiled := make([]sat.Literal, len(literals))
	for i, expression := range literals {
		literal, err := c.processLiteral(expression)
		if err != nil {
			return err
		}
		compiled[i] = literal
	}
	assumption := nlsat.Assumption{}
	if !dependency.IsNull() {
		attachment := &nlsat.AssumptionAttachment{}
		c.attachments = append(c.attachments, DependencyAttachment{Attachment: attachment, Dependency: dependency})
		assumption = nlsat.AttachedAssumption(attachment)
	}
	return c.solver.AddClause(compiled, assumption)
}

func (c *GoalCompiler) Compile(goal *basetactic.Goal) error {
	if c == nil || c.closed || goal == nil {
		return arena.ErrClosed
	}
	count, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		formula, cause := goal.Form(i)
		if cause != nil {
			return cause
		}
		dependency, cause := goal.Dependency(i)
		if cause != nil {
			return cause
		}
		if cause = c.processFormula(formula, dependency); cause != nil {
			return cause
		}
	}
	return nil
}

func (c *GoalCompiler) AtomMapping() *ast.Expr2Var { return c.atoms }
func (c *GoalCompiler) TermMapping() *ast.Expr2Var { return c.terms }
func (c *GoalCompiler) DependencyAttachments() []DependencyAttachment {
	return append([]DependencyAttachment(nil), c.attachments...)
}

func (c *GoalCompiler) isUninterpretedConstant(expression ast.ASTHandle) (bool, error) {
	kind, err := c.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	count, err := c.manager.GetNumArgs(expression)
	if err != nil || count != 0 {
		return false, err
	}
	declaration, err := c.manager.GetDecl(expression)
	if err != nil {
		return false, err
	}
	family, err := c.manager.GetFamilyID(declaration)
	return family == ast.NullFamilyID, err
}

// ContainsUnsupported implements the source tactic's post-check guard. Terms
// and abstract Boolean atoms must be uninterpreted constants unless the latter
// Boolean variable is owned by an arithmetic atom.
func (c *GoalCompiler) ContainsUnsupported() (bool, error) {
	terms, err := c.terms.Entries()
	if err != nil {
		return false, err
	}
	for _, entry := range terms {
		constant, cause := c.isUninterpretedConstant(entry.Expr)
		if cause != nil || !constant {
			return true, cause
		}
	}
	atoms, err := c.atoms.Entries()
	if err != nil {
		return false, err
	}
	for _, entry := range atoms {
		constant, cause := c.isUninterpretedConstant(entry.Expr)
		if cause != nil {
			return false, cause
		}
		if constant {
			continue
		}
		interpreted, cause := c.solver.IsInterpreted(sat.BoolVar(entry.Variable))
		if cause != nil || !interpreted {
			return true, cause
		}
	}
	return false, nil
}

func (c *GoalCompiler) algebraicNumeral(value *polynomial.Algebraic, integer bool) (result ast.ASTHandle, err error) {
	algebraic, err := c.solver.AlgebraicManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rational, err := algebraic.IsRational(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !rational {
		if integer {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return c.manager.ArithmeticAlgebraicNumeral(value)
	}
	rationals, err := algebraic.RationalManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var q unumeral.Rational
	defer func() { err = errors.Join(err, rationals.Del(&q)) }()
	if err = algebraic.ToRational(value, &q); err != nil {
		return ast.ASTHandle{}, err
	}
	text, err := rationals.String(&q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if integer {
		return c.manager.ArithmeticIntegerNumeralText(text)
	}
	return c.manager.ArithmeticRealNumeralText(text)
}

// Model constructs the source constant-only model after a SAT check. A false
// supported result leaves the goal undecided, matching nlsat_tactic.
func (c *GoalCompiler) Model() (result *model.Model, supported bool, err error) {
	unsupported, err := c.ContainsUnsupported()
	if err != nil || unsupported {
		return nil, !unsupported, err
	}
	result, err = model.NewModel(c.manager)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if err != nil || !supported {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	algebraic, err := c.solver.AlgebraicManager()
	if err != nil {
		return nil, false, err
	}
	terms, err := c.terms.Entries()
	if err != nil {
		return nil, false, err
	}
	for _, entry := range terms {
		var value polynomial.Algebraic
		if err = c.solver.Value(entry.Variable, &value); err != nil {
			return nil, false, err
		}
		var numeral ast.ASTHandle
		func() {
			defer func() { err = errors.Join(err, algebraic.Del(&value)) }()
			sort, cause := c.manager.GetSort(entry.Expr)
			if cause != nil {
				err = cause
				return
			}
			kind, cause := c.manager.GetDeclKind(sort)
			if cause != nil {
				err = cause
				return
			}
			numeral, err = c.algebraicNumeral(&value, kind == 1)
		}()
		if err != nil {
			return nil, false, err
		}
		declaration, cause := c.manager.GetDecl(entry.Expr)
		if cause != nil {
			return nil, false, cause
		}
		if cause = result.Core().RegisterConstant(declaration, numeral); cause != nil {
			return nil, false, cause
		}
	}
	atoms, err := c.atoms.Entries()
	if err != nil {
		return nil, false, err
	}
	for _, entry := range atoms {
		value, cause := c.solver.BooleanValue(sat.BoolVar(entry.Variable))
		if cause != nil {
			return nil, false, cause
		}
		if value == lbool.Undef {
			continue
		}
		var interpretation ast.ASTHandle
		if value == lbool.True {
			interpretation, cause = c.manager.MkTrue()
		} else {
			interpretation, cause = c.manager.MkFalse()
		}
		if cause != nil {
			return nil, false, cause
		}
		declaration, cause := c.manager.GetDecl(entry.Expr)
		if cause != nil {
			return nil, false, cause
		}
		if cause = result.Core().RegisterConstant(declaration, interpretation); cause != nil {
			return nil, false, cause
		}
	}
	return result, true, nil
}

// Core joins the goal dependencies attached to the solver's final conflict.
func (c *GoalCompiler) Core(out *basetactic.DependencyRef) (err error) {
	if out == nil {
		return ast.ErrASTHandle
	}
	var assumptions []nlsat.Assumption
	if err = c.solver.Core(&assumptions); err != nil {
		return err
	}
	dependencies := make(map[*nlsat.AssumptionAttachment]ast.ASTDependency, len(c.attachments))
	for _, attachment := range c.attachments {
		dependencies[attachment.Attachment] = attachment.Dependency
	}
	var joined basetactic.DependencyRef
	defer func() { err = errors.Join(err, joined.Close()) }()
	for _, assumption := range assumptions {
		dependency, present := dependencies[assumption.Attachment()]
		if !present {
			continue
		}
		combined, cause := c.manager.DependencyJoin(joined.Get(), dependency)
		if cause != nil {
			return cause
		}
		if cause = joined.Set(c.manager, combined); cause != nil {
			return cause
		}
	}
	return out.Set(c.manager, joined.Get())
}

func (c *GoalCompiler) Close() (err error) {
	if c == nil || c.closed {
		return nil
	}
	c.closed = true
	if c.expressions != nil {
		err = errors.Join(err, c.expressions.Close())
		c.expressions = nil
	}
	if c.terms != nil {
		err = errors.Join(err, c.terms.Close())
		c.terms = nil
	}
	if c.atoms != nil {
		err = errors.Join(err, c.atoms.Close())
		c.atoms = nil
	}
	c.attachments = nil
	return err
}
