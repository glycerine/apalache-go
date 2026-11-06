package model

// Source: src/model/func_interp.cpp:234-255; src/util/obj_hashtable.h.

import "github.com/glycerine/g3/pkg/ast"

// MaxOccurrenceResult uses the source running count and strict greater-than
// comparison. Ties retain whichever result first reached that running maximum.
func (f *FunctionInterpretation) MaxOccurrenceResult() (ast.ASTHandle, error) {
	if e := f.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if len(f.entries) == 0 {
		return ast.ASTHandle{}, nil
	}
	counts := make(map[ast.ASTHandle]uint32)
	var result ast.ASTHandle
	max := uint32(0)
	for _, entry := range f.entries {
		count := counts[entry.result] + 1
		counts[entry.result] = count
		if count > max {
			max = count
			result = entry.result
		}
	}
	return result, nil
}
