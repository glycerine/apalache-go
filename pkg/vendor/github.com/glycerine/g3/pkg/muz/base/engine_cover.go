package base

import "github.com/glycerine/g3/pkg/ast"

func (e *EngineBase) NumLevels(predicate ast.ASTHandle) (uint32, error) {
	if x := e.open(); x != nil {
		return 0, x
	}
	if capability, ok := e.core.(EngineLevelCounter); ok {
		return capability.NumLevels(predicate)
	}
	return 0, e.unsupported("get_num_levels")
}
func (e *EngineBase) Reachable(predicate ast.ASTHandle) (ast.ASTRef, error) {
	if x := e.open(); x != nil {
		return ast.ASTRef{}, x
	}
	if capability, ok := e.core.(EngineReachability); ok {
		return capability.Reachable(predicate)
	}
	return ast.ASTRef{}, e.unsupported("reachable")
}
func (e *EngineBase) CoverDelta(level int32, predicate ast.ASTHandle) (ast.ASTRef, error) {
	if x := e.open(); x != nil {
		return ast.ASTRef{}, x
	}
	if capability, ok := e.core.(EngineCover); ok {
		return capability.CoverDelta(level, predicate)
	}
	return ast.ASTRef{}, e.unsupported("cover delta")
}
func (e *EngineBase) AddCover(level int32, predicate, property ast.ASTHandle) error {
	if x := e.open(); x != nil {
		return x
	}
	if capability, ok := e.core.(EngineCover); ok {
		return capability.AddCover(level, predicate, property)
	}
	return e.unsupported("add cover")
}
func (e *EngineBase) AddInvariant(predicate, property ast.ASTHandle) error {
	if x := e.open(); x != nil {
		return x
	}
	if capability, ok := e.core.(EngineInvariant); ok {
		return capability.AddInvariant(predicate, property)
	}
	return e.unsupported("add invariant")
}
