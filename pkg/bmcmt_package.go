package apalache

import (
	"fmt"
	"strings"
)

// Cross returns the left-major Cartesian product used by the Scala
// implicitConversions.Crossable helper.
func Cross[X, Y any](left []X, right []Y) []Pair[X, Y] {
	result := make([]Pair[X, Y], 0, len(left)*len(right))
	for _, x := range left {
		for _, y := range right {
			result = append(result, Pair[X, Y]{First: x, Second: y})
		}
	}
	return result
}

type Theory interface {
	HasConst(string) bool
}

func TheoryHasNameExpr(theory Theory, expression Expr) bool {
	name, ok := expression.(*NameExpr)
	return ok && theory != nil && theory.HasConst(name.Name())
}

func TheoryNameExprToString(theory Theory, expression Expr) (string, error) {
	if name, ok := expression.(*NameExpr); ok && theory != nil && theory.HasConst(name.Name()) {
		return name.Name(), nil
	}
	text, err := FormatTLAExpression(expression)
	if err != nil {
		text = fmt.Sprintf("%T", expression)
	}
	return "", NewCheckerException("Expected a cell, found: "+text, expression)
}

type CellTheory struct{}

func (CellTheory) HasConst(name string) bool { return strings.HasPrefix(name, ArenaCellNamePrefix) }
func (CellTheory) String() string            { return "Cell" }
