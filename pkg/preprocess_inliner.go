package apalache

import "fmt"

type InlinerDeclarationFilter func(*OperDecl) bool

func KeepAllInlinedDeclarations(*OperDecl) bool { return true }
func KeepOnlyInlinedDeclarations(names map[string]struct{}) InlinerDeclarationFilter {
	return func(declaration *OperDecl) bool { _, found := names[declaration.Name()]; return found }
}

type Inliner struct {
	tracker         TransformationTracker
	renamer         *IncrementalRenamer
	keepNullaryMono bool
	moduleFilter    InlinerDeclarationFilter
}

func NewInliner(tracker TransformationTracker, renamer *IncrementalRenamer, keepNullaryMono bool, moduleFilter InlinerDeclarationFilter) *Inliner {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	if renamer == nil {
		renamer = NewIncrementalRenamer(tracker)
	}
	if moduleFilter == nil {
		moduleFilter = KeepAllInlinedDeclarations
	}
	return &Inliner{tracker: tracker, renamer: renamer, keepNullaryMono: keepNullaryMono, moduleFilter: moduleFilter}
}

func (inliner *Inliner) shouldInlineLocal(declaration *OperDecl) bool {
	typ, ok := declaration.Tag().Type()
	return !inliner.keepNullaryMono || ok && !typ.IsMono() || len(declaration.FormalParams()) > 0
}

func (inliner *Inliner) extend(scope map[string]*OperDecl, declaration *OperDecl, filter InlinerDeclarationFilter) map[string]*OperDecl {
	result := cloneMap(scope)
	if !declaration.IsRecursive() && filter(declaration) {
		result[declaration.Name()] = declaration
	}
	return result
}

func (inliner *Inliner) Transform(expression Expr) (Expr, error) {
	return inliner.transform(make(map[string]*OperDecl), expression)
}

func (inliner *Inliner) transform(scope map[string]*OperDecl, expression Expr) (Expr, error) {
	if expression == nil {
		return nil, fmt.Errorf("cannot inline a nil expression")
	}
	var result Expr
	switch typed := expression.(type) {
	case *OperExpr:
		arguments := typed.Arguments()
		if typed.Operator().Name() == "OPER_APP" && len(arguments) > 0 {
			if name, ok := arguments[0].(*NameExpr); ok {
				if callee, found := scope[name.Name()]; found {
					return inliner.instantiate(scope, name, arguments[1:], callee)
				}
			}
			if lambda, ok := arguments[0].(*LetInExpr); ok && isPassByNameLet(lambda) {
				declaration := lambda.Declarations()[0]
				if len(declaration.FormalParams()) == len(arguments)-1 {
					inside := inliner.extend(scope, declaration, func(*OperDecl) bool { return true })
					return inliner.instantiate(inside, lambda.Body().(*NameExpr), arguments[1:], declaration)
				}
			}
		}
		newArguments := make([]Expr, len(arguments))
		for index, argument := range arguments {
			transformed, err := inliner.transform(scope, argument)
			if err != nil {
				return nil, err
			}
			newArguments[index] = transformed
		}
		result = MustOperExpr(typed.Operator(), typed.Tag(), newArguments...)
	case *LetInExpr:
		passByName := isPassByNameLet(typed)
		keep := func(declaration *OperDecl) bool { return !inliner.shouldInlineLocal(declaration) }
		if passByName {
			keep = func(*OperDecl) bool { return true }
		}
		inside, remaining, err := inliner.pushDeclarations(scope, typed.Declarations(), keep, inliner.shouldInlineLocal)
		if err != nil {
			return nil, err
		}
		body := typed.Body()
		if !passByName {
			body, err = inliner.transform(inside, body)
			if err != nil {
				return nil, err
			}
		}
		if len(remaining) == 0 {
			result = body
		} else {
			result = NewLetInExpr(body, body.Tag(), remaining...)
		}
	case *NameExpr:
		callee, found := scope[typed.Name()]
		if !found {
			return expression, nil
		}
		embedded, err := inliner.embedPassByName(typed, callee)
		if err != nil {
			return nil, err
		}
		result = embedded
	default:
		return expression, nil
	}
	return inliner.tracker.TrackExpression(func(Expr) Expr { return result })(expression), nil
}

func (inliner *Inliner) freshBody(expression Expr) Expr {
	return inliner.renamer.Rename(NewDeepCopier(inliner.tracker).Expression(expression))
}

func (inliner *Inliner) instantiate(scope map[string]*OperDecl, callName *NameExpr, arguments []Expr, callee *OperDecl) (Expr, error) {
	parameters := callee.FormalParams()
	if len(parameters) != len(arguments) {
		return nil, fmt.Errorf("operator %s expects %d arguments, found %d", callee.Name(), len(parameters), len(arguments))
	}
	body := inliner.freshBody(callee.Body())
	parameterMap := make(map[string]Expr, len(parameters))
	for index, parameter := range parameters {
		parameterMap[parameter.Name] = arguments[index]
	}
	body = replaceNames(body, parameterMap, inliner.tracker)
	transformed, err := inliner.transform(scope, body)
	if err != nil {
		return nil, err
	}
	substitution, _, err := inliner.callSubstitution(callName.Tag(), callee)
	if err != nil {
		return nil, err
	}
	if substitution.IsEmpty() {
		return transformed, nil
	}
	return NewTypeSubstitutor(inliner.tracker, substitution).Expression(transformed)
}

func replaceNames(expression Expr, replacements map[string]Expr, tracker TransformationTracker) Expr {
	return RewriteExpressions(tracker, func(current Expr) (Expr, bool) {
		name, ok := current.(*NameExpr)
		if !ok {
			return nil, false
		}
		replacement, found := replacements[name.Name()]
		return replacement, found
	})(expression)
}

