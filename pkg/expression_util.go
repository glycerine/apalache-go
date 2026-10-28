package apalache

import "fmt"

func isFoldOperator(name string) bool {
	return name == "Apalache!ApaFoldSet" || name == "Apalache!ApaFoldSeqLeft"
}

func embeddedOperatorBody(expression Expr) (Expr, bool) {
	let, ok := expression.(*LetInExpr)
	if !ok {
		return nil, false
	}
	declarations := let.Declarations()
	if len(declarations) != 1 {
		return nil, false
	}
	return declarations[0].Body(), true
}

// FindUsedNames returns the names referenced by an expression. It preserves
// the Scala utility's specialized treatment of primes and embedded operator
// bodies used by folds, Repeat, and MkSeq.
func FindUsedNames(expression Expr) (map[string]struct{}, error) {
	used := make(map[string]struct{})
	var visit func(Expr) error
	visit = func(current Expr) error {
		switch typed := current.(type) {
		case *NameExpr:
			used[typed.Name()] = struct{}{}
		case *OperExpr:
			arguments := typed.Arguments()
			name := typed.Operator().Name()
			if name == "PRIME" && len(arguments) == 1 {
				if primed, ok := arguments[0].(*NameExpr); ok {
					used[primed.Name()+"'"] = struct{}{}
					return nil
				}
			}
			if isFoldOperator(name) && len(arguments) == 3 {
				if body, ok := embeddedOperatorBody(arguments[0]); ok {
					if err := visit(body); err != nil {
						return err
					}
					for _, argument := range arguments[1:] {
						if err := visit(argument); err != nil {
							return err
						}
					}
					return nil
				}
			}
			if name == "Apalache!Repeat" && len(arguments) == 3 {
				if body, ok := embeddedOperatorBody(arguments[0]); ok {
					if err := visit(body); err != nil {
						return err
					}
					for _, argument := range arguments[1:] {
						if err := visit(argument); err != nil {
							return err
						}
					}
					return nil
				}
			}
			if name == "Apalache!MkSeq" && len(arguments) == 2 {
				if body, ok := embeddedOperatorBody(arguments[1]); ok {
					if err := visit(body); err != nil {
						return err
					}
					return visit(arguments[0])
				}
			}
			for _, argument := range arguments {
				if err := visit(argument); err != nil {
					return err
				}
			}
		case *LetInExpr:
			for _, declaration := range typed.Declarations() {
				parameters := declaration.FormalParams()
				if len(parameters) != 0 {
					return fmt.Errorf("operator %s: expected 0 parameters, found %d parameters",
						declaration.Name(), len(parameters))
				}
				if err := visit(declaration.Body()); err != nil {
					return err
				}
			}
			return visit(typed.Body())
		}
		return nil
	}
	if expression == nil {
		return used, nil
	}
	return used, visit(expression)
}

// FindLabels returns label names in the preorder used by the Scala utility.
// Label parameters do not contribute additional labels.
func FindLabels(expression Expr) []string {
	if expression == nil {
		return nil
	}
	switch typed := expression.(type) {
	case *OperExpr:
		arguments := typed.Arguments()
		if typed.Operator().Name() == "LABEL" && len(arguments) >= 2 {
			if value, ok := arguments[1].(*ValueExpr); ok {
				if label, ok := value.Value().(StringValue); ok {
					return append([]string{string(label)}, FindLabels(arguments[0])...)
				}
			}
		}
		var labels []string
		for _, argument := range arguments {
			labels = append(labels, FindLabels(argument)...)
		}
		return labels
	case *LetInExpr:
		var labels []string
		for _, declaration := range typed.Declarations() {
			labels = append(labels, FindLabels(declaration.Body())...)
		}
		return append(labels, FindLabels(typed.Body())...)
	default:
		return nil
	}
}

type ExprToLabelKind interface{ expressionLabelKind() }

type InitTransKind struct{ Index int }
type NextTransKind struct{ Index int }
type VCKind struct{ Index int }

func (InitTransKind) expressionLabelKind() {}
func (NextTransKind) expressionLabelKind() {}
func (VCKind) expressionLabelKind()        {}

// LabelsCache caches one label sequence per transition/invariant kind. Like
// Scala's cache, the expression is deliberately not part of the key.
type LabelsCache struct{ labels map[ExprToLabelKind][]string }

func NewLabelsCache() *LabelsCache {
	return &LabelsCache{labels: make(map[ExprToLabelKind][]string)}
}

func (cache *LabelsCache) GetLabels(kind ExprToLabelKind, expression Expr) []string {
	if cache == nil {
		return FindLabels(expression)
	}
	if cache.labels == nil {
		cache.labels = make(map[ExprToLabelKind][]string)
	}
	if labels, found := cache.labels[kind]; found {
		return append([]string(nil), labels...)
	}
	labels := FindLabels(expression)
	cache.labels[kind] = append([]string(nil), labels...)
	return append([]string(nil), labels...)
}
