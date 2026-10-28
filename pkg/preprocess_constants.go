package apalache

import "fmt"

// TLAConstantInliner extracts primitive constant assignments and replaces
// references to configured module constants.
type TLAConstantInliner struct {
	tracker   TransformationTracker
	constants map[string]struct{}
}

func NewTLAConstantInliner(tracker TransformationTracker, constants []string) *TLAConstantInliner {
	if tracker == nil {
		tracker = IdleTracker{}
	}
	set := make(map[string]struct{}, len(constants))
	for _, name := range constants {
		set[name] = struct{}{}
	}
	return &TLAConstantInliner{tracker: tracker, constants: set}
}

func (inliner *TLAConstantInliner) BuildConstantMap(initial map[string]*ValueExpr, expression Expr) (map[string]*ValueExpr, error) {
	result := make(map[string]*ValueExpr, len(initial))
	for name, value := range initial {
		result[name] = value
	}
	var visit func(Expr) error
	visit = func(current Expr) error {
		switch typed := current.(type) {
		case *OperExpr:
			arguments := typed.Arguments()
			if typed.Operator().Name() == "EQ" {
				name, named := arguments[0].(*NameExpr)
				value, literal := arguments[1].(*ValueExpr)
				if named && literal {
					if _, configured := inliner.constants[name.Name()]; configured {
						if old, exists := result[name.Name()]; exists && (!EqualValues(old.Value(), value.Value()) || !old.Tag().Equal(value.Tag())) {
							return NewTlaInputError(fmt.Sprintf("Constant %s has multiple initializations: %v and %v.", name.Name(), value.Value(), old.Value()))
						}
						result[name.Name()] = value
					}
				}
			}
			for _, argument := range arguments {
				if err := visit(argument); err != nil {
					return err
				}
			}
		case *LetInExpr:
			for _, declaration := range typed.Declarations() {
				if err := visit(declaration.Body()); err != nil {
					return err
				}
			}
			return visit(typed.Body())
		}
		return nil
	}
	if err := visit(expression); err != nil {
		return nil, err
	}
	return result, nil
}

func (inliner *TLAConstantInliner) Replace(expression Expr, values map[string]*ValueExpr) Expr {
	var transform ExprTransform
	transform = inliner.tracker.TrackExpression(func(current Expr) Expr {
		switch typed := current.(type) {
		case *NameExpr:
			if _, configured := inliner.constants[typed.Name()]; configured {
				if value, found := values[typed.Name()]; found {
					return NewValueExpr(value.Value(), value.Tag())
				}
			}
			return current
		case *OperExpr:
			arguments := typed.Arguments()
			changed := false
			for index, argument := range arguments {
				arguments[index] = transform(argument)
				changed = changed || arguments[index].ID() != argument.ID()
			}
			if changed {
				return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			}
			return current
		case *LetInExpr:
			declarations := typed.Declarations()
			changed := false
			for index, declaration := range declarations {
				body := transform(declaration.Body())
				if body.ID() != declaration.Body().ID() {
					copy := declaration.copyWithTag(declaration.Tag())
					copy.SetBody(body)
					declarations[index] = inliner.tracker.TrackOperatorDeclaration(func(*OperDecl) *OperDecl { return copy })(declaration)
					changed = true
				}
			}
			body := transform(typed.Body())
			changed = changed || body.ID() != typed.Body().ID()
			if changed {
				return NewLetInExpr(body, typed.Tag(), declarations...)
			}
			return current
		default:
			return current
		}
	})
	return transform(expression)
}