func (inliner *Inliner) callSubstitution(callTag TypeTag, callee *OperDecl) (*Substitution, Type, error) {
	callType, ok := callTag.Type()
	if !ok {
		return nil, nil, fmt.Errorf("untyped call to %s", callee.Name())
	}
	calleeType, ok := callee.Tag().Type()
	if !ok {
		return nil, nil, fmt.Errorf("untyped declaration %s", callee.Name())
	}
	maxVariable := 0
	for _, number := range append(calleeType.UsedNames(), callType.UsedNames()...) {
		if number > maxVariable {
			maxVariable = number
		}
	}
	pool := NewTypeVarPool(maxVariable + 1)
	originals := calleeType.UsedNames()
	freshEntries := make([]SubstitutionEntry, len(originals))
	for index, number := range originals {
		freshEntries[index] = SubstitutionEntry{Class: NewEqClass(number), Type: pool.Fresh()}
	}
	freshening, err := NewSubstitution(freshEntries...)
	if err != nil {
		return nil, nil, err
	}
	renamed, err := freshening.ApplyRecursive(calleeType)
	if err != nil {
		return nil, nil, err
	}
	unifier, unified, unifies, err := NewTypeUnifier(pool).Unify(EmptySubstitution(), callType, renamed)
	if err != nil {
		return nil, nil, err
	}
	if !unifies {
		return nil, nil, fmt.Errorf("inliner: unable to unify signature %s of %s with call-site type %s", calleeType, callee.Name(), callType)
	}
	resolved := make([]SubstitutionEntry, 0, len(originals))
	for index, number := range originals {
		final, applyErr := unifier.ApplyRecursive(freshEntries[index].Type)
		if applyErr != nil {
			return nil, nil, applyErr
		}
		resolved = append(resolved, SubstitutionEntry{Class: NewEqClass(number), Type: final})
	}
	result, err := NewSubstitution(resolved...)
	return result, unified, err
}

func (inliner *Inliner) embedPassByName(name *NameExpr, callee *OperDecl) (Expr, error) {
	body := inliner.freshBody(callee.Body())
	substitution, unified, err := inliner.callSubstitution(name.Tag(), callee)
	if err != nil {
		return nil, err
	}
	if !substitution.IsEmpty() {
		body, err = NewTypeSubstitutor(inliner.tracker, substitution).Expression(body)
		if err != nil {
			return nil, err
		}
	}
	newName := inliner.renamer.FreshName(callee.Name())
	declaration := NewOperDecl(newName, callee.FormalParams(), body, Typed(unified))
	return NewLetInExpr(NewNameExpr(newName, Typed(unified)), Typed(unified), declaration), nil
}

func isPassByNameLet(expression *LetInExpr) bool {
	if expression == nil || len(expression.Declarations()) != 1 {
		return false
	}
	name, ok := expression.Body().(*NameExpr)
	return ok && name.Name() == expression.Declarations()[0].Name()
}

func (inliner *Inliner) pushDeclarations(initial map[string]*OperDecl, declarations []*OperDecl, keep, scopeFilter InlinerDeclarationFilter) (map[string]*OperDecl, []*OperDecl, error) {
	scope := cloneMap(initial)
	remaining := make([]*OperDecl, 0, len(declarations))
	for _, declaration := range declarations {
		body, err := inliner.transform(scope, declaration.Body())
		if err != nil {
			return nil, nil, err
		}
		copy := declaration.copyWithTag(declaration.Tag())
		copy.SetBody(body)
		copy = inliner.tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(declaration)
		scope = inliner.extend(scope, copy, scopeFilter)
		if keep(copy) {
			remaining = append(remaining, copy)
		}
	}
	return scope, remaining, nil
}

func (inliner *Inliner) Module(module *Module) (*Module, error) {
	sorted, err := SortDeclarations(module)
	if err != nil {
		return nil, err
	}
	scope := make(map[string]*OperDecl)
	output := make([]Decl, 0, len(sorted.Declarations()))
	for _, declaration := range sorted.Declarations() {
		switch typed := declaration.(type) {
		case *OperDecl:
			body, transformErr := inliner.transform(scope, typed.Body())
			if transformErr != nil {
				return nil, transformErr
			}
			copy := typed.copyWithTag(typed.Tag())
			copy.SetBody(body)
			copy = inliner.tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(typed)
			scope = inliner.extend(scope, copy, func(*OperDecl) bool { return true })
			if inliner.moduleFilter(copy) {
				output = append(output, copy)
			}
		case *AssumeDecl:
			body, transformErr := inliner.transform(scope, typed.Body())
			if transformErr != nil {
				return nil, transformErr
			}
			var copy *AssumeDecl
			if name, named := typed.DefinedName(); named {
				copy = NewNamedAssumeDecl(name, body, typed.Tag())
				scope[name] = NewOperDecl(name, nil, body, Typed(mustOperator(nil, mustTagType(body.Tag()))))
			} else {
				copy = NewUnnamedAssumeDecl(body, typed.Tag())
			}
			output = append(output, copy)
		case *TheoremDecl:
			body, transformErr := inliner.transform(scope, typed.Body())
			if transformErr != nil {
				return nil, transformErr
			}
			output = append(output, NewTheoremDecl(typed.Name(), body, typed.Tag()))
		default:
			output = append(output, declaration)
		}
	}
	result := NewModule(module.Name(), output)
	result.copyMetadataFrom(module)
	return result, nil
}

func mustTagType(tag TypeTag) Type {
	typ, ok := tag.Type()
	if !ok {
		panic("expected typed tag")
	}
	return typ
}
