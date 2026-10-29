package transforms

// Source: src/muz/transforms/dl_mk_slice.cpp/.h semantic predicate slicing.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type RuleSlicePlugin struct {
	manager    *ast.ASTManager
	rules      *base.RuleManager
	priority   uint32
	sliceable  map[ast.ASTHandle][]bool
	order      []ast.ASTHandle
	predicates map[ast.ASTHandle]ast.ASTHandle
	closed     bool
}

func NewRuleSlicePlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *RuleSlicePlugin {
	if priority == 0 {
		priority = 1
	}
	return &RuleSlicePlugin{manager: manager, rules: rules, priority: priority}
}

func (p *RuleSlicePlugin) Priority() uint32            { return p.priority }
func (p *RuleSlicePlugin) CanDestratifyNegation() bool { return false }
func (p *RuleSlicePlugin) Cancel()                     {}

func (p *RuleSlicePlugin) getSlice(declaration ast.ASTHandle) ([]bool, error) {
	if slice, found := p.sliceable[declaration]; found {
		return slice, nil
	}
	arity, err := p.manager.GetArity(declaration)
	if err != nil {
		return nil, err
	}
	slice := make([]bool, arity)
	for index := range slice {
		slice[index] = true
	}
	p.sliceable[declaration] = slice
	p.order = append(p.order, declaration)
	return slice, nil
}

func (p *RuleSlicePlugin) applicationInfo(application ast.ASTHandle) (ast.ASTHandle, []ast.ASTHandle, error) {
	declaration, err := p.manager.GetDecl(application)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	arity, err := p.manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	arguments := make([]ast.ASTHandle, arity)
	for index := uint32(0); index < arity; index++ {
		arguments[index], err = p.manager.GetArg(application, index)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
	}
	return declaration, arguments, nil
}

func ensureSliceVariable(index uint32, input, output, variableSlice *[]bool, solved *[]ast.ASTHandle) {
	for uint32(len(*input)) <= index {
		*input = append(*input, false)
		*output = append(*output, false)
		*variableSlice = append(*variableSlice, true)
		*solved = append(*solved, ast.ASTHandle{})
	}
}

func (p *RuleSlicePlugin) initializeApplication(application ast.ASTHandle, isOutput, isNegative bool, input, output, variableSlice *[]bool, solved *[]ast.ASTHandle) error {
	declaration, arguments, err := p.applicationInfo(application)
	if err != nil {
		return err
	}
	slice, err := p.getSlice(declaration)
	if err != nil {
		return err
	}
	for index, argument := range arguments {
		if isNegative {
			slice[index] = false
		}
		kind, getErr := p.manager.GetKind(argument)
		if getErr != nil {
			return getErr
		}
		if kind == ast.ASTVar {
			variable, getErr := p.manager.GetVarIndex(argument)
			if getErr != nil {
				return getErr
			}
			ensureSliceVariable(variable, input, output, variableSlice, solved)
			if isOutput {
				(*output)[variable] = true
			} else {
				(*input)[variable] = true
			}
			(*variableSlice)[variable] = (*variableSlice)[variable] && slice[index]
		} else {
			value, valueErr := p.manager.IsValue(argument)
			if valueErr != nil || !value {
				return errors.Join(valueErr, ast.ErrASTDomain)
			}
			if !isOutput {
				slice[index] = false
			}
		}
	}
	return nil
}

func (p *RuleSlicePlugin) initializeRule(rule *base.Rule) (input, output, variableSlice []bool, solved []ast.ASTHandle, err error) {
	head, err := rule.Head()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if err = p.initializeApplication(head, true, false, &input, &output, &variableSlice, &solved); err != nil {
		return nil, nil, nil, nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for index := 0; index < uninterpreted; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, nil, nil, nil, getErr
		}
		if getErr = p.initializeApplication(tail, false, negative, &input, &output, &variableSlice, &solved); getErr != nil {
			return nil, nil, nil, nil, getErr
		}
	}
	return input, output, variableSlice, solved, nil
}

func (p *RuleSlicePlugin) freeVariables(expression ast.ASTHandle, destination map[uint32]bool) error {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return err
	}
	used.Reset()
	if err = used.Process(expression); err != nil {
		return err
	}
	for index := uint32(0); index < used.MaxFoundVarIndexPlusOne(); index++ {
		if !used.Contains(index).IsNull() {
			destination[index] = true
		}
	}
	return nil
}

