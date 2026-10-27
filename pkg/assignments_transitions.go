package apalache

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type assignmentSelection []UID
type assignmentSelectionSet map[string]assignmentSelection
type assignmentSelectionMap map[UID]assignmentSelectionSet

type letSelectionInfo struct {
	bodyID     UID
	selections assignmentSelectionMap
}

// SymbolicTransitionGenerator slices an action into equivalence classes of
// branches that contain the same selected assignments.
type SymbolicTransitionGenerator struct{ tracker TransformationTracker }

func NewSymbolicTransitionGenerator(tracker TransformationTracker) *SymbolicTransitionGenerator {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &SymbolicTransitionGenerator{tracker: tracker}
}

func (generator *SymbolicTransitionGenerator) Generate(expression Expr, strategy AssignmentStrategy) ([]SymbolicTransition, error) {
	if expression == nil {
		return nil, &AssignmentError{Message: "cannot generate transitions from a nil expression"}
	}
	selected := make(map[UID]struct{}, len(strategy))
	for _, id := range strategy {
		selected[id] = struct{}{}
	}
	introduction := NewAssignmentOperatorIntroductionFromSet(selected, generator.tracker)
	transformed := introduction.Expression(expression)
	replacements := introduction.Replacements()
	newStrategy := make(AssignmentStrategy, len(strategy))
	for index, id := range strategy {
		if replacement, found := replacements[id]; found {
			newStrategy[index] = replacement
		} else {
			newStrategy[index] = id
		}
	}
	selections := generator.allSelections(transformed, nil)
	rootSelections, found := selections[transformed.ID()]
	if !found || len(rootSelections) == 0 {
		return nil, &AssignmentError{Message: "the assignment strategy produced no symbolic transitions"}
	}
	size := -1
	for _, selection := range rootSelections {
		if size < 0 {
			size = len(selection)
		} else if size != len(selection) {
			return nil, &AssignmentError{Message: "symbolic transition selections have inconsistent assignment counts"}
		}
	}
	orderedSelections := selectionSetValues(rootSelections)
	sort.Slice(orderedSelections, func(left, right int) bool { return selectionLess(orderedSelections[left], orderedSelections[right]) })
	result := make([]SymbolicTransition, len(orderedSelections))
	for index, selection := range orderedSelections {
		result[index] = SymbolicTransition{
			Assignments: orderedAssignmentSubset(selection, newStrategy),
			Expression:  generator.sliceWith(selection, selections, transformed),
		}
	}
	return result, nil
}

func (generator *SymbolicTransitionGenerator) allSelections(expression Expr, lets map[string]letSelectionInfo) assignmentSelectionMap {
	if assignment, ok := expression.(*OperExpr); ok && assignment.Operator().Name() == "Apalache!:=" {
		selection := canonicalSelection([]UID{assignment.ID()})
		return assignmentSelectionMap{assignment.ID(): selectionSet(selection)}
	}
	if operation, ok := expression.(*OperExpr); ok {
		arguments := operation.Arguments()
		switch operation.Operator().Name() {
		case "LABEL":
			bodyMap := generator.allSelections(arguments[0], lets)
			if len(bodyMap) == 0 {
				return bodyMap
			}
			result := cloneSelectionMap(bodyMap)
			selection, found := bodyMap[arguments[0].ID()]
			if !found {
				selection = selectionSet(assignmentSelection{})
			}
			result[operation.ID()] = cloneSelectionSet(selection)
			return result
		case "AND":
			result := make(assignmentSelectionMap)
			childSets := make([]assignmentSelectionSet, 0, len(arguments))
			for _, argument := range arguments {
				childMap := generator.allSelections(argument, lets)
				mergeSelectionMaps(result, childMap)
				if selections, found := childMap[argument.ID()]; found {
					childSets = append(childSets, selections)
				}
			}
			mine := allSelectionCombinations(childSets)
			if len(mine) != 0 && !selectionSetContainsEmpty(mine) {
				result[operation.ID()] = mine
			}
			return result
		case "OR":
			result := make(assignmentSelectionMap)
			mine := make(assignmentSelectionSet)
			for _, argument := range arguments {
				childMap := generator.allSelections(argument, lets)
				mergeSelectionMaps(result, childMap)
				if selections, found := childMap[argument.ID()]; found {
					mergeSelectionSets(mine, selections)
				}
			}
			if len(mine) != 0 && !selectionSetContainsEmpty(mine) {
				result[operation.ID()] = mine
			}
			return result
		case "EXISTS3":
			body := arguments[2]
			result := generator.allSelections(body, lets)
			mine, found := result[body.ID()]
			if !found {
				mine = selectionSet(assignmentSelection{})
			}
			if !selectionSetContainsEmpty(mine) {
				result[operation.ID()] = cloneSelectionSet(mine)
			}
			return result
		case "IF_THEN_ELSE":
			result := make(assignmentSelectionMap)
			mine := make(assignmentSelectionSet)
			for _, branch := range arguments[1:] {
				childMap := generator.allSelections(branch, lets)
				mergeSelectionMaps(result, childMap)
				if selections, found := childMap[branch.ID()]; found {
					mergeSelectionSets(mine, selections)
				}
			}
			if len(mine) != 0 && !selectionSetContainsEmpty(mine) {
				result[operation.ID()] = mine
			}
			return result
		case "OPER_APP":
			if len(arguments) == 1 {
				if name, named := arguments[0].(*NameExpr); named && lets != nil {
					if info, found := lets[name.Name()]; found {
						result := cloneSelectionMap(info.selections)
						mine, hasBody := info.selections[info.bodyID]
						if !hasBody {
							mine = selectionSet(assignmentSelection{})
						}
						if !selectionSetContainsEmpty(mine) {
							result[operation.ID()] = cloneSelectionSet(mine)
						}
						return result
					}
				}
			}
			return make(assignmentSelectionMap)
		default:
			return make(assignmentSelectionMap)
		}
	}
	if let, ok := expression.(*LetInExpr); ok {
		definitions := make(map[string]letSelectionInfo, len(let.Declarations()))
		for _, declaration := range let.Declarations() {
			definitions[declaration.Name()] = letSelectionInfo{
				bodyID: declaration.Body().ID(), selections: generator.allSelections(declaration.Body(), lets),
			}
		}
		inside := make(map[string]letSelectionInfo, len(lets)+len(definitions))
		for name, info := range lets {
			inside[name] = info
		}
		for name, info := range definitions {
			inside[name] = info
		}
		result := generator.allSelections(let.Body(), inside)
		mine, found := result[let.Body().ID()]
		if !found {
			mine = selectionSet(assignmentSelection{})
		}
		if !selectionSetContainsEmpty(mine) {
			result[let.ID()] = cloneSelectionSet(mine)
		}
		return result
	}
	return make(assignmentSelectionMap)
}

