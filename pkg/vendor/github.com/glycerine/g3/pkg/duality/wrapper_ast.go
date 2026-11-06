package duality

// Source: src/duality/duality_wrapper.h (ast_i, ast, sort, func_decl, expr).
// Original author: Ken McMillan.

import "github.com/glycerine/g3/pkg/ast"

// AST is the reference-counted Duality AST base. The concrete ASTRef retains
// source logical reference counting and interrupted deletion ownership.
type AST struct {
	object Object
	ref    ast.ASTRef
	closed bool
}

func newAST(context *Context, handle ast.ASTHandle) (*AST, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	ref, err := context.manager.NewRef(handle)
	if err != nil {
		return nil, err
	}
	value := &AST{ref: ref}
	value.object = Object{context: context}
	value.object.self = &value.object
	return value, nil
}

// NewAST constructs the source ast wrapper and acquires its logical reference.
func NewAST(context *Context, handle ast.ASTHandle) (*AST, error) {
	return newAST(context, handle)
}

func (a *AST) open() error {
	if a == nil || a.closed {
		return ErrWrapperOwner
	}
	return a.object.open()
}

// Raw returns the borrowed AST handle held by this wrapper.
func (a *AST) Raw() (ast.ASTHandle, error) {
	if err := a.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return a.object.context.manager.RefGet(a.ref)
}

func (a *AST) Object() (*Object, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	return &a.object, nil
}

// Clone implements the source ast copy constructor.
func (a *AST) Clone() (*AST, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	ref, err := a.object.context.manager.CopyRef(a.ref)
	if err != nil {
		return nil, err
	}
	copy := &AST{ref: ref}
	copy.object = Object{context: a.object.context}
	copy.object.self = &copy.object
	return copy, nil
}

// Assign replaces the owned AST reference. Duality operations require the
// same context; cross-context translation is an explicit separate operation.
func (a *AST) Assign(other *AST) error {
	if err := a.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if a.object.context != other.object.context {
		return ErrContextMismatch
	}
	return a.object.context.manager.AssignRef(a.ref, other.ref)
}

func (a *AST) Null() (bool, error) {
	handle, err := a.Raw()
	return handle.IsNull(), err
}

func (a *AST) Equal(other *AST) (bool, error) {
	if err := a.open(); err != nil {
		return false, err
	}
	if err := other.open(); err != nil {
		return false, err
	}
	left, err := a.Raw()
	if err != nil {
		return false, err
	}
	right, err := other.Raw()
	return left == right, err
}

// Less replaces source pointer ordering with stable owner/slot/generation
// ordering. Pointer order is representational and cannot be preserved in Go.
func (a *AST) Less(other *AST) (bool, error) {
	left, err := a.Raw()
	if err != nil {
		return false, err
	}
	right, err := other.Raw()
	if err != nil {
		return false, err
	}
	if left.Origin() != right.Origin() {
		return left.Origin() < right.Origin(), nil
	}
	if left.Slot() != right.Slot() {
		return left.Slot() < right.Slot(), nil
	}
	return left.Generation() < right.Generation(), nil
}

func (a *AST) Close() error {
	if err := a.open(); err != nil {
		return err
	}
	err := a.object.context.manager.DelRef(&a.ref)
	if a.ref == (ast.ASTRef{}) {
		a.closed = true
	}
	return err
}

// Sort is a typed owning AST wrapper.
type Sort struct{ value *AST }

func NewSort(context *Context, handle ast.ASTHandle) (*Sort, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	kind, err := context.manager.GetKind(handle)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTSort {
		return nil, ast.ErrASTDomain
	}
	value, err := newAST(context, handle)
	if err != nil {
		return nil, err
	}
	return &Sort{value: value}, nil
}

func (s *Sort) open() error {
	if s == nil || s.value == nil {
		return ErrWrapperOwner
	}
	return s.value.open()
}

func (s *Sort) Raw() (ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.value.Raw()
}

func (s *Sort) Close() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.value.Close()
}

func (s *Sort) Clone() (*Sort, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	value, err := s.value.Clone()
	if err != nil {
		return nil, err
	}
	return &Sort{value: value}, nil
}

func (s *Sort) Context() (*Context, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return s.value.object.context, nil
}

func (c *Context) BoolSort() (*Sort, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	handle, err := c.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	return NewSort(c, handle)
}

