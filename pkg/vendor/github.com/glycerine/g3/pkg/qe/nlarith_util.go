package qe

// Source: src/qe/nlarith_util.{h,cpp} nlarith::util.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type NonlinearArithmeticUtil struct {
	self         *NonlinearArithmeticUtil
	manager      *ast.ASTManager
	term         *ArithmeticQETermUtil
	family       ast.FamilyID
	enableLinear bool
	closed       bool
}

func NewNonlinearArithmeticUtil(manager *ast.ASTManager) (*NonlinearArithmeticUtil, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	term, err := NewArithmeticQETermUtil(manager)
	if err != nil {
		return nil, err
	}
	u := &NonlinearArithmeticUtil{manager: manager, term: term, family: term.family}
	u.self = u
	return u, nil
}
func (u *NonlinearArithmeticUtil) open() error {
	if u == nil || u.self != u {
		return ast.ErrASTHandle
	}
	if u.closed {
		return arena.ErrClosed
	}
	if u.manager == nil || u.term == nil {
		return ast.ErrASTHandle
	}
	return nil
}
func (u *NonlinearArithmeticUtil) SetEnableLinear(enable bool) error {
	if err := u.open(); err != nil {
		return err
	}
	u.enableLinear = enable
	return nil
}
func (u *NonlinearArithmeticUtil) Close() error {
	if u == nil || u.self != u {
		return ast.ErrASTHandle
	}
	if u.closed {
		return nil
	}
	err := u.term.Close()
	if err == nil {
		u.closed = true
		u.manager = nil
		u.term = nil
	}
	return err
}
func (u *NonlinearArithmeticUtil) numeral(h ast.ASTHandle) (*big.Rat, bool, error) {
	return u.term.numeral(h)
}