func (p *RuleSlicePlugin) solvedEquality(expression ast.ASTHandle) (uint32, ast.ASTHandle, bool, error) {
	kind, err := p.manager.GetKind(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, false, err
	}
	if kind == ast.ASTVar {
		index, getErr := p.manager.GetVarIndex(expression)
		if getErr != nil {
			return 0, ast.ASTHandle{}, false, getErr
		}
		truth, getErr := p.manager.MkTrue()
		return index, truth, getErr == nil, getErr
	}
	negative, bare, err := p.manager.IsNot(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, false, err
	}
	if negative {
		bareKind, getErr := p.manager.GetKind(bare)
		if getErr != nil {
			return 0, ast.ASTHandle{}, false, getErr
		}
		if bareKind == ast.ASTVar {
			index, getErr := p.manager.GetVarIndex(bare)
			if getErr != nil {
				return 0, ast.ASTHandle{}, false, getErr
			}
			falsity, getErr := p.manager.MkFalse()
			return index, falsity, getErr == nil, getErr
		}
	}
	operation, err := p.manager.GetDeclKind(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, false, err
	}
	if operation == ast.ASTOpEq {
		left, getErr := p.manager.GetArg(expression, 0)
		if getErr != nil {
			return 0, ast.ASTHandle{}, false, getErr
		}
		right, getErr := p.manager.GetArg(expression, 1)
		if getErr != nil {
			return 0, ast.ASTHandle{}, false, getErr
		}
		leftKind, getErr := p.manager.GetKind(left)
		if getErr != nil {
			return 0, ast.ASTHandle{}, false, getErr
		}
		if leftKind == ast.ASTVar {
			index, getErr := p.manager.GetVarIndex(left)
			return index, right, getErr == nil, getErr
		}
		rightKind, getErr := p.manager.GetKind(right)
		if getErr != nil {
			return 0, ast.ASTHandle{}, false, getErr
		}
		if rightKind == ast.ASTVar {
			index, getErr := p.manager.GetVarIndex(right)
			return index, left, getErr == nil, getErr
		}
	}
	if operation == ast.ASTOpIte {
		condition, getErr := p.manager.GetArg(expression, 0)
		if getErr != nil {
			return 0, ast.ASTHandle{}, false, getErr
		}
		thenBranch, getErr := p.manager.GetArg(expression, 1)
		if getErr != nil {
			return 0, ast.ASTHandle{}, false, getErr
		}
		elseBranch, getErr := p.manager.GetArg(expression, 2)
		if getErr != nil {
			return 0, ast.ASTHandle{}, false, getErr
		}
		thenVar, thenResult, thenOK, getErr := p.solvedEquality(thenBranch)
		if getErr != nil || !thenOK {
			return 0, ast.ASTHandle{}, false, getErr
		}
		elseVar, elseResult, elseOK, getErr := p.solvedEquality(elseBranch)
		if getErr != nil || !elseOK || thenVar != elseVar {
			return 0, ast.ASTHandle{}, false, getErr
		}
		result, getErr := p.manager.MkIte(condition, thenResult, elseResult)
		return thenVar, result, getErr == nil, getErr
	}
	return 0, ast.ASTHandle{}, false, nil
}

func (p *RuleSlicePlugin) interpretedConjuncts(rule *base.Rule) ([]ast.ASTHandle, error) {
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	values := make([]ast.ASTHandle, 0, total-uninterpreted)
	for index := uninterpreted; index < total; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		values = append(values, tail)
	}
	return p.manager.FlattenAnd(values)
}

func (p *RuleSlicePlugin) finalizeApplication(application ast.ASTHandle, variableSlice []bool) (bool, error) {
	declaration, arguments, err := p.applicationInfo(application)
	if err != nil {
		return false, err
	}
	slice, err := p.getSlice(declaration)
	if err != nil {
		return false, err
	}
	changed := false
	for index, argument := range arguments {
		kind, getErr := p.manager.GetKind(argument)
		if getErr != nil {
			return false, getErr
		}
		if kind != ast.ASTVar {
			continue
		}
		variable, getErr := p.manager.GetVarIndex(argument)
		if getErr != nil {
			return false, getErr
		}
		if int(variable) < len(variableSlice) && !variableSlice[variable] && slice[index] {
			slice[index] = false
			changed = true
		}
	}
	return changed, nil
}

