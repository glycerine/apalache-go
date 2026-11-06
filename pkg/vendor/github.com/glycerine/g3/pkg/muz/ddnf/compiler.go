package ddnf

// Source: src/muz/ddnf/ddnf.cpp ddnf::imp::compile_sort, compile_var,
// compile_expr, and compile_eq.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// Compiler owns generated AST references and borrows a completed
// Preprocessor. Its value mapping is the source DDNF node-ID encoding.
type Compiler struct {
	preprocessor *Preprocessor
	manager      *ast.ASTManager
	rules        *base.RuleManager
	cache        map[ast.ASTHandle]ast.ASTHandle
	refs         []ast.ASTRef
	closed       bool
}

func NewCompiler(preprocessor *Preprocessor, rules *base.RuleManager) (*Compiler, error) {
	if preprocessor == nil || preprocessor.closed || preprocessor.failed || preprocessor.manager == nil || rules == nil {
		return nil, ErrCore
	}
	return &Compiler{
		preprocessor: preprocessor,
		manager:      preprocessor.manager,
		rules:        rules,
		cache:        make(map[ast.ASTHandle]ast.ASTHandle),
	}, nil
}

func (c *Compiler) Close() (result error) {
	if c == nil || c.closed {
		return nil
	}
	if err := c.releaseRefsTo(0); err != nil {
		return err
	}
	c.cache = nil
	c.preprocessor = nil
	c.rules = nil
	c.manager = nil
	c.closed = true
	return result
}

// DetachReferences transfers the compiler's cached-expression trail to the
// owning DDNF engine. The C++ m_cache is query-local, while m_trail survives
// until engine destruction; callers that need that lifetime detach before
// closing the compiler.
func (c *Compiler) DetachReferences() []ast.ASTRef {
	if c == nil || c.closed {
		return nil
	}
	references := c.refs
	c.refs = nil
	return references
}

// CompileRuleSet rewrites predicates and interpreted tails into finite
// DDNF-domain rules, preserving the source output-predicate selection.
func (c *Compiler) CompileRuleSet(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if c == nil || c.closed || c.manager == nil || c.rules == nil || source == nil {
		return nil, ErrCore
	}
	destination, err := base.NewRuleSet(c.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep || resultErr != nil {
			resultErr = errors.Join(resultErr, destination.Close())
		}
	}()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		if getErr = c.compileRule(rule, source, destination); getErr != nil {
			return nil, errors.Join(fmt.Errorf("muz/ddnf: compile rule %d: %w", index, getErr), rule.Close())
		}
		if getErr = rule.Close(); getErr != nil {
			return nil, getErr
		}
	}
	keep = true
	return destination, nil
}

func (c *Compiler) compileRule(source *base.Rule, oldRules, newRules *base.RuleSet) error {
	head, err := source.Head()
	if err != nil {
		return err
	}
	compiledHead, err := c.compilePredicate(head)
	if err != nil {
		return err
	}
	uninterpreted, err := source.UninterpretedTailSize()
	if err != nil {
		return err
	}
	total, err := source.TailSize()
	if err != nil {
		return err
	}
	tail := make([]ast.ASTHandle, total)
	for index := 0; index < total; index++ {
		expression, _, getErr := source.Tail(index)
		if getErr != nil {
			return getErr
		}
		if index < uninterpreted {
			tail[index], getErr = c.compilePredicate(expression)
		} else {
			tail[index], getErr = c.CompileExpression(expression)
		}
		if getErr != nil {
			return getErr
		}
	}
	name, err := source.NameSymbol()
	if err != nil {
		return err
	}
	compiledRule, err := c.rules.NewRuleWithSymbol(compiledHead, tail, nil, name)
	if err != nil {
		return err
	}
	if err = newRules.Add(compiledRule); err != nil {
		return errors.Join(err, compiledRule.Close())
	}
	if err = compiledRule.Close(); err != nil {
		return err
	}
	declaration, err := c.manager.GetDecl(head)
	if err != nil {
		return err
	}
	isOutput, err := oldRules.IsOutputPredicate(declaration)
	if err != nil || !isOutput {
		return err
	}
	compiledDeclaration, err := c.manager.GetDecl(compiledHead)
	if err != nil {
		return err
	}
	return newRules.SetOutputPredicate(compiledDeclaration)
}

