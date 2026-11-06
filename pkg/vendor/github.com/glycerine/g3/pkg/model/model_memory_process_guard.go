package model

// Source: src/util/memory_manager.cpp:202-211.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// Model allocations borrow the AST numeral process even when their trace
// stream has no process binding, as standalone AST managers can do.
func modelMemoryFinalizing(m *ast.ASTManager, tags *diagnostic.TraceTags) bool {
	if tags.MemoryFinalizing() {
		return true
	}
	if m == nil {
		return false
	}
	numerals := m.NumeralService()
	if numerals == nil {
		return false
	}
	if runtime := numerals.RuntimeOwner(); runtime != nil {
		if process := runtime.ProcessOwner(); process != nil {
			return process.Finalizing()
		}
	}
	return false
}
