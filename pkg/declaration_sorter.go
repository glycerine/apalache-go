package apalache

import (
	"fmt"
	"strings"
)

type CyclicDependencyError struct {
	Declarations []string
	Witnesses    map[UID]struct{}
}

func (err *CyclicDependencyError) Error() string {
	return "found a cyclic dependency among operators: " + strings.Join(err.Declarations, ", ")
}

// SortDeclarations restores the define-before-use order of module
// declarations. It preserves the original order whenever dependencies permit.
func SortDeclarations(module *Module) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot sort declarations of a nil module")
	}
	declarations := module.Declarations()
	idToDeclaration := make(map[UID]Decl, len(declarations))
	ids := make([]UID, len(declarations))
	for index, declaration := range declarations {
		idToDeclaration[declaration.ID()] = declaration
		ids[index] = declaration.ID()
	}

	dependencies := declarationDependencies(declarations)
	sortedIDs, witnesses, err := StableTopologicalSort(dependencies, ids)
	if err != nil {
		return nil, err
	}
	if witnesses != nil {
		names := make([]string, 0, len(witnesses))
		for _, declaration := range declarations {
			if _, found := witnesses[declaration.ID()]; found {
				names = append(names, declaration.Name())
			}
		}
		return nil, &CyclicDependencyError{Declarations: names, Witnesses: witnesses}
	}

	sorted := make([]Decl, len(sortedIDs))
	for index, id := range sortedIDs {
		sorted[index] = idToDeclaration[id]
	}
	result := NewModule(module.Name(), sorted)
	result.properties = cloneMap(module.properties)
	return result, nil
}

func declarationDependencies(declarations []Decl) map[UID]map[UID]struct{} {
	nameToID := make(map[string]UID, len(declarations))
	dependencies := make(map[UID]map[UID]struct{}, len(declarations))
	for _, declaration := range declarations {
		nameToID[declaration.Name()] = declaration.ID()
		dependencies[declaration.ID()] = make(map[UID]struct{})
	}
	for _, declaration := range declarations {
		var uses map[UID]struct{}
		switch typed := declaration.(type) {
		case *AssumeDecl:
			uses = findExpressionUses(nameToID, typed.Body())
		case *OperDecl:
			visible := cloneMap(nameToID)
			for _, parameter := range typed.FormalParams() {
				delete(visible, parameter.Name)
			}
			uses = findExpressionUses(visible, typed.Body())
		default:
			continue
		}
		delete(uses, declaration.ID())
		dependencies[declaration.ID()] = uses
	}
	return dependencies
}

func findExpressionUses(nameToID map[string]UID, expression Expr) map[UID]struct{} {
	uses := make(map[UID]struct{})
	var visit func(map[string]UID, Expr)
	visit = func(visible map[string]UID, current Expr) {
		switch typed := current.(type) {
		case *NameExpr:
			if id, found := visible[typed.Name()]; found {
				uses[id] = struct{}{}
			}
		case *OperExpr:
			arguments := typed.Arguments()
			if typed.Operator().Name() == "OPER_APP" && len(arguments) > 0 {
				if name, ok := arguments[0].(*NameExpr); ok {
					if id, found := visible[name.Name()]; found {
						uses[id] = struct{}{}
					}
					arguments = arguments[1:]
				}
			}
			for _, argument := range arguments {
				visit(visible, argument)
			}
		case *LetInExpr:
			visit(visible, typed.Body())
			for _, declaration := range typed.Declarations() {
				inside := cloneMap(visible)
				for _, parameter := range declaration.FormalParams() {
					delete(inside, parameter.Name)
				}
				visit(inside, declaration.Body())
			}
		}
	}
	visit(nameToID, expression)
	return uses
}
