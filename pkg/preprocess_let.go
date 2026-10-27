package apalache

// OptimizeLetIn removes unused LET definitions. It assumes globally unique
// names, as established by IncrementalRenaming. The analysis intentionally
// matches Scala's conservative one-pass behavior rather than computing a full
// def-use closure.
func OptimizeLetIn(input TouchExpr, tracker TransformationTracker) TouchExpr {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	unused := make(map[string]struct{})
	var transform TouchTransform
	transform = tracker.TrackTouch(func(current TouchExpr) TouchExpr {
		expression := current.Expr
		switch typed := expression.(type) {
		case *ValueExpr, *nullExpr:
			return TouchExpr{Expr: expression}
		case *NameExpr:
			delete(unused, typed.Name())
			return TouchExpr{Expr: expression}
		case *OperExpr:
			arguments := typed.Arguments()
			changed := false
			for index, argument := range arguments {
				result := transform(TouchExpr{Expr: argument, Changed: true})
				arguments[index] = result.Expr
				changed = changed || result.Changed
			}
			if !changed {
				return TouchExpr{Expr: expression}
			}
			return TouchExpr{Expr: MustOperExpr(typed.Operator(), typed.Tag(), arguments...), Changed: true}
		case *LetInExpr:
			declarations := typed.Declarations()
			for _, declaration := range declarations {
				unused[declaration.Name()] = struct{}{}
			}
			body := transform(TouchExpr{Expr: typed.Body(), Changed: true})
			type declarationResult struct {
				declaration *OperDecl
				changed     bool
			}
			transformed := make([]declarationResult, 0, len(declarations))
			for _, declaration := range declarations {
				newBody := transform(TouchExpr{Expr: declaration.Body(), Changed: true})
				newDeclaration := declaration
				if newBody.Changed {
					copy := declaration.copyWithTag(declaration.Tag())
					copy.SetBody(newBody.Expr)
					newDeclaration = tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(declaration)
				}
				if _, isUnused := unused[newDeclaration.Name()]; !isUnused {
					transformed = append(transformed, declarationResult{declaration: newDeclaration, changed: newBody.Changed})
				}
			}
			changed := body.Changed || len(transformed) != len(declarations)
			newDeclarations := make([]*OperDecl, len(transformed))
			for index, result := range transformed {
				newDeclarations[index] = result.declaration
				changed = changed || result.changed
			}
			if !changed {
				return TouchExpr{Expr: expression}
			}
			if len(newDeclarations) == 0 {
				return TouchExpr{Expr: body.Expr, Changed: true}
			}
			return TouchExpr{Expr: NewLetInExpr(body.Expr, typed.Tag(), newDeclarations...), Changed: true}
		default:
			return TouchExpr{Expr: expression}
		}
	})
	return transform(input)
}
