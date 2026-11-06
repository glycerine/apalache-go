package smtcore

// Source: src/smt/theory_str.cpp classify_ast_by_type,
// classify_ast_by_type_in_positive_context and ctx_dep_analysis.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type smtStringExpressionKinds struct {
	variables map[ast.ASTHandle]int
	concats   map[ast.ASTHandle]int
	unrolls   map[ast.ASTHandle]int
}

func newStringExpressionKinds() smtStringExpressionKinds {
	return smtStringExpressionKinds{make(map[ast.ASTHandle]int), make(map[ast.ASTHandle]int), make(map[ast.ASTHandle]int)}
}

func stringOrderedCounts(values map[ast.ASTHandle]int) []ast.ASTHandle {
	var keys []ast.ASTHandle
	for key := range values {
		keys = append(keys, key)
	}
	stringSortHandles(keys)
	return keys
}

func stringOrderedRelations(values map[ast.ASTHandle]map[ast.ASTHandle]int) []ast.ASTHandle {
	var keys []ast.ASTHandle
	for key := range values {
		keys = append(keys, key)
	}
	stringSortHandles(keys)
	return keys
}

func stringOrderedAliases(values map[ast.ASTHandle]ast.ASTHandle) []ast.ASTHandle {
	var keys []ast.ASTHandle
	for key := range values {
		keys = append(keys, key)
	}
	stringSortHandles(keys)
	return keys
}

func stringDependencyAlias(aliases map[ast.ASTHandle]ast.ASTHandle, expression ast.ASTHandle) ast.ASTHandle {
	if alias, found := aliases[expression]; found {
		return alias
	}
	return expression
}

func (t *SMTStringTheory) classifiesEmpty(expression ast.ASTHandle) (bool, error) {
	value, known, err := t.eqcStringValue(expression)
	if err != nil || !known {
		return false, err
	}
	text, _, err := t.state.manager.SequenceStringValue(value)
	return text == "", err
}

func (t *SMTStringTheory) classifyStringExpression(expression ast.ASTHandle, kinds smtStringExpressionKinds) error {
	m := t.state.manager
	work := []ast.ASTHandle{expression}
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		_, variable := t.variableSet[current]
		_, lengthTester := t.lengthTestVariables[current]
		_, valueTester := t.valueTestVariables[current]
		_, unrollTester := t.unrollTestVariables[current]
		if variable && !lengthTester && !valueTester && !unrollTester {
			kinds.variables[current] = 1
			continue
		}
		kind, err := m.GetKind(current)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			continue
		}
		op, own, err := t.operation(current)
		if err != nil {
			return err
		}
		if own && op == 9 {
			continue // Do not descend through length terms.
		}
		if own && op == ast.ASTSequenceOpConcat {
			left, err := m.GetArg(current, 0)
			if err != nil {
				return err
			}
			right, err := m.GetArg(current, 1)
			if err != nil {
				return err
			}
			leftEmpty, err := t.classifiesEmpty(left)
			if err != nil {
				return err
			}
			rightEmpty, err := t.classifiesEmpty(right)
			if err != nil {
				return err
			}
			if !leftEmpty && !rightEmpty {
				kinds.concats[current] = 1
			}
		} else if own && op == 43 {
			kinds.unrolls[current] = 1
		}
		count, err := m.GetNumArgs(current)
		if err != nil {
			return err
		}
		for i := count; i > 0; i-- {
			child, err := m.GetArg(current, i-1)
			if err != nil {
				return err
			}
			work = append(work, child)
		}
	}
	return nil
}

func (t *SMTStringTheory) classifyPositiveStringEqualities(kinds smtStringExpressionKinds) error {
	for _, literal := range t.state.assignedLiterals {
		if literal.Sign() {
			continue // Native assignments contain NOT(eq) for a false equality.
		}
		expression, err := t.state.BoolExpression(literal.Var())
		if err != nil {
			return err
		}
		op, basic, err := t.state.basicOperation(expression)
		if err != nil {
			return err
		}
		if basic && op == ast.ASTOpEq {
			if err := t.classifyStringExpression(expression, kinds); err != nil {
				return err
			}
		}
	}
	return nil
}