func (p *RuleSlicePlugin) pruneRule(rule *base.Rule) (bool, error) {
	input, output, variableSlice, solved, err := p.initializeRule(rule)
	if err != nil {
		return false, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return false, err
	}
	seen := make(map[uint32]bool)
	for tailIndex := 0; tailIndex < uninterpreted; tailIndex++ {
		tail, _, getErr := rule.Tail(tailIndex)
		if getErr != nil {
			return false, getErr
		}
		_, arguments, getErr := p.applicationInfo(tail)
		if getErr != nil {
			return false, getErr
		}
		for _, argument := range arguments {
			kind, getErr := p.manager.GetKind(argument)
			if getErr != nil {
				return false, getErr
			}
			if kind != ast.ASTVar {
				continue
			}
			variable, getErr := p.manager.GetVarIndex(argument)
			if getErr != nil {
				return false, getErr
			}
			ensureSliceVariable(variable, &input, &output, &variableSlice, &solved)
			if seen[variable] {
				variableSlice[variable] = false
			} else {
				seen[variable] = true
			}
		}
	}
	used := make(map[uint32]bool)
	parameters := make(map[uint32]bool)
	conjuncts, err := p.interpretedConjuncts(rule)
	if err != nil {
		return false, err
	}
	for _, conjunct := range conjuncts {
		variable, solution, matched, getErr := p.solvedEquality(conjunct)
		pureOutput := int(variable) < len(output) && output[variable] && !input[variable]
		if getErr != nil {
			return false, getErr
		}
		if matched && pureOutput && variableSlice[variable] {
			ensureSliceVariable(variable, &input, &output, &variableSlice, &solved)
			if solved[variable].IsNull() {
				if getErr = p.freeVariables(solution, parameters); getErr != nil {
					return false, getErr
				}
				solved[variable] = solution
			} else {
				if getErr = p.freeVariables(conjunct, used); getErr != nil {
					return false, getErr
				}
				if getErr = p.freeVariables(solved[variable], used); getErr != nil {
					return false, getErr
				}
				used[variable] = true
			}
		} else if getErr = p.freeVariables(conjunct, used); getErr != nil {
			return false, getErr
		}
	}
	for variable := range used {
		if int(variable) < len(variableSlice) {
			variableSlice[variable] = false
		}
	}
	for variable := range variableSlice {
		if !variableSlice[variable] || used[uint32(variable)] {
			variableSlice[variable] = false
			continue
		}
		isInput, isOutput := input[variable], output[variable]
		if isInput && isOutput && (!solved[variable].IsNull() || parameters[uint32(variable)]) {
			variableSlice[variable] = false
		} else if isOutput && parameters[uint32(variable)] {
			variableSlice[variable] = false
		}
	}
	head, err := rule.Head()
	if err != nil {
		return false, err
	}
	changed, err := p.finalizeApplication(head, variableSlice)
	if err != nil {
		return false, err
	}
	for index := 0; index < uninterpreted; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return false, getErr
		}
		updated, getErr := p.finalizeApplication(tail, variableSlice)
		if getErr != nil {
			return false, getErr
		}
		changed = changed || updated
	}
	return changed, nil
}

