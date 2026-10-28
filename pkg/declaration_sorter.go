package apalache

import (
	"fmt"
	"os"
	"strings"
)

type CyclicDependencyError struct {
	Declarations []string
	Witnesses    map[UID]struct{}
	message      string
}

func (err *CyclicDependencyError) Error() string {
	if err.message != "" {
		return err.message
	}
	return "found a cyclic dependency among operators: " + strings.Join(err.Declarations, ", ")
}

func NewCyclicDependencyError(message string) *CyclicDependencyError {
	return &CyclicDependencyError{message: message}
}

func (err *CyclicDependencyError) Unwrap() error { return NewLirError(err.Error()) }

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
		if err := writeDependencyGraph("dependencies.dot", declarations, dependencies, witnesses); err != nil {
			return nil, fmt.Errorf("write cyclic dependency graph: %w", err)
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

// writeDependencyGraph mirrors DeclarationSorter's Scala cycle diagnostic.
// Declaration order makes the otherwise set-valued graph reproducible in Go.
func writeDependencyGraph(filename string, declarations []Decl, dependencies map[UID]map[UID]struct{}, witnesses map[UID]struct{}) error {
	var document strings.Builder
	document.WriteString("digraph dependencies {\n")
	for _, declaration := range declarations {
		from := declaration.ID()
		if _, found := witnesses[from]; !found {
			continue
		}
		fmt.Fprintf(&document, "  n%s[label=\"%s\"];\n", from, declaration.Name())
		for _, dependency := range declarations {
			to := dependency.ID()
			if _, retained := witnesses[to]; !retained {
				continue
			}
			if _, found := dependencies[from][to]; found {
				fmt.Fprintf(&document, "  n%s -> n%s;\n", from, to)
			}
		}
	}
	document.WriteString("}\n")
	return os.WriteFile(filename, []byte(document.String()), 0o666)
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