type smtStringDependencyAnalysis struct {
	variables       map[ast.ASTHandle]int
	freeVariables   map[ast.ASTHandle]int
	unrollGroups    map[ast.ASTHandle]map[ast.ASTHandle]int
	variableConcats map[ast.ASTHandle]map[ast.ASTHandle]int
	variableUnrolls map[ast.ASTHandle]map[ast.ASTHandle]int
	variableAliases map[ast.ASTHandle]ast.ASTHandle
	concatAliases   map[ast.ASTHandle]ast.ASTHandle
	variableValues  map[ast.ASTHandle]ast.ASTHandle
	concatValues    map[ast.ASTHandle]ast.ASTHandle
	concatGroups    map[ast.ASTHandle]map[ast.ASTHandle]int
	dependencies    map[ast.ASTHandle]map[ast.ASTHandle]int
}

func stringPutRelation(relations map[ast.ASTHandle]map[ast.ASTHandle]int, from, to ast.ASTHandle, kind int) {
	if relations[from] == nil {
		relations[from] = make(map[ast.ASTHandle]int)
	}
	relations[from][to] = kind
}

func (t *SMTStringTheory) analyzeStringDependencies() (analysis *smtStringDependencyAnalysis, result error) {
	if _, err := t.State(); err != nil {
		return nil, err
	}
	kinds := newStringExpressionKinds()
	for variable := range t.variableSet {
		if _, internal := t.internalVariables[variable]; !internal {
			kinds.variables[variable] = 1
		}
	}
	if err := t.classifyPositiveStringEqualities(kinds); err != nil {
		return nil, err
	}
	g := newStringGrounding(t)
	defer func() { result = errors.Join(result, g.close()) }()
	a := &smtStringDependencyAnalysis{
		variables: kinds.variables, freeVariables: make(map[ast.ASTHandle]int),
		unrollGroups: make(map[ast.ASTHandle]map[ast.ASTHandle]int), variableConcats: g.variableConcats,
		variableUnrolls: make(map[ast.ASTHandle]map[ast.ASTHandle]int), variableAliases: g.variableAliases,
		concatAliases: g.concatAliases, variableValues: g.variableValues, concatValues: g.concatValues,
		concatGroups: make(map[ast.ASTHandle]map[ast.ASTHandle]int), dependencies: make(map[ast.ASTHandle]map[ast.ASTHandle]int),
	}
	unrollAliases := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, unroll := range stringOrderedCounts(kinds.unrolls) {
		if _, found := unrollAliases[unroll]; found {
			continue
		}
		first := t.state.context.enodes[unroll]
		if first == nil {
			return nil, ast.ErrASTDomain
		}
		var root ast.ASTHandle
		for current := first; ; {
			op, own, err := t.operation(current.Owner())
			if err != nil {
				return nil, err
			}
			if own && op == 43 {
				if root.IsNull() {
					root = current.Owner()
				}
				unrollAliases[current.Owner()] = root
			}
			current = current.Next()
			if current == first {
				break
			}
		}
	}
	for _, unroll := range stringOrderedCounts(kinds.unrolls) {
		stringPutRelation(a.unrollGroups, unrollAliases[unroll], unroll, 1)
	}
	for _, variable := range stringOrderedCounts(a.variables) {
		if _, found := a.variableAliases[variable]; found {
			continue
		}
		var root ast.ASTHandle
		for _, member := range t.stringClass(variable) {
			if _, known := t.variableSet[member]; known {
				if root.IsNull() {
					root = member
				} else {
					a.variableAliases[member] = root
				}
			}
		}
	}
	for _, variable := range stringOrderedCounts(a.variables) {
		root := stringDependencyAlias(a.variableAliases, variable)
		if _, found := a.variableValues[root]; !found {
			value, known, err := t.eqcStringValue(root)
			if err != nil {
				return nil, err
			}
			if known {
				a.variableValues[root] = value
			}
		}
		if _, found := a.variableConcats[root]; found {
			continue
		}
		members := t.stringClass(root)
		for _, member := range members[1:] {
			left, right, concat, err := t.state.manager.SequenceConcatParts(member)
			if err != nil {
				return nil, err
			}
			if concat {
				leftEmpty, err := t.classifiesEmpty(left)
				if err != nil {
					return nil, err
				}
				rightEmpty, err := t.classifiesEmpty(right)
				if err != nil {
					return nil, err
				}
				if !leftEmpty && !rightEmpty {
					stringPutRelation(a.variableConcats, root, member, 1)
				}
			} else {
				op, own, err := t.operation(member)
				if err != nil {
					return nil, err
				}
				if own && op == 43 {
					stringPutRelation(a.variableUnrolls, root, member, 1)
				}
			}
		}
	}
	for _, concat := range stringOrderedCounts(kinds.concats) {
		if _, found := a.concatAliases[concat]; found {
			continue
		}
		var root ast.ASTHandle
		for _, member := range t.stringClass(concat) {
			_, _, isConcat, err := t.state.manager.SequenceConcatParts(member)
			if err != nil {
				return nil, err
			}
			if isConcat {
				if root.IsNull() {
					root = member
				} else {
					a.concatAliases[member] = root
				}
			}
		}
	}
	for _, concat := range stringOrderedCounts(kinds.concats) {
		root := stringDependencyAlias(a.concatAliases, concat)
		if _, found := a.concatValues[root]; !found {
			value, known, err := t.eqcStringValue(root)
			if err != nil {
				return nil, err
			}
			if known {
				a.concatValues[root] = value
			}
		}
		if _, found := a.concatGroups[root]; !found {
			for _, member := range t.stringClass(root) {
				_, _, isConcat, err := t.state.manager.SequenceConcatParts(member)
				if err != nil {
					return nil, err
				}
				if _, classified := kinds.concats[member]; isConcat && classified {
					stringPutRelation(a.concatGroups, root, member, 1)
				}
			}
		}
	}
	if len(t.containsByPair) != 0 {
		if err := g.computeContains(); err != nil {
			return nil, err
		}
	}
	if err := a.buildDependencies(t); err != nil {
		return nil, err
	}
	return a, nil // Native ctx_dep_analysis's active code always returns zero.
}

