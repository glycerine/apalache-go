package apalache

import (
	"fmt"
	"strconv"
	"strings"
)

func MakeRenamedName(base string, counter int) string { return fmt.Sprintf("%s$%d", base, counter) }

func ParseRenamedName(name string) (base string, counter int, renamed bool, err error) {
	if strings.Count(name, "$") > 1 {
		return "", 0, false, fmt.Errorf("variable name %q contains more than one separator", name)
	}
	before, after, found := strings.Cut(name, "$")
	if !found {
		return "", 0, false, nil
	}
	if after == "" {
		return "", 0, false, fmt.Errorf("renamed variable %q has no counter", name)
	}
	for _, digit := range after {
		if digit < '0' || digit > '9' {
			return "", 0, false, nil
		}
	}
	counter, err = strconv.Atoi(after)
	if err != nil {
		return "", 0, false, err
	}
	return before, counter, true, nil
}

func RenamedNameBase(name string) string {
	base, _, renamed, err := ParseRenamedName(name)
	if err != nil {
		panic(err)
	}
	if renamed {
		return base
	}
	return name
}

func MergeNameCounters(left, right map[string]int, takeMax bool) map[string]int {
	result := cloneMap(left)
	for name, rightCounter := range right {
		leftCounter, exists := left[name]
		if !exists || takeMax && rightCounter > leftCounter || !takeMax && rightCounter < leftCounter {
			result[name] = rightCounter
		}
	}
	return result
}

func NameCountersInExpression(expression Expr, takeMax bool) map[string]int {
	result := make(map[string]int)
	var addName = func(name string) {
		base, counter, renamed, err := ParseRenamedName(name)
		if err != nil {
			panic(err)
		}
		if renamed {
			result = MergeNameCounters(result, map[string]int{base: counter}, takeMax)
		}
	}
	var visit func(Expr)
	visit = func(expression Expr) {
		switch typed := expression.(type) {
		case *NameExpr:
			addName(typed.Name())
		case *OperExpr:
			for _, argument := range typed.Arguments() {
				visit(argument)
			}
		case *LetInExpr:
			visit(typed.Body())
			for _, declaration := range typed.Declarations() {
				addName(declaration.Name())
				for _, parameter := range declaration.FormalParams() {
					addName(parameter.Name)
				}
				visit(declaration.Body())
			}
		}
	}
	visit(expression)
	return result
}

func NameCountersInDeclarations(declarations []Decl, takeMax bool) map[string]int {
	result := make(map[string]int)
	for _, declaration := range declarations {
		operator, ok := declaration.(*OperDecl)
		if !ok {
			continue
		}
		one := NameCountersInExpression(operator.Body(), takeMax)
		for _, name := range append([]string{operator.Name()}, parameterNames(operator.FormalParams())...) {
			base, counter, renamed, err := ParseRenamedName(name)
			if err != nil {
				panic(err)
			}
			if renamed {
				one = MergeNameCounters(one, map[string]int{base: counter}, takeMax)
			}
		}
		result = MergeNameCounters(result, one, takeMax)
	}
	return result
}

func parameterNames(parameters []OperatorParam) []string {
	result := make([]string, len(parameters))
	for index, parameter := range parameters {
		result[index] = parameter.Name
	}
	return result
}

func OffsetRenamedName(offsets map[string]int, name string) (string, error) {
	base, counter, renamed, err := ParseRenamedName(name)
	if err != nil || !renamed {
		return name, err
	}
	offset := offsets[base]
	if offset == 0 {
		return name, nil
	}
	if counter-offset <= 0 {
		return "", fmt.Errorf("offset %d makes counter in %q non-positive", offset, name)
	}
	return MakeRenamedName(base, counter-offset), nil
}

type IncrementalRenamer struct {
	tracker  TransformationTracker
	counters map[string]int
}

func NewIncrementalRenamer(tracker TransformationTracker) *IncrementalRenamer {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &IncrementalRenamer{tracker: tracker, counters: make(map[string]int)}
}

func (renamer *IncrementalRenamer) next(name string) string {
	base := RenamedNameBase(name)
	renamer.counters[base]++
	return MakeRenamedName(base, renamer.counters[base])
}

// FreshName returns the next incremental spelling for base.
func (renamer *IncrementalRenamer) FreshName(base string) string { return renamer.next(base) }

func (renamer *IncrementalRenamer) Rename(expression Expr) Expr {
	return renamer.rename(expression, make(map[string]string))
}

