package base

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrEngineBase = errors.New("muz/base: invalid or closed engine")

type EngineQueryCore interface {
	Query(ast.ASTHandle) (lbool.LBool, error)
	Answer() (ast.ASTRef, error)
}

type EngineBase struct {
	manager *ast.ASTManager
	name    string
	core    EngineQueryCore
	closed  bool
}

func NewEngineBase(manager *ast.ASTManager, name string, core EngineQueryCore) (*EngineBase, error) {
	if manager == nil || core == nil || name == "" {
		return nil, ErrEngineBase
	}
	if _, e := manager.ProofsEnabled(); e != nil {
		return nil, e
	}
	return &EngineBase{manager: manager, name: name, core: core}, nil
}
func (e *EngineBase) open() error {
	if e == nil || e.closed || e.manager == nil || e.core == nil {
		return ErrEngineBase
	}
	return nil
}
func (e *EngineBase) Name() (string, error) {
	if x := e.open(); x != nil {
		return "", x
	}
	return e.name, nil
}
func (e *EngineBase) unsupported(operation string) error {
	return fmt.Errorf("muz/base: %s is not supported for %s", operation, e.name)
}
func (e *EngineBase) Query(q ast.ASTHandle) (lbool.LBool, error) {
	if x := e.open(); x != nil {
		return lbool.Undef, x
	}
	return e.core.Query(q)
}
func (e *EngineBase) Answer() (ast.ASTRef, error) {
	if x := e.open(); x != nil {
		return ast.ASTRef{}, x
	}
	return e.core.Answer()
}
