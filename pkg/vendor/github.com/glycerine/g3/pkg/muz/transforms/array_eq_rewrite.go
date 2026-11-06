package transforms

// Source: src/muz/transforms/dl_mk_array_eq_rewrite.cpp/.h array-equivalence
// representative selection and select-over-store rewriting.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type ArrayEqRewritePlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	priority uint32
	closed   bool
}

func NewArrayEqRewritePlugin(manager *ast.ASTManager, rules *base.RuleManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (_ *ArrayEqRewritePlugin, resultErr error) {
	if manager == nil || rules == nil || context == nil || globals == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	return &ArrayEqRewritePlugin{manager: manager, rules: rules, context: context, globals: globals, priority: priority}, nil
}

func (p *ArrayEqRewritePlugin) Priority() uint32            { return p.priority }
func (p *ArrayEqRewritePlugin) CanDestratifyNegation() bool { return false }
func (p *ArrayEqRewritePlugin) Cancel()                     {}

type arrayEqClass struct {
	members []ast.ASTHandle
	ids     []uint32
	root    uint32
}

func mergeArrayEq(classes *[]arrayEqClass, ids map[ast.ASTHandle]uint32, nextID *uint32, left, right ast.ASTHandle) {
	leftID, leftKnown := ids[left]
	if !leftKnown {
		leftID = *nextID
		*nextID = *nextID + 1
		ids[left] = leftID
	}
	rightID, rightKnown := ids[right]
	if !rightKnown {
		rightID = *nextID
		*nextID = *nextID + 1
		ids[right] = rightID
	}
	leftClass, rightClass := -1, -1
	for i := range *classes {
		for _, id := range (*classes)[i].ids {
			if id == leftID {
				leftClass = i
			}
			if id == rightID {
				rightClass = i
			}
		}
	}
	if leftClass < 0 && rightClass < 0 {
		if leftID == rightID {
			*classes = append(*classes, arrayEqClass{
				members: []ast.ASTHandle{left},
				ids:     []uint32{leftID},
				root:    leftID,
			})
			return
		}
		*classes = append(*classes, arrayEqClass{
			members: []ast.ASTHandle{right, left},
			ids:     []uint32{rightID, leftID},
			root:    rightID,
		})
		return
	}
	if leftClass < 0 {
		class := (*classes)[rightClass]
		class.members = append(append(class.members[:1:1], left), class.members[1:]...)
		class.ids = append(append(class.ids[:1:1], leftID), class.ids[1:]...)
		(*classes)[rightClass] = class
		return
	}
	if rightClass < 0 {
		class := (*classes)[leftClass]
		if len(class.members) == 1 {
			class.members = []ast.ASTHandle{right, class.members[0]}
			class.ids = []uint32{rightID, class.ids[0]}
			class.root = rightID
		} else {
			class.members = append(append(class.members[:1:1], right), class.members[1:]...)
			class.ids = append(append(class.ids[:1:1], rightID), class.ids[1:]...)
		}
		(*classes)[leftClass] = class
		return
	}
	if leftClass == rightClass {
		return
	}
	leftGroup, rightGroup := (*classes)[leftClass], (*classes)[rightClass]
	var merged arrayEqClass
	if len(leftGroup.members) > len(rightGroup.members) {
		merged = arrayEqClass{
			members: append(append([]ast.ASTHandle{leftGroup.members[0]}, rightGroup.members[1:]...), rightGroup.members[0]),
			ids:     append(append([]uint32{leftGroup.ids[0]}, rightGroup.ids[1:]...), rightGroup.ids[0]),
			root:    leftGroup.root,
		}
		merged.members = append(merged.members, leftGroup.members[1:]...)
		merged.ids = append(merged.ids, leftGroup.ids[1:]...)
	} else {
		merged = arrayEqClass{
			members: append(append([]ast.ASTHandle{rightGroup.members[0]}, leftGroup.members[1:]...), leftGroup.members[0]),
			ids:     append(append([]uint32{rightGroup.ids[0]}, leftGroup.ids[1:]...), leftGroup.ids[0]),
			root:    rightGroup.root,
		}
		merged.members = append(merged.members, rightGroup.members[1:]...)
		merged.ids = append(merged.ids, rightGroup.ids[1:]...)
	}
	(*classes)[leftClass] = merged
	if rightClass > leftClass {
		*classes = append((*classes)[:rightClass], (*classes)[rightClass+1:]...)
	} else {
		*classes = append((*classes)[:rightClass], (*classes)[rightClass+1:]...)
		leftClass--
		(*classes)[leftClass] = merged
	}
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
	classIDs := make(map[ast.ASTHandle]uint32)
	var nextClassID uint32
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
			mergeArrayEq(&classes, classIDs, &nextClassID, left, right)
		} else {
			tails, negated = append(tails, tail), append(negated, false)
		}
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i].root < classes[j].root })
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
	local, err := p.context.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.context.Del(&local)) }()
	optionName := "expand_select_store"
	if err = p.context.SetBoolName(&local, &optionName, true); err != nil {
		return nil, err
	}
	rewriter, err := astrewriter.NewTheoryRewriter(p.manager, p.context, p.globals, &local)
	if err != nil {
		if rewriter != nil {
			err = errors.Join(err, rewriter.Close())
		}
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rewriter.Close()) }()
	// The C++ transform runs th_rewriter over every retained tail, including
	// predicate applications whose arguments contain interpreted array terms.
	for i := 0; i < len(tails); i++ {
		ref, makeErr := p.manager.NewRef(ast.ASTHandle{})
		if makeErr != nil {
			return nil, makeErr
		}
		owned = append(owned, ref)
		if makeErr = rewriter.Rewrite(tails[i], ref); makeErr != nil {
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
	name, err := rule.NameSymbol()
	if err != nil {
		return nil, err
	}
	return p.rules.NewRuleWithSymbol(head, tails, negated, name)
}

func (p *ArrayEqRewritePlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil || p.context == nil || p.globals == nil {
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
	return nil
}