func (s *Sort) Kind() (SortKind, error) {
	if err := s.open(); err != nil {
		return SortUnknown, err
	}
	handle, err := s.Raw()
	if err != nil {
		return SortUnknown, err
	}
	context := s.value.object.context
	family, err := context.manager.GetFamilyID(handle)
	if err != nil {
		return SortUnknown, err
	}
	kind, err := context.manager.GetDeclKind(handle)
	if err != nil {
		return SortUnknown, err
	}
	switch {
	case family == ast.NullFamilyID:
		return SortUninterpreted, nil
	case family == context.families.Basic && kind == 0:
		return SortBool, nil
	case family == context.families.Arithmetic && kind == 0:
		return SortInt, nil
	case family == context.families.Arithmetic && kind == 1:
		return SortReal, nil
	case family == context.families.Array && kind == 0:
		return SortArray, nil
	default:
		return SortUnknown, nil
	}
}

func (s *Sort) IsBool() (bool, error) {
	kind, err := s.Kind()
	return kind == SortBool, err
}

// FuncDecl is a typed owning declaration wrapper.
type FuncDecl struct{ value *AST }

func NewFuncDecl(context *Context, handle ast.ASTHandle) (*FuncDecl, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	kind, err := context.manager.GetKind(handle)
	if err != nil {
		return nil, err
	}
	if kind != ast.ASTFuncDecl {
		return nil, ast.ErrASTDomain
	}
	value, err := newAST(context, handle)
	if err != nil {
		return nil, err
	}
	return &FuncDecl{value: value}, nil
}

func (d *FuncDecl) open() error {
	if d == nil || d.value == nil {
		return ErrWrapperOwner
	}
	return d.value.open()
}

func (d *FuncDecl) Raw() (ast.ASTHandle, error) {
	if err := d.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return d.value.Raw()
}

func (d *FuncDecl) Close() error {
	if err := d.open(); err != nil {
		return err
	}
	return d.value.Close()
}

func (d *FuncDecl) Arity() (uint32, error) {
	handle, err := d.Raw()
	if err != nil {
		return 0, err
	}
	return d.value.object.context.manager.GetArity(handle)
}

func (d *FuncDecl) Domain(index uint32) (*Sort, error) {
	handle, err := d.Raw()
	if err != nil {
		return nil, err
	}
	context := d.value.object.context
	sort, err := context.manager.GetDomain(handle, index)
	if err != nil {
		return nil, err
	}
	return NewSort(context, sort)
}

func (d *FuncDecl) Range() (*Sort, error) {
	handle, err := d.Raw()
	if err != nil {
		return nil, err
	}
	context := d.value.object.context
	sort, err := context.manager.GetRange(handle)
	if err != nil {
		return nil, err
	}
	return NewSort(context, sort)
}

func (d *FuncDecl) Name() (*Symbol, error) {
	handle, err := d.Raw()
	if err != nil {
		return nil, err
	}
	context := d.value.object.context
	name, err := context.manager.GetASTName(handle)
	if err != nil {
		return nil, err
	}
	return newSymbol(context, name)
}

func (d *FuncDecl) IsConst() (bool, error) {
	arity, err := d.Arity()
	return arity == 0, err
}

func (c *Context) Function(name *Symbol, domain []*Sort, result *Sort) (*FuncDecl, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if err := name.open(); err != nil {
		return nil, err
	}
	if name.object.context != c {
		return nil, ErrContextMismatch
	}
	resultContext, err := result.Context()
	if err != nil {
		return nil, err
	}
	if resultContext != c {
		return nil, ErrContextMismatch
	}
	rangeHandle, err := result.Raw()
	if err != nil {
		return nil, err
	}
	handles := make([]ast.ASTHandle, len(domain))
	for i, item := range domain {
		itemContext, err := item.Context()
		if err != nil {
			return nil, err
		}
		if itemContext != c {
			return nil, ErrContextMismatch
		}
		handles[i], err = item.Raw()
		if err != nil {
			return nil, err
		}
	}
	handle, err := c.manager.MkFuncDecl(name.value, handles, rangeHandle, ast.ASTFuncDeclInfo{})
	if err != nil {
		return nil, err
	}
	return NewFuncDecl(c, handle)
}
