package rel

import "github.com/glycerine/g3/pkg/ast"

// SourceRecursivePredicateOrder applies dl_compiler's detect_chains to the
// supplied predicate graph. The ordered slices preserve the established Go
// traversal order; dependencies outside heads do not participate in the SCC.
func SourceRecursivePredicateOrder(heads []ast.ASTHandle, dependencies map[ast.ASTHandle][]ast.ASTHandle) ([]ast.ASTHandle, error) {
	keys := make([]finitePredicateOrderKey, 0, len(heads))
	predicates := make(map[finitePredicateOrderKey]ast.ASTHandle, len(heads))
	edges := make(map[finitePredicateOrderKey][]finitePredicateOrderKey, len(heads))
	for _, head := range heads {
		key := finitePredicateKey(head)
		if head.IsNull() {
			return nil, ErrCompilerUnsupported
		}
		if _, duplicate := predicates[key]; duplicate {
			return nil, ErrCompilerUnsupported
		}
		keys = append(keys, key)
		predicates[key] = head
		seen := make(map[finitePredicateOrderKey]bool)
		for _, dependency := range dependencies[head] {
			dependencyKey := finitePredicateKey(dependency)
			if !seen[dependencyKey] {
				seen[dependencyKey] = true
				edges[key] = append(edges[key], dependencyKey)
			}
		}
	}
	ordered, _ := sourceDependencyOrder(keys, edges)
	if len(ordered) != len(heads) {
		return nil, ErrCompilerUnsupported
	}
	result := make([]ast.ASTHandle, 0, len(ordered))
	for _, key := range ordered {
		result = append(result, predicates[key])
	}
	return result, nil
}
