package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, nonlinear::compile.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

// ruleOwnerCleanupFailure keeps concrete retained rule owners reachable when
// native AST cleanup is refused. It implements memory.ConstructionFailure
// structurally without introducing a generic ownership adapter.
type ruleOwnerCleanupFailure struct {
	manager    *ast.ASTManager
	cause      error
	cleanupErr error
	reported   error
	reportText string
	rules      []*base.Rule
	child      memory.ConstructionFailure
}

type astRefCleanupFailure struct {
	manager    *ast.ASTManager
	refs       []ast.ASTRef
	cause      error
	cleanupErr error
	reported   error
	reportText string
	child      memory.ConstructionFailure
}

func (f *astRefCleanupFailure) Error() string { return f.reportText }
func (f *astRefCleanupFailure) Unwrap() error { return f.reported }
func (f *astRefCleanupFailure) Cleanup() error {
	if f.child != nil {
		if f.cleanupErr = f.child.Cleanup(); f.cleanupErr != nil {
			return f.cleanupErr
		}
		f.child = nil
	}
	f.cleanupErr = closeASTRefs(f.manager, f.refs)
	return f.cleanupErr
}
func (f *astRefCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.manager.NumeralService().RuntimeOwner()
}
func (f *astRefCleanupFailure) OriginalCause() error { return f.cause }
func (f *astRefCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func closeASTRefs(manager *ast.ASTManager, refs []ast.ASTRef) (result error) {
	for i := len(refs) - 1; i >= 0; i-- {
		if refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := manager.DelRef(&refs[i]); err != nil {
			return errors.Join(result, err)
		}
	}
	return result
}

func disposeASTRefs(manager *ast.ASTManager, refs []ast.ASTRef, cause error) error {
	failure := &astRefCleanupFailure{manager: manager, refs: refs, cause: cause}
	if cause != nil {
		// These refs are constructed after the nested operation's roots. Drain
		// its retry receipt before touching the dependent outer frontier.
		errors.As(cause, &failure.child)
	}
	if cleanup := failure.Cleanup(); cleanup != nil {
		reported := cleanup
		if cause == nil {
			// Keep the refusal as the stable originating cause after a later
			// successful retry clears cleanupErr.
			failure.cause = cleanup
		} else {
			reported = errors.Join(cause, cleanup)
		}
		failure.reported = reported
		failure.reportText = reported.Error()
		return failure
	}
	return cause
}

func (f *ruleOwnerCleanupFailure) Error() string { return f.reportText }
func (f *ruleOwnerCleanupFailure) Unwrap() error { return f.reported }
func (f *ruleOwnerCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.manager.NumeralService().RuntimeOwner()
}
func (f *ruleOwnerCleanupFailure) OriginalCause() error { return f.cause }
func (f *ruleOwnerCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (f *ruleOwnerCleanupFailure) Cleanup() error {
	if f == nil {
		return nil
	}
	if f.child != nil {
		if f.cleanupErr = f.child.Cleanup(); f.cleanupErr != nil {
			return f.cleanupErr
		}
		f.child = nil
	}
	var pending []*base.Rule
	var cause error
	for i := len(f.rules) - 1; i >= 0; i-- {
		if err := f.rules[i].Close(); err != nil {
			pending = append(pending, f.rules[i])
			cause = errors.Join(cause, err)
		}
	}
	// The scan above is newest-to-oldest. Store pending owners in their
	// original order so every retry again closes newest-to-oldest.
	for left, right := 0, len(pending)-1; left < right; left, right = left+1, right-1 {
		pending[left], pending[right] = pending[right], pending[left]
	}
	f.rules = pending
	f.cleanupErr = cause
	if cause != nil {
		return cause
	}
	return nil
}

func closeRuleOwners(manager *ast.ASTManager, rules []*base.Rule) error {
	return disposeRuleOwners(manager, rules, nil)
}

func disposeRuleOwners(manager *ast.ASTManager, rules []*base.Rule, cause error) error {
	failure := &ruleOwnerCleanupFailure{manager: manager, rules: rules, cause: cause}
	if cause != nil {
		// Rule owners may depend on AST roots held by the nested operation.
		// Adopt that receipt before the first cleanup attempt so dependencies
		// always drain before retained rules.
		errors.As(cause, &failure.child)
	}
	if cleanup := failure.Cleanup(); cleanup != nil {
		reported := cleanup
		if cause == nil {
			failure.cause = cleanup
		} else {
			reported = errors.Join(cause, cleanup)
		}
		failure.cleanupErr = cleanup
		failure.reported = reported
		failure.reportText = reported.Error()
		return failure
	}
	return cause
}

func (c *Compiler) simplifyBoolean(and bool, args []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m := c.manager
	var decl ast.ASTHandle
	var err error
	if and {
		decl, err = m.MkAndDecl()
	} else {
		decl, err = m.MkOrDecl()
	}
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	if err = rewriter.SimplifyBooleanApplicationDefault(m, decl, args, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}

func (c *Compiler) applyVariables(decl ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	m := c.manager
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = disposeASTRefs(m, []ast.ASTRef{result}, resultErr)
			result = ast.ASTRef{}
		}
	}()
	count, err := m.GetArity(decl)
	if err != nil {
		return result, err
	}
	args := make([]ast.ASTHandle, count)
	var refs []ast.ASTRef
	defer func() {
		resultErr = disposeASTRefs(m, refs, resultErr)
	}()
	for i := range args {
		sort, err := m.GetDomain(decl, uint32(i))
		if err != nil {
			return result, err
		}
		args[i], err = m.MkVar(uint32(i), sort)
		if err != nil {
			return result, err
		}
		ref, err := m.NewRef(args[i])
		if err != nil {
			return result, err
		}
		refs = append(refs, ref)
	}
	value, err := m.MkApp(decl, args)
	if err != nil {
		return result, err
	}
	return m.NewRef(value)
}

func (c *Compiler) compileRule(manager *base.RuleManager, predicate ast.ASTHandle, rule *base.Rule, index, level uint32) (result ast.ASTRef, resultErr error) {
	m := c.manager
	var refs []ast.ASTRef
	defer func() {
		resultErr = disposeASTRefs(m, refs, resultErr)
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = disposeASTRefs(m, []ast.ASTRef{result}, resultErr)
			result = ast.ASTRef{}
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		ref, err := m.NewRef(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return value, nil
	}
	ruleRef, err := c.levelDeclaration(predicate, level, &index)
	if err != nil {
		return result, err
	}
	refs = append(refs, ruleRef)
	ruleDecl, err := m.RefGet(ruleRef)
	if err != nil {
		return result, err
	}
	head, err := rule.Head()
	if err != nil {
		return result, err
	}
	variableSorts, err := rule.VariableSorts()
	if err != nil {
		return result, err
	}
	arity, err := m.GetArity(predicate)
	if err != nil {
		return result, err
	}
	args := make([]ast.ASTHandle, arity)
	seen := make(map[ast.ASTHandle]struct{})
	var conditions []ast.ASTHandle
	var fresh uint32
	for i := range args {
		arg, err := m.GetArg(head, uint32(i))
		if err != nil {
			return result, err
		}
		kind, err := m.GetKind(arg)
		if err != nil {
			return result, err
		}
		_, duplicate := seen[arg]
		if kind == ast.ASTVar && !duplicate {
			seen[arg] = struct{}{}
			args[i] = arg
			position, err := m.GetVarIndex(arg)
			if err != nil || int(position) >= len(variableSorts) {
				return result, errors.Join(ast.ErrASTDomain, err)
			}
			variableSorts[position] = ast.ASTHandle{}
		} else {
			sort, err := m.GetSort(arg)
			if err != nil {
				return result, err
			}
			args[i], err = hold(m.MkVar(uint32(len(variableSorts))+fresh, sort))
			if err != nil {
				return result, err
			}
			fresh++
			eq, err := hold(m.MkEq(args[i], arg))
			if err != nil {
				return result, err
			}
			conditions = append(conditions, eq)
		}
	}
	ruleHead, err := hold(m.MkApp(ruleDecl, args))
	if err != nil {
		return result, err
	}
	tailCount, err := rule.TailSize()
	if err != nil {
		return result, err
	}
	for i := 0; i < tailCount; i++ {
		item, _, err := rule.Tail(i)
		if err != nil {
			return result, err
		}
		conditions = append(conditions, item)
	}
	bodyRef, err := c.simplifyBoolean(true, conditions)
	if err != nil {
		return result, err
	}
	refs = append(refs, bodyRef)
	body, err := m.RefGet(bodyRef)
	if err != nil {
		return result, err
	}
	levelBody, err := c.ReplacePredicates(manager, body, level)
	if err != nil {
		return result, err
	}
	refs = append(refs, levelBody)
	body, err = m.RefGet(levelBody)
	if err != nil {
		return result, err
	}
	predicateName, err := m.GetASTName(predicate)
	if err != nil {
		return result, err
	}
	predicateText, err := m.Symbols.Display(predicateName)
	if err != nil {
		return result, err
	}
	argSorts := make([]ast.ASTHandle, len(args))
	for i, arg := range args {
		argSorts[i], err = m.GetSort(arg)
		if err != nil {
			return result, err
		}
	}
	bindings := make([]ast.ASTHandle, len(variableSorts))
	for i, sort := range variableSorts {
		if sort.IsNull() {
			continue
		}
		name, err := m.Symbols.Intern(fmt.Sprintf("%s@%d", predicateText, i))
		if err != nil {
			return result, err
		}
		decl, err := m.MkFuncDecl(name, argSorts, sort, ast.ASTFuncDeclInfo{})
		if err != nil {
			return result, err
		}
		bindings[i], err = hold(m.MkApp(decl, args))
		if err != nil {
			return result, err
		}
	}
	sub, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		if sub != nil {
			err = errors.Join(err, sub.Close())
		}
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, sub.Close()) }()
	subRef, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	refs = append(refs, subRef)
	if err = sub.Substitute(body, bindings, subRef); err != nil {
		return result, err
	}
	body, err = m.RefGet(subRef)
	if err != nil {
		return result, err
	}
	implication, err := hold(m.MkImplies(ruleHead, body))
	if err != nil {
		return result, err
	}
	return c.BindVariables(implication, ruleHead)
}