func (renamer *IncrementalRenamer) rename(expression Expr, substitutions map[string]string) Expr {
	tracked := renamer.tracker.TrackExpression(func(expression Expr) Expr {
		switch typed := expression.(type) {
		case *NameExpr:
			if name, found := substitutions[typed.Name()]; found {
				return renamer.tracker.Hold(typed, NewNameExpr(name, typed.Tag()))
			}
			return typed
		case *OperExpr:
			arguments := typed.Arguments()
			name := typed.Operator().Name()
			if isSingleBinder(name) && len(arguments) > 0 {
				binder, ok := arguments[0].(*NameExpr)
				if !ok {
					panic(fmt.Errorf("%s expected a name binder", name))
				}
				inside := cloneMap(substitutions)
				inside[binder.Name()] = renamer.next(binder.Name())
				arguments[0] = renamer.tracker.Hold(binder, NewNameExpr(inside[binder.Name()], binder.Tag()))
				for index := 1; index < len(arguments); index++ {
					arguments[index] = renamer.rename(arguments[index], inside)
				}
				return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			}
			if isMultipleBinder(name) && len(arguments) > 0 {
				if (len(arguments)-1)%2 != 0 {
					panic(fmt.Errorf("%s expected name/set pairs", name))
				}
				inside := cloneMap(substitutions)
				for index := 1; index < len(arguments); index += 2 {
					binder, ok := arguments[index].(*NameExpr)
					if !ok {
						panic(fmt.Errorf("%s expected a name at argument %d", name, index))
					}
					inside[binder.Name()] = renamer.next(binder.Name())
				}
				for index, argument := range arguments {
					arguments[index] = renamer.rename(argument, inside)
				}
				return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			}
			changed := false
			for index, argument := range arguments {
				arguments[index] = renamer.rename(argument, substitutions)
				changed = changed || !EqualExpr(argument, arguments[index])
			}
			if !changed {
				return typed
			}
			return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
		case *LetInExpr:
			declarations := typed.Declarations()
			inside := cloneMap(substitutions)
			for _, declaration := range declarations {
				inside[declaration.Name()] = renamer.next(declaration.Name())
				for _, parameter := range declaration.FormalParams() {
					inside[parameter.Name] = renamer.next(parameter.Name)
				}
			}
			for index, declaration := range declarations {
				parameters := declaration.FormalParams()
				for parameterIndex := range parameters {
					parameters[parameterIndex].Name = inside[parameters[parameterIndex].Name]
				}
				copy := NewOperDecl(inside[declaration.Name()], parameters,
					renamer.rename(declaration.Body(), inside), declaration.Tag())
				copy.SetRecursive(declaration.IsRecursive())
				declarations[index] = renamer.tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(declaration)
			}
			return NewLetInExpr(renamer.rename(typed.Body(), inside), typed.Tag(), declarations...)
		default:
			return expression
		}
	})
	return tracked(expression)
}

func isSingleBinder(operator string) bool {
	switch operator {
	case "SET_FILTER", "EXISTS3", "FORALL3", "CHOOSE3", "CHOOSE2":
		return true
	default:
		return false
	}
}

func isMultipleBinder(operator string) bool {
	return operator == "SET_MAP" || operator == "FUN_CTOR" || operator == "FUN_REC_CTOR"
}

func (renamer *IncrementalRenamer) SyncFrom(expression Expr) {
	renamer.counters = MergeNameCounters(renamer.counters, NameCountersInExpression(expression, true), true)
}

func (renamer *IncrementalRenamer) ShiftCounters(expression Expr, offsets map[string]int) (Expr, error) {
	var transform ExprTransform
	var transformErr error
	transform = renamer.tracker.TrackExpression(func(expression Expr) Expr {
		if transformErr != nil {
			return expression
		}
		switch typed := expression.(type) {
		case *NameExpr:
			name, err := OffsetRenamedName(offsets, typed.Name())
			if err != nil {
				transformErr = err
				return typed
			}
			if name == typed.Name() {
				return typed
			}
			return NewNameExpr(name, typed.Tag())
		case *OperExpr:
			arguments := typed.Arguments()
			changed := false
			for index, argument := range arguments {
				shifted := transform(argument)
				arguments[index] = shifted
				changed = changed || !EqualExpr(argument, shifted)
			}
			if changed {
				return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			}
			return typed
		case *LetInExpr:
			body := transform(typed.Body())
			declarations := typed.Declarations()
			changed := !EqualExpr(body, typed.Body())
			for index, declaration := range declarations {
				name, err := OffsetRenamedName(offsets, declaration.Name())
				if err != nil {
					transformErr = err
					return typed
				}
				parameters := declaration.FormalParams()
				for parameterIndex := range parameters {
					parameters[parameterIndex].Name, err = OffsetRenamedName(offsets, parameters[parameterIndex].Name)
					if err != nil {
						transformErr = err
						return typed
					}
				}
				declarationBody := transform(declaration.Body())
				copy := NewOperDecl(name, parameters, declarationBody, declaration.Tag())
				copy.SetRecursive(declaration.IsRecursive())
				declarations[index] = renamer.tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(declaration)
				changed = changed || !EqualDecl(copy, declaration)
			}
			if changed {
				return NewLetInExpr(body, typed.Tag(), declarations...)
			}
			return typed
		default:
			return expression
		}
	})
	result := transform(expression)
	return result, transformErr
}

