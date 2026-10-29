package transforms

// Source: src/muz/transforms/dl_mk_array_eq_rewrite.cpp/.h array-equivalence
// representative selection and select-over-store rewriting.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type ArrayEqRewritePlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	rewriter *astrewriter.TheoryRewriter
	priority uint32
	closed   bool
}

func NewArrayEqRewritePlugin(manager *ast.ASTManager, rules *base.RuleManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (_ *ArrayEqRewritePlugin, resultErr error) {
	if manager == nil || rules == nil || context == nil || globals == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	local, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Del(&local)) }()
	name := "expand_select_store"
	if err = context.SetBoolName(&local, &name, true); err != nil {
		return nil, err
	}
	rewriter, err := astrewriter.NewTheoryRewriter(manager, context, globals, &local)
	if err != nil {
		return nil, err
	}
	return &ArrayEqRewritePlugin{manager: manager, rules: rules, rewriter: rewriter, priority: priority}, nil
}

func (p *ArrayEqRewritePlugin) Priority() uint32            { return p.priority }
func (p *ArrayEqRewritePlugin) CanDestratifyNegation() bool { return false }
func (p *ArrayEqRewritePlugin) Cancel()                     {}

type arrayEqClass struct {
	members []ast.ASTHandle
}

func mergeArrayEq(classes *[]arrayEqClass, left, right ast.ASTHandle) {
	leftClass, rightClass := -1, -1
	for i := range *classes {
		for _, member := range (*classes)[i].members {
			if member == left {
				leftClass = i
			}
			if member == right {
				rightClass = i
			}
		}
	}
	if leftClass < 0 && rightClass < 0 {
		*classes = append(*classes, arrayEqClass{members: []ast.ASTHandle{left, right}})
		return
	}
	if leftClass < 0 {
		(*classes)[rightClass].members = append((*classes)[rightClass].members, left)
		return
	}
	if rightClass < 0 {
		(*classes)[leftClass].members = append((*classes)[leftClass].members, right)
		return
	}
	if leftClass == rightClass {
		return
	}
	(*classes)[leftClass].members = append((*classes)[leftClass].members, (*classes)[rightClass].members...)
	*classes = append((*classes)[:rightClass], (*classes)[rightClass+1:]...)
}

func (p *ArrayEqRewritePlugin) arrayEquality(expression ast.ASTHandle) (left, right ast.ASTHandle, yes bool, err error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return left, right, false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil || family != p.manager.BasicFamilyID() {
		return left, right, false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	if err != nil || op != ast.ASTOpEq {
		return left, right, false, err
	}
	count, err := p.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return left, right, false, err
	}
	left, err = p.manager.GetArg(expression, 0)
	if err != nil {
		return left, right, false, err
	}
	right, err = p.manager.GetArg(expression, 1)
	if err != nil {
		return left, right, false, err
	}
	sort, err := p.manager.GetSort(left)
	if err != nil {
		return left, right, false, err
	}
	family, err = p.manager.GetFamilyID(sort)
	if err != nil {
		return left, right, false, err
	}
	arrayFamily, err := p.manager.ArrayFamilyID()
	return left, right, family == arrayFamily, err
}

// replaceApp follows the source helper: it recursively rebuilds applications
// but deliberately does not descend through variables or quantifiers.
func (p *ArrayEqRewritePlugin) replaceApp(expression, oldValue, newValue ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	if expression == oldValue {
		return p.manager.NewRef(newValue)
	}
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		if err != nil {
			return ast.ASTRef{}, err
		}
		return p.manager.NewRef(expression)
	}
	count, err := p.manager.GetNumArgs(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	args := make([]ast.ASTHandle, count)
	refs := make([]ast.ASTRef, 0, count)
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&refs[i]))
		}
	}()
	for i := uint32(0); i < count; i++ {
		argument, getErr := p.manager.GetArg(expression, i)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		ref, getErr := p.replaceApp(argument, oldValue, newValue)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
		refs = append(refs, ref)
		args[i], getErr = p.manager.RefGet(ref)
		if getErr != nil {
			return ast.ASTRef{}, getErr
		}
	}
	declaration, err := p.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTRef{}, err
	}
	rebuilt, err := p.manager.MkApp(declaration, args)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.manager.NewRef(rebuilt)
}

func (p *ArrayEqRewritePlugin) transformRule(rule *base.Rule) (_ *base.Rule, resultErr error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for i := 0; i < uninterpreted; i++ {
		tail, negative, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, getErr
		}
		tails, negated = append(tails, tail), append(negated, negative)
	}
	var classes []arrayEqClass
	for i := uninterpreted; i < total; i++ {
		tail, _, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, getErr
		}
		left, right, equality, getErr := p.arrayEquality(tail)
		if getErr != nil {
			return nil, getErr
		}
		if equality {
			mergeArrayEq(&classes, left, right)
		} else {
			tails, negated = append(tails, tail), append(negated, false)
		}
	}
	owned := make([]ast.ASTRef, 0)
	defer func() {
		for i := len(owned) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&owned[i]))
		}
	}()
	for _, class := range classes {
		representative := class.members[0]
		for _, member := range class.members {
			kind, getErr := p.manager.GetKind(member)
			if getErr != nil {
				return nil, getErr
			}
			if kind != ast.ASTVar {
				representative = member
				break
			}
		}
		for _, member := range class.members {
			for i := range tails {
				ref, replaceErr := p.replaceApp(tails[i], member, representative)
				if replaceErr != nil {
					return nil, replaceErr
				}
				owned = append(owned, ref)
				tails[i], replaceErr = p.manager.RefGet(ref)
				if replaceErr != nil {
					return nil, replaceErr
				}
			}
		}
		for _, member := range class.members {
			equality, makeErr := p.manager.MkEq(member, representative)
			if makeErr != nil {
				return nil, makeErr
			}
			tails, negated = append(tails, equality), append(negated, false)
		}
	}
	for i := uninterpreted; i < len(tails); i++ {
		ref, makeErr := p.manager.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return nil, makeErr
		}
		owned = append(owned, ref)
		if makeErr = p.rewriter.Rewrite(tails[i], ref); makeErr != nil {
			return nil, makeErr
		}
		tails[i], makeErr = p.manager.RefGet(ref)
		if makeErr != nil {
			return nil, makeErr
		}
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	name, err := rule.Name()
	if err != nil {
		return nil, err
	}
	return p.rules.NewRule(head, tails, negated, name)
}

func (p *ArrayEqRewritePlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil || p.rewriter == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		replacement, transformErr := p.transformRule(rule)
		transformErr = errors.Join(transformErr, rule.Close())
		if transformErr == nil {
			transformErr = result.Add(replacement)
		}
		if replacement != nil {
			transformErr = errors.Join(transformErr, replacement.Close())
		}
		if transformErr != nil {
			return nil, transformErr
		}
	}
	keep = true
	return result, nil
}

func (p *ArrayEqRewritePlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	if p.closed {
		return nil
	}
	p.closed = true
	err := p.rewriter.Close()
	p.rewriter = nil
	return err
}
