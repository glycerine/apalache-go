package transforms

// Source: src/muz/transforms/dl_mk_array_blast.cpp/.h retained local-store
// substitution and Ackermann select elimination.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

var errArrayBlastRefused = errors.New("muz/transforms: array blast refused")

type ArrayBlastPlugin struct {
	manager    *ast.ASTManager
	rules      *base.RuleManager
	rewriter   *astrewriter.TheoryRewriter
	simplifier *InterpTailSimplifierPlugin
	priority   uint32
	closed     bool
}

func NewArrayBlastPlugin(manager *ast.ASTManager, rules *base.RuleManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (_ *ArrayBlastPlugin, resultErr error) {
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
	simplifier, err := NewInterpTailSimplifierPlugin(manager, rules, context, globals, priority)
	if err != nil {
		return nil, errors.Join(err, rewriter.Close())
	}
	return &ArrayBlastPlugin{manager: manager, rules: rules, rewriter: rewriter, simplifier: simplifier, priority: priority}, nil
}

func (p *ArrayBlastPlugin) Priority() uint32            { return p.priority }
func (p *ArrayBlastPlugin) CanDestratifyNegation() bool { return false }
func (p *ArrayBlastPlugin) Cancel()                     {}

func (p *ArrayBlastPlugin) arrayApplication(expression ast.ASTHandle, operation int32) (bool, error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	arrayFamily, err := p.manager.ArrayFamilyID()
	if err != nil || family != arrayFamily {
		return false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	return op == operation, err
}

func (p *ArrayBlastPlugin) equality(expression ast.ASTHandle) (left, right ast.ASTHandle, yes bool, err error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return left, right, false, err
	}
	family, err := p.manager.GetFamilyID(expression)
	if err != nil || family != p.manager.BasicFamilyID() {
		return left, right, false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	if err != nil || (op != ast.ASTOpEq && op != ast.ASTOpIff) {
		return left, right, false, err
	}
	count, err := p.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return left, right, false, err
	}
	left, err = p.manager.GetArg(expression, 0)
	if err == nil {
		right, err = p.manager.GetArg(expression, 1)
	}
	return left, right, err == nil, err
}

func (p *ArrayBlastPlugin) storeDefinition(expression ast.ASTHandle) (variable, store ast.ASTHandle, yes bool, err error) {
	left, right, equal, err := p.equality(expression)
	if err != nil || !equal {
		return variable, store, false, err
	}
	rightStore, err := p.arrayApplication(right, ast.ASTArrayOpStore)
	if err != nil {
		return variable, store, false, err
	}
	if !rightStore {
		left, right = right, left
	}
	leftKind, err := p.manager.GetKind(left)
	if err != nil {
		return variable, store, false, err
	}
	rightStore, err = p.arrayApplication(right, ast.ASTArrayOpStore)
	return left, right, leftKind == ast.ASTVar && rightStore, err
}

func (p *ArrayBlastPlugin) variables(expression ast.ASTHandle) (map[uint32]bool, error) {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return nil, err
	}
	if err = used.Process(expression); err != nil {
		return nil, err
	}
	result := make(map[uint32]bool)
	for i := uint32(0); i < used.MaxFoundVarIndexPlusOne(); i++ {
		if !used.Contains(i).IsNull() {
			result[i] = true
		}
	}
	return result, nil
}

func (p *ArrayBlastPlugin) disjoint(left, right ast.ASTHandle) (bool, error) {
	lhs, err := p.variables(left)
	if err != nil {
		return false, err
	}
	rhs, err := p.variables(right)
	if err != nil {
		return false, err
	}
	for variable := range lhs {
		if rhs[variable] {
			return false, nil
		}
	}
	return true, nil
}

func (p *ArrayBlastPlugin) rewrite(expression ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	result, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = p.rewriter.Rewrite(expression, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, p.manager.DelRef(&result))
	}
	return result, nil
}