func (renamer *IncrementalRenamer) Normalize(expression Expr) (Expr, error) {
	offsets := normalizationOffsets(NameCountersInExpression(expression, false))
	return renamer.ShiftCounters(expression, offsets)
}

func normalizationOffsets(counters map[string]int) map[string]int {
	offsets := cloneMap(counters)
	for name, minimum := range offsets {
		if minimum > 1 {
			offsets[name] = minimum - 1
		} else {
			delete(offsets, name)
		}
	}
	return offsets
}

func (renamer *IncrementalRenamer) NormalizeExpressions(expressions []Expr) ([]Expr, error) {
	counters := make(map[string]int)
	for _, expression := range expressions {
		counters = MergeNameCounters(counters, NameCountersInExpression(expression, false), false)
	}
	offsets := normalizationOffsets(counters)
	result := make([]Expr, len(expressions))
	for index, expression := range expressions {
		var err error
		result[index], err = renamer.ShiftCounters(expression, offsets)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (renamer *IncrementalRenamer) NormalizeDeclaration(declaration Decl) (Decl, error) {
	result, err := renamer.NormalizeDeclarations([]Decl{declaration})
	if err != nil {
		return nil, err
	}
	return result[0], nil
}

func (renamer *IncrementalRenamer) NormalizeDeclarations(declarations []Decl) ([]Decl, error) {
	offsets := normalizationOffsets(NameCountersInDeclarations(declarations, false))
	result := append([]Decl(nil), declarations...)
	for index, declaration := range declarations {
		operator, ok := declaration.(*OperDecl)
		if !ok {
			continue
		}
		body, err := renamer.ShiftCounters(operator.Body(), offsets)
		if err != nil {
			return nil, err
		}
		copy := operator.copyWithTag(operator.Tag())
		copy.SetBody(body)
		result[index] = renamer.tracker.TrackDeclaration(func(Decl) Decl { return copy })(operator)
	}
	return result, nil
}

func (renamer *IncrementalRenamer) SyncAndNormalize(expression Expr) (Expr, error) {
	renamer.SyncFrom(expression)
	return renamer.Normalize(renamer.Rename(expression))
}

func (renamer *IncrementalRenamer) SyncAndNormalizeExpressions(expressions []Expr) ([]Expr, error) {
	for _, expression := range expressions {
		renamer.SyncFrom(expression)
	}
	renamed := make([]Expr, len(expressions))
	for index, expression := range expressions {
		renamed[index] = renamer.Rename(expression)
	}
	return renamer.NormalizeExpressions(renamed)
}

func (renamer *IncrementalRenamer) SyncAndNormalizeDeclaration(declaration Decl) (Decl, error) {
	result, err := renamer.SyncAndNormalizeDeclarations([]Decl{declaration})
	if err != nil {
		return nil, err
	}
	return result[0], nil
}

func (renamer *IncrementalRenamer) SyncAndNormalizeDeclarations(declarations []Decl) ([]Decl, error) {
	for _, declaration := range declarations {
		if operator, ok := declaration.(*OperDecl); ok {
			renamer.SyncFrom(operator.Body())
		}
	}
	renamed := append([]Decl(nil), declarations...)
	for index, declaration := range declarations {
		operator, ok := declaration.(*OperDecl)
		if !ok {
			continue
		}
		copy := operator.copyWithTag(operator.Tag())
		copy.SetBody(renamer.Rename(operator.Body()))
		renamed[index] = renamer.tracker.TrackDeclaration(func(Decl) Decl { return copy })(operator)
	}
	return renamer.NormalizeDeclarations(renamed)
}

func (renamer *IncrementalRenamer) RenameModule(module *Module) (*Module, error) {
	declarations, err := renamer.SyncAndNormalizeDeclarations(module.Declarations())
	if err != nil {
		return nil, err
	}
	result := NewModule(module.Name(), declarations)
	result.copyMetadataFrom(module)
	return result, nil
}
