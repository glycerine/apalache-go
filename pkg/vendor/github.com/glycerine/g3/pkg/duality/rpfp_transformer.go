package duality

// Source: src/duality/duality.h, RPFP::Transformer construction, copying,
// full/empty tests, complement and ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Transformer owns the relation parameters, individual parameters, and
// formula of a symbolic Duality relation. Its context is borrowed.
type Transformer struct {
	context          *Context
	relationParams   []ast.ASTRef
	individualParams []ast.ASTRef
	formula          ast.ASTRef
	closed           bool
}

func NewTransformer(context *Context, relations []*FuncDecl, individuals []*Expr, formula *Expr) (_ *Transformer, result error) {
	if context == nil {
		return nil, ErrWrapperOwner
	}
	if err := context.open(); err != nil {
		return nil, err
	}
	t := &Transformer{context: context}
	defer func() {
		if result != nil {
			result = errors.Join(result, t.Close())
		}
	}()
	for _, d := range relations {
		if err := d.open(); err != nil {
			return nil, err
		}
		if d.value.object.context != context {
			return nil, ErrContextMismatch
		}
		ref, err := context.manager.CopyRef(d.value.ref)
		if err != nil {
			return nil, err
		}
		t.relationParams = append(t.relationParams, ref)
	}
	for _, e := range individuals {
		if err := e.open(); err != nil {
			return nil, err
		}
		if e.value.object.context != context {
			return nil, ErrContextMismatch
		}
		ref, err := context.manager.CopyRef(e.value.ref)
		if err != nil {
			return nil, err
		}
		t.individualParams = append(t.individualParams, ref)
	}
	if err := formula.open(); err != nil {
		return nil, err
	}
	if formula.value.object.context != context {
		return nil, ErrContextMismatch
	}
	var err error
	t.formula, err = context.manager.CopyRef(formula.value.ref)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Transformer) open() error {
	if t == nil || t.closed || t.context == nil || t.formula == (ast.ASTRef{}) {
		return ErrWrapperOwner
	}
	return t.context.open()
}

// Clone copies the source transformer and its owned references.
func (t *Transformer) Clone() (_ *Transformer, result error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	clone := &Transformer{context: t.context}
	defer func() {
		if result != nil {
			result = errors.Join(result, clone.Close())
		}
	}()
	for _, ref := range t.relationParams {
		copied, err := t.context.manager.CopyRef(ref)
		if err != nil {
			return nil, err
		}
		clone.relationParams = append(clone.relationParams, copied)
	}
	for _, ref := range t.individualParams {
		copied, err := t.context.manager.CopyRef(ref)
		if err != nil {
			return nil, err
		}
		clone.individualParams = append(clone.individualParams, copied)
	}
	var err error
	clone.formula, err = t.context.manager.CopyRef(t.formula)
	if err != nil {
		return nil, err
	}
	return clone, nil
}

// Formula returns an owning expression copy, as the source wrapper does.
func (t *Transformer) Formula() (*Expr, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	h, err := t.context.manager.RefGet(t.formula)
	if err != nil {
		return nil, err
	}
	return NewExpr(t.context, h)
}

func (t *Transformer) setBoolean(value bool) error {
	if err := t.open(); err != nil {
		return err
	}
	h, err := t.context.manager.MkBoolVal(value)
	if err != nil {
		return err
	}
	return t.context.manager.SetRef(t.formula, h)
}
func (t *Transformer) SetEmpty() error { return t.setBoolean(false) }
func (t *Transformer) SetFull() error  { return t.setBoolean(true) }
func (t *Transformer) IsEmpty() (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	h, err := t.context.manager.RefGet(t.formula)
	if err != nil {
		return false, err
	}
	return t.context.manager.IsFalse(h), nil
}
func (t *Transformer) IsFull() (bool, error) {
	if err := t.open(); err != nil {
		return false, err
	}
	h, err := t.context.manager.RefGet(t.formula)
	if err != nil {
		return false, err
	}
	return t.context.manager.IsTrue(h), nil
}
func (t *Transformer) Complement() error {
	if err := t.open(); err != nil {
		return err
	}
	h, err := t.context.manager.RefGet(t.formula)
	if err != nil {
		return err
	}
	negated, err := t.context.manager.MkNot(h)
	if err != nil {
		return err
	}
	return t.context.manager.SetRef(t.formula, negated)
}

// Close may be retried after a reference-release error.
func (t *Transformer) Close() error {
	if t == nil || t.closed {
		return nil
	}
	m := t.context.manager
	if t.formula != (ast.ASTRef{}) {
		if err := m.DelRef(&t.formula); err != nil {
			return err
		}
	}
	for len(t.individualParams) > 0 {
		i := len(t.individualParams) - 1
		err := m.DelRef(&t.individualParams[i])
		if t.individualParams[i] == (ast.ASTRef{}) {
			t.individualParams = t.individualParams[:i]
		}
		if err != nil {
			return err
		}
	}
	for len(t.relationParams) > 0 {
		i := len(t.relationParams) - 1
		err := m.DelRef(&t.relationParams[i])
		if t.relationParams[i] == (ast.ASTRef{}) {
			t.relationParams = t.relationParams[:i]
		}
		if err != nil {
			return err
		}
	}
	t.closed = true
	return nil
}