func (p *ArrayBlastPlugin) maximumVariable(rule *base.Rule) (uint32, error) {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return 0, err
	}
	head, err := rule.Head()
	if err == nil {
		err = used.Process(head)
	}
	count, x := rule.TailSize()
	if err == nil {
		err = x
	}
	for i := 0; err == nil && i < count; i++ {
		tail, _, getErr := rule.Tail(i)
		if getErr != nil {
			err = getErr
		} else {
			err = used.Process(tail)
		}
	}
	return used.MaxFoundVarIndexPlusOne(), err
}

func (p *ArrayBlastPlugin) selectBase(expression ast.ASTHandle) (ast.ASTHandle, error) {
	for {
		selectTerm, err := p.arrayApplication(expression, ast.ASTArrayOpSelect)
		if err != nil || !selectTerm {
			return expression, err
		}
		expression, err = p.manager.GetArg(expression, 0)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
}

func (p *ArrayBlastPlugin) selectIndices(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	var result []ast.ASTHandle
	for {
		selectTerm, err := p.arrayApplication(expression, ast.ASTArrayOpSelect)
		if err != nil || !selectTerm {
			return result, err
		}
		count, err := p.manager.GetNumArgs(expression)
		if err != nil {
			return nil, err
		}
		for i := uint32(1); i < count; i++ {
			index, getErr := p.manager.GetArg(expression, i)
			if getErr != nil {
				return nil, getErr
			}
			result = append(result, index)
		}
		expression, err = p.manager.GetArg(expression, 0)
		if err != nil {
			return nil, err
		}
	}
}

type arrayBlastDefinition struct {
	selectTerm ast.ASTHandle
	variable   ast.ASTHandle
}

func (p *ArrayBlastPlugin) ackermannize(rule *base.Rule, expressions []ast.ASTHandle, pins *[]ast.ASTRef) (_ []ast.ASTHandle, changed bool, resultErr error) {
	pin := func(expression ast.ASTHandle) error {
		reference, err := p.manager.NewRef(expression)
		if err == nil {
			*pins = append(*pins, reference)
		}
		return err
	}
	next, err := p.maximumVariable(rule)
	if err != nil {
		return nil, false, err
	}
	next++
	definitions := make(map[ast.ASTHandle]ast.ASTHandle)
	var order []arrayBlastDefinition
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	var rewriteExpression func(ast.ASTHandle) (ast.ASTHandle, error)
	rewriteExpression = func(expression ast.ASTHandle) (ast.ASTHandle, error) {
		if rewritten, found := cache[expression]; found {
			return rewritten, nil
		}
		kind, getErr := p.manager.GetKind(expression)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if kind == ast.ASTVar {
			cache[expression] = expression
			return expression, nil
		}
		if kind != ast.ASTApp {
			return ast.ASTHandle{}, errArrayBlastRefused
		}
		count, getErr := p.manager.GetNumArgs(expression)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		arguments := make([]ast.ASTHandle, count)
		for i := uint32(0); i < count; i++ {
			argument, x := p.manager.GetArg(expression, i)
			if x == nil {
				arguments[i], x = rewriteExpression(argument)
			}
			if x != nil {
				return ast.ASTHandle{}, x
			}
		}
		declaration, getErr := p.manager.GetDecl(expression)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		rebuilt, getErr := p.manager.MkApp(declaration, arguments)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if getErr = pin(rebuilt); getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		selectTerm, getErr := p.arrayApplication(rebuilt, ast.ASTArrayOpSelect)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if !selectTerm {
			cache[expression] = rebuilt
			return rebuilt, nil
		}
		base, getErr := p.selectBase(rebuilt)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		baseKind, getErr := p.manager.GetKind(base)
		if getErr != nil || baseKind != ast.ASTVar {
			return ast.ASTHandle{}, errors.Join(getErr, errArrayBlastRefused)
		}
		variable := definitions[rebuilt]
		if variable.IsNull() {
			sort, x := p.manager.GetSort(rebuilt)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			variable, x = p.manager.MkVar(next, sort)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			if x = pin(variable); x != nil {
				return ast.ASTHandle{}, x
			}
			next++
			definitions[rebuilt] = variable
			order = append(order, arrayBlastDefinition{selectTerm: rebuilt, variable: variable})
		}
		cache[expression] = variable
		return variable, nil
	}

	result := make([]ast.ASTHandle, 0, len(expressions))
	for _, expression := range expressions {
		left, right, equal, getErr := p.equality(expression)
		if getErr != nil {
			return nil, false, getErr
		}
		var selected, variable ast.ASTHandle
		if equal {
			leftSelect, x := p.arrayApplication(left, ast.ASTArrayOpSelect)
			if x != nil {
				return nil, false, x
			}
			if !leftSelect {
				left, right = right, left
				leftSelect, x = p.arrayApplication(left, ast.ASTArrayOpSelect)
				if x != nil {
					return nil, false, x
				}
			}
			rightKind, x := p.manager.GetKind(right)
			if x != nil {
				return nil, false, x
			}
			if leftSelect && rightKind == ast.ASTVar {
				selected, variable = left, right
			}
		}
		if selected.IsNull() {
			rewritten, x := rewriteExpression(expression)
			if x != nil {
				return nil, false, x
			}
			result = append(result, rewritten)
			continue
		}
		count, getErr := p.manager.GetNumArgs(selected)
		if getErr != nil {
			return nil, false, getErr
		}
		arguments := make([]ast.ASTHandle, count)
		for i := uint32(0); i < count; i++ {
			argument, x := p.manager.GetArg(selected, i)
			if x == nil {
				arguments[i], x = rewriteExpression(argument)
			}
			if x != nil {
				return nil, false, x
			}
		}
		declaration, getErr := p.manager.GetDecl(selected)
		if getErr != nil {
			return nil, false, getErr
		}
		selected, getErr = p.manager.MkApp(declaration, arguments)
		if getErr != nil {
			return nil, false, getErr
		}
		if getErr = pin(selected); getErr != nil {
			return nil, false, getErr
		}
		definition := definitions[selected]
		if definition.IsNull() {
			base, x := p.selectBase(selected)
			if x != nil {
				return nil, false, x
			}
			baseKind, x := p.manager.GetKind(base)
			if x != nil || baseKind != ast.ASTVar {
				return nil, false, errors.Join(x, errArrayBlastRefused)
			}
			definitions[selected] = variable
			definition = variable
			order = append(order, arrayBlastDefinition{selectTerm: selected, variable: variable})
		}
		equality, getErr := p.manager.MkEq(variable, definition)
		if getErr != nil {
			return nil, false, getErr
		}
		if getErr = pin(equality); getErr != nil {
			return nil, false, getErr
		}
		result = append(result, equality)
	}
	for i := 0; i < len(order); i++ {
		leftBase, getErr := p.selectBase(order[i].selectTerm)
		if getErr != nil {
			return nil, false, getErr
		}
		leftIndices, getErr := p.selectIndices(order[i].selectTerm)
		if getErr != nil {
			return nil, false, getErr
		}
		for j := i + 1; j < len(order); j++ {
			rightBase, x := p.selectBase(order[j].selectTerm)
			if x != nil {
				return nil, false, x
			}
			if leftBase != rightBase {
				continue
			}
			rightIndices, x := p.selectIndices(order[j].selectTerm)
			if x != nil || len(leftIndices) != len(rightIndices) {
				return nil, false, errors.Join(x, ast.ErrASTDomain)
			}
			indicesEqual := make([]ast.ASTHandle, len(leftIndices))
			for index := range leftIndices {
				indicesEqual[index], x = p.manager.MkEq(leftIndices[index], rightIndices[index])
				if x != nil {
					return nil, false, x
				}
				if x = pin(indicesEqual[index]); x != nil {
					return nil, false, x
				}
			}
			antecedent, x := p.manager.MkAndArgs(indicesEqual)
			if x != nil {
				return nil, false, x
			}
			if x = pin(antecedent); x != nil {
				return nil, false, x
			}
			valuesEqual, x := p.manager.MkEq(order[i].variable, order[j].variable)
			if x != nil {
				return nil, false, x
			}
			if x = pin(valuesEqual); x != nil {
				return nil, false, x
			}
			implication, x := p.manager.MkImplies(antecedent, valuesEqual)
			if x != nil {
				return nil, false, x
			}
			if x = pin(implication); x != nil {
				return nil, false, x
			}
			result = append(result, implication)
		}
	}
	return result, true, nil
}

func (p *ArrayBlastPlugin) transformRule(rule *base.Rule) (_ *base.Rule, changed bool, resultErr error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, false, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, false, err
	}
	conjuncts := make([]ast.ASTHandle, 0)
	for i := uninterpreted; i < total; i++ {
		tail, _, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, false, getErr
		}
		flattened, getErr := p.manager.FlattenAnd([]ast.ASTHandle{tail})
		if getErr != nil {
			return nil, false, getErr
		}
		conjuncts = append(conjuncts, flattened...)
	}
	replace, err := astrewriter.NewExprSafeReplace(p.manager)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, replace.Close()) }()
	var retained []ast.ASTHandle
	inserted := false
	for _, conjunct := range conjuncts {
		variable, store, definition, getErr := p.storeDefinition(conjunct)
		if getErr != nil {
			return nil, false, getErr
		}
		if definition {
			disjoint, x := p.disjoint(variable, store)
			if x != nil {
				return nil, false, x
			}
			if disjoint {
				if x = replace.Insert(variable, store); x != nil {
					return nil, false, x
				}
				inserted = true
				continue
			}
		}
		retained = append(retained, conjunct)
	}
	if !inserted {
		return nil, false, nil
	}
	head, err := rule.Head()
	if err != nil {
		return nil, false, err
	}
	headRef, err := p.manager.NewRef(head)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&headRef)) }()
	if err = replace.ApplyRef(headRef); err != nil {
		return nil, false, err
	}
	head, err = p.manager.RefGet(headRef)
	if err != nil {
		return nil, false, err
	}
	rewrittenHead, err := p.rewrite(head)
	if err != nil {
		return nil, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&rewrittenHead)) }()
	head, err = p.manager.RefGet(rewrittenHead)
	if err != nil {
		return nil, false, err
	}
	headKind, err := p.manager.GetKind(head)
	if err != nil || headKind != ast.ASTApp {
		return nil, false, errors.Join(err, ast.ErrASTDomain)
	}
	headCount, err := p.manager.GetNumArgs(head)
	if err != nil {
		return nil, false, err
	}
	for i := uint32(0); i < headCount; i++ {
		argument, x := p.manager.GetArg(head, i)
		if x != nil {
			return nil, false, x
		}
		kind, x := p.manager.GetKind(argument)
		if x != nil {
			return nil, false, x
		}
		value, x := p.manager.IsValue(argument)
		if x != nil || (kind != ast.ASTVar && !value) {
			return nil, false, x
		}
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	var rewrittenRefs []ast.ASTRef
	defer func() {
		for index := len(rewrittenRefs) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, p.manager.DelRef(&rewrittenRefs[index]))
		}
	}()
	for i := 0; i < uninterpreted; i++ {
		tail, negative, getErr := rule.Tail(i)
		if getErr != nil {
			return nil, false, getErr
		}
		ref, getErr := p.manager.NewRef(tail)
		if getErr != nil {
			return nil, false, getErr
		}
		if getErr = replace.ApplyRef(ref); getErr == nil {
			tail, getErr = p.manager.RefGet(ref)
		}
		var rewritten ast.ASTRef
		if getErr == nil {
			rewritten, getErr = p.rewrite(tail)
		}
		if getErr == nil {
			tail, getErr = p.manager.RefGet(rewritten)
		}
		if rewritten != (ast.ASTRef{}) {
			rewrittenRefs = append(rewrittenRefs, rewritten)
		}
		getErr = errors.Join(getErr, p.manager.DelRef(&ref))
		if getErr != nil {
			return nil, false, getErr
		}
		tails = append(tails, tail)
		negated = append(negated, negative)
	}
	for _, expression := range retained {
		ref, getErr := p.manager.NewRef(expression)
		if getErr != nil {
			return nil, false, getErr
		}
		if getErr = replace.ApplyRef(ref); getErr == nil {
			expression, getErr = p.manager.RefGet(ref)
		}
		var rewritten ast.ASTRef
		if getErr == nil {
			rewritten, getErr = p.rewrite(expression)
		}
		if getErr == nil {
			expression, getErr = p.manager.RefGet(rewritten)
		}
		if rewritten != (ast.ASTRef{}) {
			rewrittenRefs = append(rewrittenRefs, rewritten)
		}
		getErr = errors.Join(getErr, p.manager.DelRef(&ref))
		if getErr != nil {
			return nil, false, getErr
		}
		tails = append(tails, expression)
		negated = append(negated, false)
	}
	tails, ackermannized, err := p.ackermannize(rule, tails, &rewrittenRefs)
	if errors.Is(err, errArrayBlastRefused) {
		return nil, false, nil
	}
	if err != nil || !ackermannized {
		return nil, false, err
	}
	if len(tails) > uninterpreted {
		combined, combineErr := p.manager.MkAndArgs(tails[uninterpreted:])
		if combineErr != nil {
			return nil, false, combineErr
		}
		kind, combineErr := p.manager.GetKind(combined)
		if combineErr != nil {
			return nil, false, combineErr
		}
		if kind != ast.ASTApp {
			truth, makeErr := p.manager.MkTrue()
			if makeErr != nil {
				return nil, false, makeErr
			}
			combined, combineErr = p.manager.MkEq(combined, truth)
			if combineErr != nil {
				return nil, false, combineErr
			}
		}
		combinedRef, combineErr := p.manager.NewRef(combined)
		if combineErr != nil {
			return nil, false, combineErr
		}
		rewrittenRefs = append(rewrittenRefs, combinedRef)
		tails = append(tails[:uninterpreted], combined)
		negated = append(negated[:uninterpreted], false)
	} else {
		negated = negated[:uninterpreted]
	}
	name, err := rule.Name()
	if err != nil {
		return nil, false, err
	}
	for index, tail := range tails {
		kind, kindErr := p.manager.GetKind(tail)
		if kindErr != nil || kind != ast.ASTApp {
			return nil, false, fmt.Errorf("muz/transforms: array blast tail %d is not an application: %w", index, errors.Join(kindErr, ast.ErrASTDomain))
		}
	}
	replacement, err := p.rules.NewRule(head, tails, negated, name)
	if err != nil {
		return nil, false, err
	}
	simplified, dropped, simplifiedChanged, err := p.simplifier.transformRule(replacement)
	if err != nil {
		return nil, false, errors.Join(err, replacement.Close())
	}
	if dropped {
		return nil, true, replacement.Close()
	}
	if simplifiedChanged {
		return simplified, true, replacement.Close()
	}
	return replacement, true, nil
}

func (p *ArrayBlastPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
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
	modified := false
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		replacement, changed, transformErr := p.transformRule(rule)
		if transformErr == nil {
			candidate := rule
			if changed && replacement != nil {
				candidate = replacement
			}
			if replacement != nil || !changed {
				transformErr = result.Add(candidate)
			}
			modified = modified || changed
		}
		transformErr = errors.Join(transformErr, rule.Close())
		if replacement != nil {
			transformErr = errors.Join(transformErr, replacement.Close())
		}
		if transformErr != nil {
			return nil, fmt.Errorf("muz/transforms: array blast rule %d: %w", i, transformErr)
		}
	}
	if !modified {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *ArrayBlastPlugin) Close() error {
	if p == nil {
		return base.ErrRuleTransformerOwner
	}
	if p.closed {
		return nil
	}
	p.closed = true
	return errors.Join(p.simplifier.Close(), p.rewriter.Close())
}
