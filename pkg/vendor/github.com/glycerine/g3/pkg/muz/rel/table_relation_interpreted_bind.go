package rel

import "github.com/glycerine/g3/pkg/ast"

func tableRelationBindingSize(sorts map[uint32]ast.ASTHandle) int {
	var maximum uint32
	found := false
	for variable := range sorts {
		if !found || variable > maximum {
			maximum, found = variable, true
		}
	}
	if !found {
		return 0
	}
	return int(maximum) + 1
}

func tableRelationBindingIndex(size int, variable uint32) int {
	return size - int(variable) - 1
}
