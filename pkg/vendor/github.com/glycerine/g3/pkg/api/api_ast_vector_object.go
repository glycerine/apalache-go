package api

// Source: src/api/api_ast_vector.cpp:27-130;
// src/api/api_ast_vector.h:25-37.

import (
	"bytes"
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// apiASTVectorObject is the concrete Z3_ast_vector_ref wrapper. Each non-null
// element owns one reference in the context's AST manager. Expanded slots are
// null ast_ref values, as in the source vector.
type apiASTVectorObject struct {
	object  apiObject
	values  []ast.ASTHandle
	pending []ast.ASTHandle
}

func newAPIASTVectorObject(c *apiContextResults) (*apiASTVectorObject, error) {
	if e := c.open(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(apiASTVectorObject{}).Size())
	if e := c.manager.Budget().Reserve(n); e != nil {
		return nil, e
	}
	v := &apiASTVectorObject{}
	v.object = apiObject{context: c, destructor: v, ownerBytes: n}
	v.object.self = &v.object
	id, e := c.addObject(&v.object)
	if e != nil {
		c.manager.Budget().Release(n)
		return nil, e
	}
	v.object.id = id
	return v, nil
}

func (v *apiASTVectorObject) check() error {
	if v == nil {
		return errAPIObjectOwner
	}
	return v.object.open()
}

func (v *apiASTVectorObject) retain(h ast.ASTHandle) error {
	if h.IsNull() {
		return nil
	}
	return v.object.context.manager.IncRef(h)
}

func (v *apiASTVectorObject) release(h ast.ASTHandle) error {
	if h.IsNull() {
		return nil
	}
	consumed, e := v.object.context.manager.DecRefConsumed(h)
	if !consumed {
		v.pending = append(v.pending, h)
	}
	return e
}

func (v *apiASTVectorObject) size() (uint32, error) {
	if e := v.check(); e != nil {
		return 0, e
	}
	return uint32(len(v.values)), nil
}

func (v *apiASTVectorObject) get(i uint32) (ast.ASTHandle, error) {
	if e := v.check(); e != nil {
		return ast.ASTHandle{}, e
	}
	if uint64(i) >= uint64(len(v.values)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return v.values[i], nil
}

func (v *apiASTVectorObject) set(i uint32, h ast.ASTHandle) error {
	if e := v.check(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(v.values)) {
		return arena.ErrBounds
	}
	// ast_ref::operator= retains the incoming AST before releasing the old one.
	if e := v.retain(h); e != nil {
		return e
	}
	old := v.values[i]
	if e := v.release(old); e != nil {
		if !h.IsNull() {
			v.pending = append(v.pending, h)
		}
		return e
	}
	v.values[i] = h
	return nil
}

func (v *apiASTVectorObject) push(h ast.ASTHandle) error {
	if e := v.check(); e != nil {
		return e
	}
	if e := v.retain(h); e != nil {
		return e
	}
	v.values = append(v.values, h)
	return nil
}

func (v *apiASTVectorObject) resize(n uint32) error {
	if e := v.check(); e != nil {
		return e
	}
	if uint64(n) > uint64(int(^uint(0)>>1)) {
		return arena.ErrBounds
	}
	if int(n) >= len(v.values) {
		v.values = append(v.values, make([]ast.ASTHandle, int(n)-len(v.values))...)
		return nil
	}
	// vector::resize destroys removed ast_ref elements from the end.
	for i := len(v.values); i > int(n); i-- {
		h := v.values[i-1]
		if e := v.release(h); e != nil {
			return e
		}
		v.values[i-1] = ast.ASTHandle{}
	}
	v.values = v.values[:n]
	return nil
}

func (v *apiASTVectorObject) translate(target *apiContextResults) (result *apiASTVectorObject, err error) {
	if e := v.check(); e != nil {
		return nil, e
	}
	if e := target.open(); e != nil {
		return nil, e
	}
	if v.object.context == target {
		return nil, ast.ErrASTDomain
	}
	translator, e := ast.NewASTTranslation(v.object.context.manager, target.manager, true)
	if e != nil {
		return nil, e
	}
	defer func() { err = errors.Join(err, translator.Close()) }()
	result, e = newAPIASTVectorObject(target)
	if e != nil {
		return nil, e
	}
	if e = target.saveObject(&result.object); e != nil {
		return result, e
	}
	for _, h := range v.values {
		translated, x := translator.Translate(h)
		if x != nil {
			return result, x
		}
		if x = result.push(translated); x != nil {
			return result, x
		}
	}
	return result, nil
}

func (v *apiASTVectorObject) string() (string, error) {
	if e := v.check(); e != nil {
		return "", e
	}
	printer, e := ast.NewSimpleASTPrinterContext(v.object.context.manager, nil, nil)
	if e != nil {
		return "", e
	}
	var out bytes.Buffer
	out.WriteString("(ast-vector")
	for _, h := range v.values {
		out.WriteString("\n  ")
		if x := writeAPIAST(printer, &out, h, 0); x != nil {
			return "", x
		}
	}
	out.WriteByte(')')
	return out.String(), nil
}

func writeAPIAST(printer *ast.SimpleASTPrinterContext, out io.Writer, h ast.ASTHandle, indent uint32) error {
	if h.IsNull() {
		_, e := io.WriteString(out, "null")
		return e
	}
	kind, e := printer.Manager().GetKind(h)
	if e != nil {
		return e
	}
	switch kind {
	case ast.ASTSort:
		return printer.DisplaySort(out, h, indent)
	case ast.ASTFuncDecl:
		return printer.DisplayFuncDecl(out, h, indent)
	default:
		return printer.DisplayExpr(out, h, indent)
	}
}

func (v *apiASTVectorObject) apiDestroy() error {
	// The base object has already been removed from the context registry when
	// its virtual destructor runs, so teardown cannot use the public check.
	for i := len(v.values); i != 0; i-- {
		h := v.values[i-1]
		if e := v.release(h); e != nil {
			return e
		}
		v.values[i-1] = ast.ASTHandle{}
	}
	v.values = v.values[:0]
	for len(v.pending) != 0 {
		last := len(v.pending) - 1
		h := v.pending[last]
		consumed, e := v.object.context.manager.DecRefConsumed(h)
		if consumed {
			v.pending = v.pending[:last]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return errAPIObjectOwner
		}
	}
	v.values = nil
	v.pending = nil
	return nil
}
