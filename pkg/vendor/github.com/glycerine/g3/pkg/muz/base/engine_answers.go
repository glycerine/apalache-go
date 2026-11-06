package base

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (e *EngineBase) GroundSATAnswer() (ast.ASTRef, error) {
	if x := e.open(); x != nil {
		return ast.ASTRef{}, x
	}
	if capability, ok := e.core.(EngineGroundAnswer); ok {
		return capability.GroundSATAnswer()
	}
	return ast.ASTRef{}, e.unsupported("ground SAT answer")
}
func (e *EngineBase) Model() (*model.Model, error) {
	if x := e.open(); x != nil {
		return nil, x
	}
	if capability, ok := e.core.(EngineModelProvider); ok {
		return capability.Model()
	}
	return model.NewModel(e.manager)
}
func (e *EngineBase) Proof() (ast.ASTRef, error) {
	if x := e.open(); x != nil {
		return ast.ASTRef{}, x
	}
	if capability, ok := e.core.(EngineProofProvider); ok {
		return capability.Proof()
	}
	truth, err := e.manager.MkTrue()
	if err != nil {
		return ast.ASTRef{}, err
	}
	proof, err := e.manager.MkProofAsserted(truth)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return e.manager.NewRef(proof)
}