func (a *smtStringDependencyAnalysis) buildDependencies(t *SMTStringTheory) error {
	for _, variable := range stringOrderedAliases(a.variableValues) {
		stringPutRelation(a.dependencies, stringDependencyAlias(a.variableAliases, variable), a.variableValues[variable], 1)
	}
	for _, variable := range stringOrderedRelations(a.variableConcats) {
		root := stringDependencyAlias(a.variableAliases, variable)
		for _, concat := range stringOrderedCounts(a.variableConcats[variable]) {
			kinds := newStringExpressionKinds()
			if err := t.classifyStringExpression(concat, kinds); err != nil {
				return err
			}
			for _, child := range stringOrderedCounts(kinds.variables) {
				child = stringDependencyAlias(a.variableAliases, child)
				if a.dependencies[root][child] != 1 {
					stringPutRelation(a.dependencies, root, child, 2)
				}
			}
		}
	}
	for _, variable := range stringOrderedRelations(a.variableUnrolls) {
		root := stringDependencyAlias(a.variableAliases, variable)
		for _, unroll := range stringOrderedCounts(a.variableUnrolls[variable]) {
			kinds := newStringExpressionKinds()
			if err := t.classifyStringExpression(unroll, kinds); err != nil {
				return err
			}
			for _, child := range stringOrderedCounts(kinds.variables) {
				child = stringDependencyAlias(a.variableAliases, child)
				_, known, err := t.eqcStringValue(child)
				if err != nil {
					return err
				}
				if _, found := a.dependencies[root][child]; !known && !found {
					stringPutRelation(a.dependencies, root, child, 6)
				}
			}
		}
	}
	for _, concat := range stringOrderedAliases(a.concatValues) {
		kinds := newStringExpressionKinds()
		if err := t.classifyStringExpression(concat, kinds); err != nil {
			return err
		}
		for _, child := range stringOrderedCounts(kinds.variables) {
			child = stringDependencyAlias(a.variableAliases, child)
			if a.dependencies[child][a.concatValues[concat]] != 1 {
				stringPutRelation(a.dependencies, child, a.concatValues[concat], 3)
			}
		}
	}
	leftGroups, rightGroups := newStringBoundaryGroups(), newStringBoundaryGroups()
	for _, root := range stringOrderedRelations(a.concatGroups) {
		left, right := make(map[ast.ASTHandle]int), make(map[ast.ASTHandle]int)
		var leftConstant, rightConstant ast.ASTHandle
		for _, concat := range stringOrderedCounts(a.concatGroups[root]) {
			for side := 0; side < 2; side++ {
				boundary, err := t.stringConcatBoundary(concat, side == 1)
				if err != nil {
					return err
				}
				text, literal, err := t.state.manager.SequenceStringValue(boundary)
				if err != nil {
					return err
				}
				if side == 0 {
					if literal {
						if leftConstant.IsNull() && text == "" {
							leftConstant = boundary
						}
					} else {
						left[boundary] = 1
					}
				} else if literal {
					if rightConstant.IsNull() && text == "" {
						rightConstant = boundary
					}
				} else {
					right[boundary] = 1
				}
			}
		}
		for _, boundary := range stringOrderedCounts(left) {
			variable := stringDependencyAlias(a.variableAliases, boundary)
			if !leftConstant.IsNull() && a.dependencies[variable][leftConstant] != 1 {
				stringPutRelation(a.dependencies, variable, leftConstant, 4)
			}
		}
		if err := leftGroups.add(t, a.variableAliases, left, true); err != nil {
			return err
		}
		for _, boundary := range stringOrderedCounts(right) {
			variable := stringDependencyAlias(a.variableAliases, boundary)
			if !rightConstant.IsNull() && a.dependencies[variable][rightConstant] != 1 {
				stringPutRelation(a.dependencies, variable, rightConstant, 5)
			}
		}
		if err := rightGroups.add(t, a.variableAliases, right, false); err != nil {
			return err
		}
	}
	constrained := make(map[ast.ASTHandle]map[ast.ASTHandle]int)
	for _, groups := range []smtStringBoundaryGroups{leftGroups, rightGroups} {
		for _, members := range groups.groups {
			keys := stringOrderedCounts(members)
			for i, left := range keys {
				for _, right := range keys[i+1:] {
					stringPutRelation(constrained, left, right, 1)
					stringPutRelation(constrained, right, left, 1)
				}
			}
		}
	}
	a.selectFreeVariables(t, constrained)
	return nil
}

