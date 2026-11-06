package arith

// Source: src/tactic/arith/fm_tactic.cpp:1277-1441.
import (
	"github.com/glycerine/g3/pkg/ast"
)

type FMResolution struct {
	Constraint   *FMConstraint
	Dependency   ast.ASTDependency
	Tautology    bool
	Inconsistent bool
}

func FMResolve(store *FMConstraintStore, variables *FMVariableTable, lower, upper *FMConstraint, x uint32) (FMResolution, error) {
	if store == nil || variables == nil || lower == nil || upper == nil || lower.Dead || upper.Dead {
		return FMResolution{}, ast.ErrASTDomain
	}
	xs, coefficients, constant, strict, err := fmResolvePolynomial(variables, lower, upper, x)
	if err != nil {
		return FMResolution{}, err
	}
	if len(xs) == 0 && (constant.Sign() > 0 || !strict && constant.Sign() == 0) {
		return FMResolution{Tautology: true}, store.diagnostics.resolution(x, lower, upper, nil, " consequent literal is always true: ", 1366)
	}
	literals, tautology := fmMergeLiterals(lower.Literals, upper.Literals)
	if tautology {
		return FMResolution{Tautology: true}, store.diagnostics.resolution(x, lower, upper, nil, " tautology: ", 1410)
	}
	dependency, err := store.manager.DependencyJoin(lower.Dependency, upper.Dependency)
	if err != nil {
		return FMResolution{}, err
	}
	if len(literals) == 0 && len(xs) == 0 && (constant.Sign() < 0 || strict && constant.Sign() == 0) {
		return FMResolution{Dependency: dependency, Inconsistent: true}, store.diagnostics.resolution(x, lower, upper, nil, " inconsistent: ", 1420)
	}
	c, err := store.New(literals, xs, coefficients, &constant, strict, dependency)
	if err != nil {
		return FMResolution{}, err
	}
	return FMResolution{Constraint: c, Dependency: dependency}, store.diagnostics.resolution(x, lower, upper, c, "", 1438)
}