// Compile builds the source nonlinear implications for one unfolding level.
// Returned references are owned by the caller.
func (c *Compiler) Compile(rules *base.RuleSet, level uint32) (result []ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rules == nil {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	defer func() {
		if resultErr != nil {
			resultErr = disposeASTRefs(m, result, resultErr)
			result = nil
		}
	}()
	manager, err := rules.RuleManager()
	if err != nil {
		return result, err
	}
	count, err := rules.Len()
	if err != nil {
		return result, err
	}
	var predicates []ast.ASTHandle
	seen := make(map[ast.ASTHandle]struct{})
	for i := 0; i < count; i++ {
		rule, err := rules.Rule(i)
		if err != nil {
			return result, err
		}
		decl, e := rule.HeadDeclaration()
		e = errors.Join(e, closeRuleOwners(m, []*base.Rule{rule}))
		if e != nil {
			return result, e
		}
		if _, found := seen[decl]; !found {
			seen[decl] = struct{}{}
			predicates = append(predicates, decl)
		}
	}
	for _, predicate := range predicates {
		if err := c.compileGroup(rules, manager, predicate, level, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (c *Compiler) compileGroup(rules *base.RuleSet, manager *base.RuleManager, predicate ast.ASTHandle, level uint32, result *[]ast.ASTRef) (resultErr error) {
	m := c.manager
	group, err := rules.PredicateRules(predicate)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, closeRuleOwners(m, group))
	}()
	var refs []ast.ASTRef
	defer func() {
		resultErr = disposeASTRefs(m, refs, resultErr)
	}()
	levelRef, err := c.levelDeclaration(predicate, level, nil)
	if err != nil {
		return err
	}
	refs = append(refs, levelRef)
	levelDecl, err := m.RefGet(levelRef)
	if err != nil {
		return err
	}
	var alternatives []ast.ASTHandle
	for i, rule := range group {
		index := uint32(i)
		ruleRef, err := c.levelDeclaration(predicate, level, &index)
		if err != nil {
			return err
		}
		refs = append(refs, ruleRef)
		ruleDecl, err := m.RefGet(ruleRef)
		if err != nil {
			return err
		}
		application, err := c.applyVariables(ruleDecl)
		if err != nil {
			return err
		}
		refs = append(refs, application)
		value, err := m.RefGet(application)
		if err != nil {
			return err
		}
		alternatives = append(alternatives, value)
		formula, err := c.compileRule(manager, predicate, rule, index, level)
		if err != nil {
			return err
		}
		*result = append(*result, formula)
	}
	bodyRef, err := c.simplifyBoolean(false, alternatives)
	if err != nil {
		return err
	}
	refs = append(refs, bodyRef)
	body, err := m.RefGet(bodyRef)
	if err != nil {
		return err
	}
	headRef, err := c.applyVariables(levelDecl)
	if err != nil {
		return err
	}
	refs = append(refs, headRef)
	head, err := m.RefGet(headRef)
	if err != nil {
		return err
	}
	implication, err := m.MkImplies(head, body)
	if err != nil {
		return err
	}
	impRef, err := m.NewRef(implication)
	if err != nil {
		return err
	}
	refs = append(refs, impRef)
	formula, err := c.BindVariables(implication, head)
	if err != nil {
		return err
	}
	*result = append(*result, formula)
	return nil
}