func (t *SMTStringTheory) stringConcatBoundary(expression ast.ASTHandle, right bool) (ast.ASTHandle, error) {
	for {
		left, tail, concat, err := t.state.manager.SequenceConcatParts(expression)
		if err != nil || !concat {
			return expression, err
		}
		expression = left
		if right {
			expression = tail
		}
	}
}

type smtStringBoundaryGroups struct {
	indices map[ast.ASTHandle]int
	groups  map[int]map[ast.ASTHandle]int
}

func newStringBoundaryGroups() smtStringBoundaryGroups {
	return smtStringBoundaryGroups{make(map[ast.ASTHandle]int), make(map[int]map[ast.ASTHandle]int)}
}

func (g smtStringBoundaryGroups) add(t *SMTStringTheory, aliases map[ast.ASTHandle]ast.ASTHandle, nodes map[ast.ASTHandle]int, filterBeforeAlias bool) error {
	members := make(map[ast.ASTHandle]int)
	for _, node := range stringOrderedCounts(nodes) {
		if filterBeforeAlias {
			_, known, err := t.eqcStringValue(node)
			if err != nil {
				return err
			}
			if known {
				continue
			}
		}
		members[stringDependencyAlias(aliases, node)] = 1
	}
	if len(members) <= 1 {
		return nil
	}
	group := -1
	for _, member := range stringOrderedCounts(members) {
		if previous, found := g.indices[member]; found {
			group = previous
			break
		}
	}
	if group == -1 {
		group = len(g.groups)
	}
	for _, member := range stringOrderedCounts(members) {
		_, known, err := t.eqcStringValue(member)
		if err != nil {
			return err
		}
		if !known {
			g.indices[member] = group
			if g.groups[group] == nil {
				g.groups[group] = make(map[ast.ASTHandle]int)
			}
			g.groups[group][member] = 1
		}
	}
	return nil
}

func (a *smtStringDependencyAnalysis) selectFreeVariables(t *SMTStringTheory, constrained map[ast.ASTHandle]map[ast.ASTHandle]int) {
	selectVariable := func(variable ast.ASTHandle) {
		for already := range a.freeVariables {
			if _, found := constrained[variable][already]; found {
				return
			}
		}
		a.freeVariables[variable] = 1
	}
	if len(a.dependencies) == 0 {
		for _, variable := range stringOrderedCounts(a.variables) {
			selectVariable(stringDependencyAlias(a.variableAliases, variable))
		}
		return
	}
	for _, variable := range stringOrderedCounts(a.variables) {
		root := stringDependencyAlias(a.variableAliases, variable)
		if _, dependent := a.dependencies[root]; !dependent {
			selectVariable(root)
		}
	}
	for _, from := range stringOrderedRelations(a.dependencies) {
		for _, target := range stringOrderedCounts(a.dependencies[from]) {
			if _, variable := t.variableSet[target]; !variable {
				continue
			}
			root := stringDependencyAlias(a.variableAliases, target)
			if _, dependent := a.dependencies[root]; dependent {
				continue
			}
			if _, selected := a.freeVariables[root]; selected {
				a.freeVariables[root]++
			} else {
				selectVariable(root)
			}
		}
	}
}