func (c *Compiler) compilePredicate(application ast.ASTHandle) (ast.ASTHandle, error) {
	if compiled, found := c.cache[application]; found {
		return compiled, nil
	}
	declaration, err := c.manager.GetDecl(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if compiledDeclaration, found := c.cache[declaration]; found {
		return c.compilePredicateArguments(application, compiledDeclaration)
	}
	family, err := c.manager.GetFamilyID(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family != ast.NullFamilyID {
		return ast.ASTHandle{}, ErrCore
	}
	count, err := c.manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, count)
	for index := range domain {
		argument, e := c.manager.GetArg(application, uint32(index))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		sort, e := c.manager.GetSort(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		domain[index], e = c.compileSort(sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	name, err := c.manager.GetASTName(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compiledDeclaration, err := c.manager.MkFuncDecl(name, domain, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.rules.RegisterPredicate(compiledDeclaration); err != nil {
		return ast.ASTHandle{}, err
	}
	if _, err = c.hold(declaration, compiledDeclaration); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.compilePredicateArguments(application, compiledDeclaration)
}

func (c *Compiler) compilePredicateArguments(application, declaration ast.ASTHandle) (ast.ASTHandle, error) {
	count, err := c.manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, count)
	for index := range arguments {
		argument, e := c.manager.GetArg(application, uint32(index))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		arguments[index], e = c.CompileExpression(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	compiled, err := c.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(application, compiled)
}

// CompileExpression rewrites a source-fragment Boolean expression over the
// finite DDNF node-ID domains. Ground terms are retained unchanged.
func (c *Compiler) CompileExpression(expression ast.ASTHandle) (ast.ASTHandle, error) {
	if c == nil || c.closed || c.manager == nil || c.preprocessor == nil {
		return ast.ASTHandle{}, ErrCore
	}
	if compiled, found := c.cache[expression]; found {
		return compiled, nil
	}
	ground, err := c.preprocessor.isGround(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if ground {
		c.cache[expression] = expression
		return expression, nil
	}
	kind, err := c.manager.GetKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTVar {
		return c.compileVariable(expression)
	}
	if kind == ast.ASTQuantifier {
		return ast.ASTHandle{}, ErrCore
	}
	if kind != ast.ASTApp {
		return ast.ASTHandle{}, ErrCore
	}
	operator, err := c.manager.GetDeclKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch operator {
	case ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpIff, ast.ASTOpNot, ast.ASTOpImplies:
		return c.compileConnective(expression)
	case ast.ASTOpEq:
		if _, found := c.preprocessor.constraints[expression]; found {
			return c.compileEquality(expression)
		}
		return c.compileVariableEquality(expression)
	default:
		return ast.ASTHandle{}, ErrCore
	}
}

func (c *Compiler) compileConnective(expression ast.ASTHandle) (ast.ASTHandle, error) {
	count, err := c.manager.GetNumArgs(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, count)
	for index := range arguments {
		argument, e := c.manager.GetArg(expression, uint32(index))
		if e != nil {
			return ast.ASTHandle{}, e
		}
		arguments[index], e = c.CompileExpression(argument)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	declaration, err := c.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compiled, err := c.manager.MkApp(declaration, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(expression, compiled)
}

func (c *Compiler) compileVariableEquality(expression ast.ASTHandle) (ast.ASTHandle, error) {
	left, err := c.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := c.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftKind, err := c.manager.GetKind(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightKind, err := c.manager.GetKind(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if leftKind != ast.ASTVar || rightKind != ast.ASTVar {
		return ast.ASTHandle{}, ErrCore
	}
	compiledLeft, err := c.compileVariable(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compiledRight, err := c.compileVariable(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compiled, err := c.manager.MkEq(compiledLeft, compiledRight)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(expression, compiled)
}

func (c *Compiler) compileEquality(expression ast.ASTHandle) (ast.ASTHandle, error) {
	constraint := c.preprocessor.constraints[expression]
	width, err := c.manager.BitVectorSize(constraint.variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	core, err := c.preprocessor.registry.Get(width)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nodes, err := c.preprocessor.registry.Lookup(width, constraint.vector)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(nodes) == 0 {
		return ast.ASTHandle{}, ErrCore
	}
	variable, err := c.compileVariable(constraint.variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compactWidth, err := c.compactWidth(core)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	mark := len(c.refs)
	releaseTemps := func(cause error) error {
		return errors.Join(cause, c.releaseRefsTo(mark))
	}
	values := make([]ast.ASTHandle, len(nodes))
	for index, node := range nodes {
		numeral, e := c.bitVectorNumeral(uint64(node.ID()), compactWidth)
		if e != nil {
			return ast.ASTHandle{}, releaseTemps(e)
		}
		values[index], e = c.manager.MkEq(variable, numeral)
		if e != nil {
			return ast.ASTHandle{}, releaseTemps(e)
		}
		ref, e := c.manager.NewRef(values[index])
		if e != nil {
			return ast.ASTHandle{}, releaseTemps(e)
		}
		c.refs = append(c.refs, ref)
	}
	compiled, err := c.manager.MkOrArray(values)
	if err != nil {
		return ast.ASTHandle{}, releaseTemps(err)
	}
	// Source assigns expr_ref result before the local eqs vector dies.
	// A singleton OR is its child, so releasing eqs first destroys result.
	resultRef, err := c.manager.NewRef(compiled)
	if err != nil {
		return ast.ASTHandle{}, releaseTemps(err)
	}
	cleanupErr := c.releaseRefsTo(mark)
	// Keep the result owned even if temporary cleanup stops, so Close can
	// retry all releases without leaking this reference.
	c.refs = append(c.refs, resultRef)
	if cleanupErr != nil {
		return ast.ASTHandle{}, cleanupErr
	}
	c.cache[expression] = compiled
	return compiled, nil
}

func (c *Compiler) compileVariable(variable ast.ASTHandle) (ast.ASTHandle, error) {
	if compiled, found := c.cache[variable]; found {
		return compiled, nil
	}
	index, err := c.manager.GetVarIndex(variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	originalSort, err := c.manager.GetSort(variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compiledSort, err := c.compileSort(originalSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compiled, err := c.manager.MkVar(index, compiledSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(variable, compiled)
}

func (c *Compiler) compileSort(sort ast.ASTHandle) (ast.ASTHandle, error) {
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if sort == boolean {
		return sort, nil
	}
	width, err := c.manager.BitVectorSize(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	core, err := c.preprocessor.registry.Get(width)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	compactWidth, err := c.compactWidth(core)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.BitVectorSort(compactWidth)
}

func (c *Compiler) compactWidth(core *Core) (uint32, error) {
	if core == nil || core.Size() == 0 {
		return 0, ErrCore
	}
	count := uint64(core.Size())
	width := uint32(1)
	for width < 64 && (uint64(1)<<width) <= count {
		width++
	}
	if width >= 64 {
		return 0, arena.ErrBounds
	}
	return width, nil
}

func (c *Compiler) bitVectorNumeral(value uint64, width uint32) (ast.ASTHandle, error) {
	service := c.manager.NumeralService()
	rational, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer service.Del(&rational)
	if err = service.SetUint64(&rational, value); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.BitVectorNumeral(&rational, width)
}

func (c *Compiler) hold(key, expression ast.ASTHandle) (ast.ASTHandle, error) {
	reference, err := c.manager.NewRef(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, reference)
	c.cache[key] = expression
	return expression, nil
}

// releaseRefsTo ends a source-local expr_ref_vector. A stopped release stays
// in the compiler owner so Close can retry it.
func (c *Compiler) releaseRefsTo(mark int) error {
	if c == nil || c.closed || c.manager == nil || mark < 0 || mark > len(c.refs) {
		return ErrCore
	}
	for len(c.refs) > mark {
		index := len(c.refs) - 1
		if err := c.manager.DelRef(&c.refs[index]); err != nil {
			return err
		}
		c.refs = c.refs[:index]
	}
	return nil
}
