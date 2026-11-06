package substitution

// Source: src/ast/substitution/expr_offset.h;
// src/ast/substitution/expr_offset_map.h;
// src/ast/substitution/var_offset_map.h.

import (
	"math"

	"github.com/glycerine/g3/pkg/ast"
	uhash "github.com/glycerine/g3/pkg/util/hash"
)

// ExprOffset pairs an expression with a variable namespace offset. It avoids
// constructing renamed term variants while matching and unifying expressions.
type ExprOffset struct {
	expr   ast.ASTHandle
	offset uint32
}

func NewExprOffset(expr ast.ASTHandle, offset uint32) ExprOffset {
	return ExprOffset{expr: expr, offset: offset}
}

func (e ExprOffset) Expr() ast.ASTHandle { return e.expr }
func (e ExprOffset) Offset() uint32      { return e.offset }

// Hash reproduces expr_offset::hash. A null expression has no source hash.
func (e ExprOffset) Hash(m *ast.ASTManager) (uint32, error) {
	if m == nil || e.expr.IsNull() {
		return 0, ast.ErrASTHandle
	}
	id, err := m.GetASTID(e.expr)
	if err != nil {
		return 0, err
	}
	return uhash.Mix(id, e.offset, 17), nil
}

type varOffset struct {
	variable uint32
	offset   uint32
}

// The source templates are instantiated by retained consumers with
// expr_offset, expr*, unsigned, and color. These concrete maps cover those
// instantiations without introducing a universal template adapter.
type varOffsetExprMap struct {
	values             map[varOffset]ExprOffset
	offsets, variables uint32
}

func (m *varOffsetExprMap) reserve(offsets, variables uint32) error {
	if offsets != 0 && variables > math.MaxUint32/offsets {
		return ast.ErrASTDomain
	}
	if offsets > m.offsets || variables > m.variables {
		m.offsets, m.variables = offsets, variables
	}
	if m.values == nil {
		m.values = make(map[varOffset]ExprOffset)
	} else {
		clear(m.values)
	}
	return nil
}

func (m *varOffsetExprMap) reset() {
	clear(m.values)
}

func (m *varOffsetExprMap) insert(variable, offset uint32, value ExprOffset) error {
	if variable >= m.variables || offset >= m.offsets {
		return ast.ErrASTDomain
	}
	m.values[varOffset{variable, offset}] = value
	return nil
}

func (m *varOffsetExprMap) find(variable, offset uint32) (ExprOffset, bool, error) {
	if variable >= m.variables || offset >= m.offsets {
		return ExprOffset{}, false, ast.ErrASTDomain
	}
	v, ok := m.values[varOffset{variable, offset}]
	return v, ok, nil
}

func (m *varOffsetExprMap) erase(variable, offset uint32) error {
	if variable >= m.variables || offset >= m.offsets {
		return ast.ErrASTDomain
	}
	delete(m.values, varOffset{variable, offset})
	return nil
}

type exprOffsetExprMap map[ExprOffset]ast.ASTHandle
type exprOffsetOffsetMap map[ExprOffset]ExprOffset
type exprOffsetUintMap map[ExprOffset]uint32

func (m exprOffsetExprMap) contains(key ExprOffset) bool               { _, ok := m[key]; return ok }
func (m exprOffsetExprMap) find(key ExprOffset) (ast.ASTHandle, bool)  { v, ok := m[key]; return v, ok }
func (m exprOffsetExprMap) insert(key ExprOffset, value ast.ASTHandle) { m[key] = value }
func (m exprOffsetExprMap) reset()                                     { clear(m) }

func (m exprOffsetOffsetMap) contains(key ExprOffset) bool           { _, ok := m[key]; return ok }
func (m exprOffsetOffsetMap) find(key ExprOffset) (ExprOffset, bool) { v, ok := m[key]; return v, ok }
func (m exprOffsetOffsetMap) insert(key, value ExprOffset)           { m[key] = value }
func (m exprOffsetOffsetMap) reset()                                 { clear(m) }

func (m exprOffsetUintMap) contains(key ExprOffset) bool        { _, ok := m[key]; return ok }
func (m exprOffsetUintMap) find(key ExprOffset) (uint32, bool)  { v, ok := m[key]; return v, ok }
func (m exprOffsetUintMap) insert(key ExprOffset, value uint32) { m[key] = value }
func (m exprOffsetUintMap) reset()                              { clear(m) }