func (generator *SymbolicTransitionGenerator) sliceWith(selection assignmentSelection, selections assignmentSelectionMap, expression Expr) Expr {
	if operation, ok := expression.(*OperExpr); ok {
		if operation.Operator().Name() == "Apalache!:=" {
			return operation
		}
		arguments := operation.Arguments()
		switch operation.Operator().Name() {
		case "OR":
			retained := make([]Expr, 0, len(arguments))
			for _, argument := range arguments {
				if selectionsIntersect(labelsAt(argument.ID(), selections), selection) {
					retained = append(retained, argument)
				}
			}
			switch len(retained) {
			case 0:
				return operation
			case 1:
				return generator.sliceWith(selection, selections, retained[0])
			default:
				for index, argument := range retained {
					retained[index] = generator.sliceWith(selection, selections, argument)
				}
				return MustOperExpr(operation.Operator(), operation.Tag(), retained...)
			}
		case "IF_THEN_ELSE":
			branches := make([]Expr, len(arguments)-1)
			for index, branch := range arguments[1:] {
				if selectionsIntersect(labelsAt(branch.ID(), selections), selection) {
					branches[index] = generator.sliceWith(selection, selections, branch)
				} else {
					branches[index] = NewValueExpr(BoolValue(false), operation.Tag())
				}
			}
			if len(branches) == 2 {
				thenFalse, elseFalse := isFalseLiteral(branches[0]), isFalseLiteral(branches[1])
				switch {
				case thenFalse && elseFalse:
					return operation
				case !thenFalse && elseFalse:
					return MustOperExpr(Op("AND"), operation.Tag(), arguments[0], branches[0])
				case thenFalse && !elseFalse:
					return MustOperExpr(Op("AND"), operation.Tag(), MustOperExpr(Op("NOT"), operation.Tag(), arguments[0]), branches[1])
				}
			}
			return MustOperExpr(operation.Operator(), operation.Tag(), append([]Expr{arguments[0]}, branches...)...)
		default:
			changed := false
			for index, argument := range arguments {
				arguments[index] = generator.sliceWith(selection, selections, argument)
				changed = changed || !EqualExpr(argument, arguments[index])
			}
			if !changed {
				return operation
			}
			return MustOperExpr(operation.Operator(), operation.Tag(), arguments...)
		}
	}
	if let, ok := expression.(*LetInExpr); ok {
		declarations := let.Declarations()
		changed := false
		for index, declaration := range declarations {
			body := generator.sliceWith(selection, selections, declaration.Body())
			if !EqualExpr(body, declaration.Body()) {
				copy := declaration.copyWithTag(declaration.Tag())
				copy.SetBody(body)
				declarations[index] = copy
				changed = true
			}
		}
		body := generator.sliceWith(selection, selections, let.Body())
		if isFalseLiteral(body) {
			return body
		}
		changed = changed || !EqualExpr(body, let.Body())
		if !changed {
			return let
		}
		return NewLetInExpr(body, let.Tag(), declarations...)
	}
	return expression
}

