package model

// Source: src/model/model_core.cpp:21-109.

import "github.com/glycerine/g3/pkg/ast"

// Entries track real AST/interpretation ownership independently of dictionary
// storage, including interrupted registration and destruction.
type modelConstantEntry struct {
	declaration, value, pending  ast.ASTHandle
	declarationOwned, valueOwned bool
}

type modelFunctionEntry struct {
	declaration      ast.ASTHandle
	function         *FunctionInterpretation
	declarationOwned bool
}

func modelHandleHash(h ast.ASTHandle) uint64 {
	return h.Origin() ^ uint64(h.Slot())*0x9e3779b97f4a7c15 ^ uint64(h.Generation())*0xbf58476d1ce4e5b9
}

func (md *ModelCore) retireConstant(entry *modelConstantEntry) error {
	if !entry.pending.IsNull() {
		consumed, e := md.manager.DecRefConsumed(entry.pending)
		if consumed {
			entry.pending = ast.ASTHandle{}
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	if entry.declarationOwned {
		consumed, e := md.manager.DecRefConsumed(entry.declaration)
		if consumed {
			entry.declarationOwned = false
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	if entry.valueOwned {
		consumed, e := md.manager.DecRefConsumed(entry.value)
		if consumed {
			entry.valueOwned = false
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	return nil
}

func (md *ModelCore) retireFunction(entry *modelFunctionEntry, deallocationLine int) error {
	if entry.declarationOwned {
		consumed, e := md.manager.DecRefConsumed(entry.declaration)
		if consumed {
			entry.declarationOwned = false
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	if entry.function != nil {
		function := entry.function
		if e := function.Close(); e != nil {
			return e
		}
		function.traceEvaluatorFunctionDeallocated(deallocationLine)
		entry.function = nil
	}
	return nil
}

func (md *ModelCore) finishConstantRemoval() error {
	if md.removedConstant != nil {
		if e := md.retireConstant(md.removedConstant); e != nil {
			return e
		}
		md.removedConstant = nil
	}
	return nil
}
