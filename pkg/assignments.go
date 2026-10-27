package apalache

import (
	"fmt"
	"sort"
)

type AssignmentStrategy []UID

type SymbolicTransition struct {
	Assignments AssignmentStrategy
	Expression  Expr
}

type AssignmentError struct{ Message string }

func (err *AssignmentError) Error() string { return err.Message }

// AssignmentOperatorIntroduction replaces selected x' = e candidates with
// Apalache!:= and records the fresh UID assigned to each replacement.
type AssignmentOperatorIntroduction struct {
	isAssignment func(UID) bool
	tracker      TransformationTracker
	replacements map[UID]UID
}

func NewAssignmentOperatorIntroduction(isAssignment func(UID) bool, tracker TransformationTracker) *AssignmentOperatorIntroduction {
	if isAssignment == nil {
		isAssignment = func(UID) bool { return false }
	}
	if tracker == nil {
		tracker = IdleTracker{}
	}
	return &AssignmentOperatorIntroduction{isAssignment: isAssignment, tracker: tracker, replacements: make(map[UID]UID)}
}

func NewAssignmentOperatorIntroductionFromSet(assignments map[UID]struct{}, tracker TransformationTracker) *AssignmentOperatorIntroduction {
	return NewAssignmentOperatorIntroduction(func(id UID) bool {
		_, found := assignments[id]
		return found
	}, tracker)
}

func (introduction *AssignmentOperatorIntroduction) Expression(expression Expr) Expr {
	if expression == nil {
		panic("cannot introduce assignments in a nil expression")
	}
	var transform ExprTransform
	transform = introduction.tracker.TrackExpression(func(current Expr) Expr {
		switch typed := current.(type) {
		case *OperExpr:
			arguments := typed.Arguments()
			if typed.Operator().Name() == "EQ" && introduction.isAssignment(typed.ID()) && isPrimedName(arguments[0]) {
				replacement := MustOperExpr(Op("Apalache!:="), typed.Tag(), arguments...)
				introduction.replacements[typed.ID()] = replacement.ID()
				return replacement
			}
			if typed.Operator().Name() == "LABEL" {
				body := transform(arguments[0])
				if EqualExpr(body, arguments[0]) {
					return typed
				}
				arguments[0] = body
				return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
			}
			changed := false
			for index, argument := range arguments {
				arguments[index] = transform(argument)
				changed = changed || !EqualExpr(argument, arguments[index])
			}
			if !changed {
				return typed
			}
			return MustOperExpr(typed.Operator(), typed.Tag(), arguments...)
		case *LetInExpr:
			declarations := typed.Declarations()
			changed := false
			for index, declaration := range declarations {
				body := transform(declaration.Body())
				if !EqualExpr(body, declaration.Body()) {
					copy := declaration.copyWithTag(declaration.Tag())
					copy.SetBody(body)
					declarations[index] = copy
					changed = true
				}
			}
			body := transform(typed.Body())
			changed = changed || !EqualExpr(body, typed.Body())
			if !changed {
				return typed
			}
			return NewLetInExpr(body, typed.Tag(), declarations...)
		default:
			return current
		}
	})
	return transform(expression)
}

func (introduction *AssignmentOperatorIntroduction) Replacements() map[UID]UID {
	return cloneMap(introduction.replacements)
}

func isPrimedName(expression Expr) bool {
	prime, ok := expression.(*OperExpr)
	if !ok || prime.Operator().Name() != "PRIME" {
		return false
	}
	_, named := prime.Arguments()[0].(*NameExpr)
	return named
}

// ExpressionToOperatorDefinition wraps an expression in a nullary operator.
func ExpressionToOperatorDefinition(name string, expression Expr) *OperDecl {
	if typ, typed := expression.Tag().Type(); typed {
		return NewOperDecl(name, nil, expression, Typed(mustOperator(nil, typ)))
	}
	return NewOperDecl(name, nil, expression, Untyped())
}

// ExpressionsToOperatorDefinitions uses zero-padded suffixes so lexical and
// numeric transition ordering coincide.
func ExpressionsToOperatorDefinitions(prefix string, expressions []Expr) []*OperDecl {
	result := make([]*OperDecl, len(expressions))
	for index, expression := range expressions {
		result[index] = ExpressionToOperatorDefinition(fmt.Sprintf("%s%04d", prefix, index), expression)
	}
	return result
}

func DeclarationsFromTransitions(prefix string, transitions []SymbolicTransition) []*OperDecl {
	expressions := make([]Expr, len(transitions))
	for index, transition := range transitions {
		expressions[index] = transition.Expression
	}
	return ExpressionsToOperatorDefinitions(prefix, expressions)
}

func InsertTransitions(module *Module, prefix string, transitions []SymbolicTransition) *Module {
	inserted := DeclarationsFromTransitions(prefix, transitions)
	declarations := make([]Decl, 0, len(inserted)+len(module.Declarations()))
	for _, declaration := range inserted {
		declarations = append(declarations, declaration)
	}
	declarations = append(declarations, module.Declarations()...)
	result := NewModule(module.Name(), declarations)
	result.properties = cloneMap(module.properties)
	return result
}

func TransitionsFromModule(module *Module, prefix string) []Expr {
	declarations := make([]*OperDecl, 0)
	for _, declaration := range module.OperDeclarations() {
		if len(declaration.Name()) >= len(prefix) && declaration.Name()[:len(prefix)] == prefix {
			declarations = append(declarations, declaration)
		}
	}
	sort.Slice(declarations, func(left, right int) bool { return declarations[left].Name() < declarations[right].Name() })
	result := make([]Expr, len(declarations))
	for index, declaration := range declarations {
		result[index] = declaration.Body()
	}
	return result
}

func OperatorBody(module *Module, name string) (Expr, bool) {
	for _, declaration := range module.OperDeclarations() {
		if declaration.Name() == name {
			return declaration.Body(), true
		}
	}
	return nil, false
}