func allSelectionCombinations(sets []assignmentSelectionSet) assignmentSelectionSet {
	if len(sets) == 0 {
		return make(assignmentSelectionSet)
	}
	result := cloneSelectionSet(sets[0])
	for _, next := range sets[1:] {
		combined := make(assignmentSelectionSet)
		for _, left := range result {
			for _, right := range next {
				selection := unionSelections(left, right)
				combined[selectionKey(selection)] = selection
			}
		}
		result = combined
	}
	return result
}

func labelsAt(id UID, selections assignmentSelectionMap) assignmentSelection {
	set, found := selections[id]
	if !found {
		return nil
	}
	all := assignmentSelection(nil)
	for _, selection := range set {
		all = unionSelections(all, selection)
	}
	return all
}

func orderedAssignmentSubset(selection assignmentSelection, strategy AssignmentStrategy) AssignmentStrategy {
	selected := make(map[UID]struct{}, len(selection))
	for _, id := range selection {
		selected[id] = struct{}{}
	}
	result := make(AssignmentStrategy, 0, len(selection))
	for _, id := range strategy {
		if _, found := selected[id]; found {
			result = append(result, id)
		}
	}
	return result
}

func canonicalSelection(ids []UID) assignmentSelection {
	seen := make(map[UID]struct{}, len(ids))
	result := make(assignmentSelection, 0, len(ids))
	for _, id := range ids {
		if _, found := seen[id]; !found {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Compare(result[right]) < 0 })
	return result
}

func unionSelections(left, right assignmentSelection) assignmentSelection {
	return canonicalSelection(append(append([]UID(nil), left...), right...))
}

func selectionKey(selection assignmentSelection) string {
	parts := make([]string, len(selection))
	for index, id := range selection {
		parts[index] = strconv.FormatUint(id.Uint64(), 10)
	}
	return strings.Join(parts, ",")
}

func selectionSet(selections ...assignmentSelection) assignmentSelectionSet {
	result := make(assignmentSelectionSet, len(selections))
	for _, selection := range selections {
		canonical := canonicalSelection(selection)
		result[selectionKey(canonical)] = canonical
	}
	return result
}

func cloneSelectionSet(source assignmentSelectionSet) assignmentSelectionSet {
	result := make(assignmentSelectionSet, len(source))
	for key, selection := range source {
		result[key] = append(assignmentSelection(nil), selection...)
	}
	return result
}

func cloneSelectionMap(source assignmentSelectionMap) assignmentSelectionMap {
	result := make(assignmentSelectionMap, len(source))
	for id, selections := range source {
		result[id] = cloneSelectionSet(selections)
	}
	return result
}

func mergeSelectionSets(destination, source assignmentSelectionSet) {
	for key, selection := range source {
		destination[key] = append(assignmentSelection(nil), selection...)
	}
}

func mergeSelectionMaps(destination, source assignmentSelectionMap) {
	for id, selections := range source {
		destination[id] = cloneSelectionSet(selections)
	}
}

func selectionSetContainsEmpty(set assignmentSelectionSet) bool {
	_, found := set[""]
	return found
}

func selectionSetValues(set assignmentSelectionSet) []assignmentSelection {
	result := make([]assignmentSelection, 0, len(set))
	for _, selection := range set {
		result = append(result, append(assignmentSelection(nil), selection...))
	}
	return result
}

func selectionLess(left, right assignmentSelection) bool {
	if len(left) != len(right) {
		return len(left) < len(right)
	}
	for index := range left {
		comparison := left[index].Compare(right[index])
		if comparison != 0 {
			return comparison < 0
		}
	}
	return false
}

func selectionsIntersect(left, right assignmentSelection) bool {
	selected := make(map[UID]struct{}, len(left))
	for _, id := range left {
		selected[id] = struct{}{}
	}
	for _, id := range right {
		if _, found := selected[id]; found {
			return true
		}
	}
	return false
}

func isFalseLiteral(expression Expr) bool {
	value, ok := expression.(*ValueExpr)
	if !ok {
		return false
	}
	boolean, ok := value.Value().(BoolValue)
	return ok && !bool(boolean)
}

func validateSelectionMap(selections assignmentSelectionMap) error {
	for id, set := range selections {
		for key, selection := range set {
			if key != selectionKey(selection) {
				return fmt.Errorf("non-canonical selection at %s", id)
			}
		}
	}
	return nil
}
