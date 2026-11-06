package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// NLPolynomial owns coefficients in increasing degree order.
type NLPolynomial struct {
	manager      *ast.ASTManager
	coefficients []ast.ASTRef
	closed       bool
}

func newNLPolynomial(m *ast.ASTManager, handles []ast.ASTHandle) (*NLPolynomial, error) {
	p := &NLPolynomial{manager: m}
	for _, h := range handles {
		r, err := m.NewRef(h)
		if err != nil {
			_ = p.Close()
			return nil, err
		}
		p.coefficients = append(p.coefficients, r)
	}
	return p, nil
}
func (p *NLPolynomial) open() error {
	if p == nil || p.manager == nil || p.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (p *NLPolynomial) Len() int {
	if p == nil || p.closed {
		return 0
	}
	return len(p.coefficients)
}
func (p *NLPolynomial) Degree() (uint32, error) {
	if err := p.open(); err != nil {
		return 0, err
	}
	if len(p.coefficients) == 0 {
		return 0, ast.ErrASTDomain
	}
	return uint32(len(p.coefficients) - 1), nil
}
func (p *NLPolynomial) Coefficient(i uint32) (ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if i >= uint32(len(p.coefficients)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return p.manager.RefGet(p.coefficients[i])
}
func (p *NLPolynomial) setCoefficient(i uint32, h ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if i >= uint32(len(p.coefficients)) {
		return ast.ErrASTDomain
	}
	r, err := p.manager.NewRef(h)
	if err != nil {
		return err
	}
	old := p.coefficients[i]
	if err = p.manager.DelRef(&old); err != nil {
		_ = p.manager.DelRef(&r)
		return err
	}
	p.coefficients[i] = r
	return nil
}
func (p *NLPolynomial) handles() ([]ast.ASTHandle, error) {
	r := make([]ast.ASTHandle, len(p.coefficients))
	for i := range r {
		h, err := p.manager.RefGet(p.coefficients[i])
		if err != nil {
			return nil, err
		}
		r[i] = h
	}
	return r, nil
}
func (p *NLPolynomial) Clone() (*NLPolynomial, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	h, err := p.handles()
	if err != nil {
		return nil, err
	}
	return newNLPolynomial(p.manager, h)
}
func (p *NLPolynomial) Close() (result error) {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	for i := len(p.coefficients) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.coefficients[i]))
	}
	if result == nil {
		p.closed = true
		p.coefficients = nil
	}
	return result
}
func (p *NLPolynomial) replace(handles []ast.ASTHandle) (result error) {
	for i := len(p.coefficients) - 1; i >= 0; i-- {
		result = errors.Join(result, p.manager.DelRef(&p.coefficients[i]))
	}
	if result != nil {
		return result
	}
	p.coefficients = nil
	for _, h := range handles {
		r, err := p.manager.NewRef(h)
		if err != nil {
			return err
		}
		p.coefficients = append(p.coefficients, r)
	}
	return nil
}