func (p *RuleSlicePlugin) updateApplication(application ast.ASTHandle) (ast.ASTHandle, error) {
	declaration, arguments, err := p.applicationInfo(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	derived, found := p.predicates[declaration]
	if !found {
		return application, nil
	}
	slice := p.sliceable[declaration]
	kept := make([]ast.ASTHandle, 0, len(arguments))
	for index, argument := range arguments {
		if !slice[index] {
			kept = append(kept, argument)
		}
	}
	return p.manager.MkApp(derived, kept)
}

func (p *RuleSlicePlugin) updateRule(rule *base.Rule) (*base.Rule, error) {
	headDecl, err := rule.HeadDeclaration()
	if err != nil {
		return nil, err
	}
	updated := p.predicates[headDecl] != (ast.ASTHandle{})
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	for index := 0; index < uninterpreted && !updated; index++ {
		declaration, getErr := rule.TailDeclaration(index)
		if getErr != nil {
			return nil, getErr
		}
		updated = !p.predicates[declaration].IsNull()
	}
	if !updated {
		return rule.Retain()
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	head, err = p.updateApplication(head)
	if err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	tails := make([]ast.ASTHandle, 0, total)
	negated := make([]bool, 0, total)
	for index := 0; index < uninterpreted; index++ {
		tail, negative, getErr := rule.Tail(index)
		if getErr != nil {
			return nil, getErr
		}
		tail, getErr = p.updateApplication(tail)
		if getErr != nil {
			return nil, getErr
		}
		tails = append(tails, tail)
		negated = append(negated, negative)
	}
	conjuncts, err := p.interpretedConjuncts(rule)
	if err != nil {
		return nil, err
	}
	for _, conjunct := range conjuncts {
		tails = append(tails, conjunct)
		negated = append(negated, false)
	}
	name, err := rule.Name()
	if err != nil {
		return nil, err
	}
	return p.rules.NewRule(head, tails, negated, name)
}

func (p *RuleSlicePlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		total, getErr := rule.TailSize()
		for position := -1; getErr == nil && position < total; position++ {
			var expression ast.ASTHandle
			if position < 0 {
				expression, getErr = rule.Head()
			} else {
				expression, _, getErr = rule.Tail(position)
			}
			if getErr == nil {
				var quantified bool
				quantified, getErr = p.manager.HasQuantifiers(expression)
				if quantified && getErr == nil {
					getErr = ast.ErrASTDomain
				}
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			if errors.Is(getErr, ast.ErrASTDomain) {
				return nil, nil
			}
			return nil, getErr
		}
	}
	p.sliceable = make(map[ast.ASTHandle][]bool)
	p.predicates = make(map[ast.ASTHandle]ast.ASTHandle)
	p.order = nil
	changed := true
	for changed {
		changed = false
		for index := 0; index < count; index++ {
			rule, getErr := source.Rule(index)
			if getErr != nil {
				return nil, getErr
			}
			updated, pruneErr := p.pruneRule(rule)
			pruneErr = errors.Join(pruneErr, rule.Close())
			if pruneErr != nil {
				return nil, pruneErr
			}
			changed = changed || updated
		}
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
	hasUnslicedOutput := false
	suffix, err := p.manager.Symbols.Intern("slice")
	if err != nil {
		return nil, err
	}
	for _, declaration := range p.order {
		slice := p.sliceable[declaration]
		domain := make([]ast.ASTHandle, 0, len(slice))
		for index, remove := range slice {
			if !remove {
				sort, getErr := p.manager.GetDomain(declaration, uint32(index))
				if getErr != nil {
					return nil, getErr
				}
				domain = append(domain, sort)
			}
		}
		isOutput, getErr := source.IsOutputPredicate(declaration)
		if getErr != nil {
			return nil, getErr
		}
		if len(domain) == len(slice) {
			if isOutput {
				if getErr = result.SetOutputPredicate(declaration); getErr != nil {
					return nil, getErr
				}
				hasUnslicedOutput = true
			}
			continue
		}
		prefix, getErr := p.manager.GetASTName(declaration)
		if getErr != nil {
			return nil, getErr
		}
		rangeSort, getErr := p.manager.GetRange(declaration)
		if getErr != nil {
			return nil, getErr
		}
		derived, getErr := p.manager.MkFreshFuncDecl(prefix, suffix, domain, rangeSort)
		if getErr != nil {
			return nil, getErr
		}
		if getErr = p.rules.RegisterPredicate(derived); getErr != nil {
			return nil, getErr
		}
		p.predicates[declaration] = derived
		if getErr = result.InheritPredicate(source, declaration, derived); getErr != nil {
			return nil, getErr
		}
	}
	if !hasUnslicedOutput {
		p.predicates = nil
		return nil, nil
	}
	if len(p.predicates) == 0 {
		return nil, nil
	}
	for index := 0; index < count; index++ {
		rule, getErr := source.Rule(index)
		if getErr != nil {
			return nil, getErr
		}
		updated, updateErr := p.updateRule(rule)
		updateErr = errors.Join(updateErr, rule.Close())
		if updateErr == nil {
			updateErr = result.Add(updated)
		}
		updateErr = errors.Join(updateErr, closeRuleIf(updated))
		if updateErr != nil {
			return nil, updateErr
		}
	}
	keep = true
	return result, nil
}

func (p *RuleSlicePlugin) DerivedPredicate(original ast.ASTHandle) (ast.ASTHandle, bool) {
	if p == nil || p.closed {
		return ast.ASTHandle{}, false
	}
	derived, found := p.predicates[original]
	return derived, found
}

// PredicateSlices returns independent Go container snapshots of the
// original-to-derived mapping and removed-column masks produced by the latest
// successful transform. The AST handles remain borrowed from the transformed
// rule set.
func (p *RuleSlicePlugin) PredicateSlices() (map[ast.ASTHandle]ast.ASTHandle, map[ast.ASTHandle][]bool) {
	if p == nil || p.closed {
		return make(map[ast.ASTHandle]ast.ASTHandle), make(map[ast.ASTHandle][]bool)
	}
	predicates := make(map[ast.ASTHandle]ast.ASTHandle, len(p.predicates))
	slices := make(map[ast.ASTHandle][]bool, len(p.predicates))
	for original, derived := range p.predicates {
		predicates[original] = derived
		slices[original] = append([]bool(nil), p.sliceable[original]...)
	}
	return predicates, slices
}

func (p *RuleSlicePlugin) Close() error {
	if p == nil || p.closed {
		return base.ErrRuleTransformerOwner
	}
	p.closed = true
	p.manager = nil
	p.rules = nil
	p.sliceable = nil
	p.order = nil
	p.predicates = nil
	return nil
}
