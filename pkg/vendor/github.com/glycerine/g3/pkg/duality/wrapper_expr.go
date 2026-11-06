package duality

// Source: src/duality/duality_wrapper.h (expr and Boolean operators).
// Original author: Ken McMillan.

import "github.com/glycerine/g3/pkg/ast"

// Expr is a typed owning expression wrapper.
type Expr struct{ value *AST }

func NewExpr(context *Context, handle ast.ASTHandle) (*Expr, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	kind, err := context.manager.GetKind(handle)
	if err != nil {
		return nil, err
	}
	if kind > ast.ASTQuantifier {
		return nil, ast.ErrASTDomain
	}
	value, err := newAST(context, handle)
	if err != nil {
		return nil, err
	}
	return &Expr{value: value}, nil
}

func (e *Expr) open() error {
	if e == nil || e.value == nil {
		return ErrWrapperOwner
	}
	return e.value.open()
}

func (e *Expr) Raw() (ast.ASTHandle, error) {
	if err := e.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return e.value.Raw()
}

func (e *Expr) Close() error {
	if err := e.open(); err != nil {
		return err
	}
	return e.value.Close()
}

// Closed reports whether Close consumed this expression's owning reference,
// including when the underlying release also returned an error.
func (e *Expr) Closed() bool {
	return e != nil && e.value != nil && e.value.closed
}

func (e *Expr) Clone() (*Expr, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	value, err := e.value.Clone()
	if err != nil {
		return nil, err
	}
	return &Expr{value: value}, nil
}

func (e *Expr) Context() (*Context, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	return e.value.object.context, nil
}

func (e *Expr) Sort() (*Sort, error) {
	handle, err := e.Raw()
	if err != nil {
		return nil, err
	}
	context := e.value.object.context
	sort, err := context.manager.GetSort(handle)
	if err != nil {
		return nil, err
	}
	return NewSort(context, sort)
}

func (e *Expr) IsBool() (bool, error) {
	sort, err := e.Sort()
	if err != nil {
		return false, err
	}
	defer sort.Close()
	return sort.IsBool()
}

func (e *Expr) IsApp() (bool, error) {
	handle, err := e.Raw()
	if err != nil {
		return false, err
	}
	kind, err := e.value.object.context.manager.GetKind(handle)
	return kind == ast.ASTApp, err
}

func (e *Expr) IsQuantifier() (bool, error) {
	handle, err := e.Raw()
	if err != nil {
		return false, err
	}
	kind, err := e.value.object.context.manager.GetKind(handle)
	return kind == ast.ASTQuantifier, err
}

func (e *Expr) IsVar() (bool, error) {
	handle, err := e.Raw()
	if err != nil {
		return false, err
	}
	kind, err := e.value.object.context.manager.GetKind(handle)
	return kind == ast.ASTVar, err
}

func (e *Expr) IsGround() (bool, error) {
	handle, err := e.Raw()
	if err != nil {
		return false, err
	}
	return e.value.object.context.manager.IsGround(handle)
}

func (e *Expr) HasQuantifiers() (bool, error) {
	handle, err := e.Raw()
	if err != nil {
		return false, err
	}
	return e.value.object.context.manager.HasQuantifiers(handle)
}

func (e *Expr) Decl() (*FuncDecl, error) {
	handle, err := e.Raw()
	if err != nil {
		return nil, err
	}
	context := e.value.object.context
	decl, err := context.manager.GetDecl(handle)
	if err != nil {
		return nil, err
	}
	return NewFuncDecl(context, decl)
}

func (e *Expr) NumArgs() (uint32, error) {
	handle, err := e.Raw()
	if err != nil {
		return 0, err
	}
	manager := e.value.object.context.manager
	kind, err := manager.GetKind(handle)
	if err != nil {
		return 0, err
	}
	switch kind {
	case ast.ASTApp:
		return manager.GetNumArgs(handle)
	case ast.ASTQuantifier:
		return 1, nil
	case ast.ASTVar:
		return 0, nil
	default:
		return 0, ast.ErrASTDomain
	}
}

func (e *Expr) Arg(index uint32) (*Expr, error) {
	handle, err := e.Raw()
	if err != nil {
		return nil, err
	}
	context := e.value.object.context
	kind, err := context.manager.GetKind(handle)
	if err != nil {
		return nil, err
	}
	var child ast.ASTHandle
	switch kind {
	case ast.ASTApp:
		child, err = context.manager.GetArg(handle, index)
	case ast.ASTQuantifier:
		if index != 0 {
			return nil, ast.ErrASTDomain
		}
		child, err = context.manager.GetQuantifierBody(handle)
	default:
		return nil, ast.ErrASTDomain
	}
	if err != nil {
		return nil, err
	}
	return NewExpr(context, child)
}

func (d *FuncDecl) Call(args ...*Expr) (*Expr, error) {
	if err := d.open(); err != nil {
		return nil, err
	}
	context := d.value.object.context
	handles := make([]ast.ASTHandle, len(args))
	for i, arg := range args {
		argContext, err := arg.Context()
		if err != nil {
			return nil, err
		}
		if argContext != context {
			return nil, ErrContextMismatch
		}
		handles[i], err = arg.Raw()
		if err != nil {
			return nil, err
		}
	}
	decl, err := d.Raw()
	if err != nil {
		return nil, err
	}
	handle, err := context.manager.MkApp(decl, handles)
	if err != nil {
		return nil, err
	}
	return NewExpr(context, handle)
}

func (c *Context) Constant(name *Symbol, sort *Sort) (*Expr, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if err := name.open(); err != nil {
		return nil, err
	}
	if name.object.context != c {
		return nil, ErrContextMismatch
	}
	sortContext, err := sort.Context()
	if err != nil {
		return nil, err
	}
	if sortContext != c {
		return nil, ErrContextMismatch
	}
	handle, err := sort.Raw()
	if err != nil {
		return nil, err
	}
	constant, err := c.manager.MkConst(name.value, handle)
	if err != nil {
		return nil, err
	}
	return NewExpr(c, constant)
}

func (c *Context) BoolConst(name string) (*Expr, error) {
	symbol, err := c.StringSymbol(name)
	if err != nil {
		return nil, err
	}
	sort, err := c.BoolSort()
	if err != nil {
		return nil, err
	}
	defer sort.Close()
	return c.Constant(symbol, sort)
}

func (c *Context) BoolVal(value bool) (*Expr, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	kind := ast.ASTOpFalse
	if value {
		kind = ast.ASTOpTrue
	}
	handle, err := c.manager.MkFamilyApp(c.families.Basic, kind, nil, nil)
	if err != nil {
		return nil, err
	}
	return NewExpr(c, handle)
}

func exprPair(left, right *Expr) (*Context, ast.ASTHandle, ast.ASTHandle, error) {
	leftContext, err := left.Context()
	if err != nil {
		return nil, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	rightContext, err := right.Context()
	if err != nil {
		return nil, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if leftContext != rightContext {
		return nil, ast.ASTHandle{}, ast.ASTHandle{}, ErrContextMismatch
	}
	a, err := left.Raw()
	if err != nil {
		return nil, ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	b, err := right.Raw()
	return leftContext, a, b, err
}

func (e *Expr) Not() (*Expr, error) {
	handle, err := e.Raw()
	if err != nil {
		return nil, err
	}
	context := e.value.object.context
	result, err := context.manager.MkNot(handle)
	if err != nil {
		return nil, err
	}
	return NewExpr(context, result)
}

func (e *Expr) And(other *Expr) (*Expr, error) {
	context, left, right, err := exprPair(e, other)
	if err != nil {
		return nil, err
	}
	result, err := context.manager.MkAnd(left, right)
	if err != nil {
		return nil, err
	}
	return NewExpr(context, result)
}

func (e *Expr) Or(other *Expr) (*Expr, error) {
	context, left, right, err := exprPair(e, other)
	if err != nil {
		return nil, err
	}
	result, err := context.manager.MkOr(left, right)
	if err != nil {
		return nil, err
	}
	return NewExpr(context, result)
}

func (e *Expr) Implies(other *Expr) (*Expr, error) {
	context, left, right, err := exprPair(e, other)
	if err != nil {
		return nil, err
	}
	result, err := context.manager.MkImplies(left, right)
	if err != nil {
		return nil, err
	}
	return NewExpr(context, result)
}

func (e *Expr) Eq(other *Expr) (*Expr, error) {
	context, left, right, err := exprPair(e, other)
	if err != nil {
		return nil, err
	}
	result, err := context.manager.MkEq(left, right)
	if err != nil {
		return nil, err
	}
	return NewExpr(context, result)
}

func (e *Expr) Distinct(other *Expr) (*Expr, error) {
	context, left, right, err := exprPair(e, other)
	if err != nil {
		return nil, err
	}
	result, err := context.manager.MkDistinct([]ast.ASTHandle{left, right})
	if err != nil {
		return nil, err
	}
	return NewExpr(context, result)
}
