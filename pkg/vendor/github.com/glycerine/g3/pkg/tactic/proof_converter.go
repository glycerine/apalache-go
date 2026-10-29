package tactic

// Source: src/tactic/proof_converter.h; src/tactic/proof_converter.cpp;
// src/tactic/converter.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type proofConverterBase struct {
	self       *proofConverterBase
	owner      ProofConverter
	budget     *arena.Budget
	ownerBytes uint64
	references uint32
	closed     bool
}

func (b *proofConverterBase) init(owner ProofConverter, budget *arena.Budget, bytes uint64) error {
	if owner == nil || budget == nil {
		return ast.ErrASTHandle
	}
	if err := budget.Reserve(bytes); err != nil {
		return err
	}
	b.self, b.owner, b.budget, b.ownerBytes = &*b, owner, budget, bytes
	return nil
}
func (b *proofConverterBase) open() error {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return arena.ErrClosed
	}
	if b.owner == nil {
		return ast.ErrASTHandle
	}
	return nil
}
func (b *proofConverterBase) IncRef() error {
	if err := b.open(); err != nil {
		return err
	}
	b.references++
	return nil
}
func (b *proofConverterBase) DecRef() error {
	if err := b.open(); err != nil {
		return err
	}
	if b.references == 0 {
		return arena.ErrBounds
	}
	b.references--
	if b.references == 0 {
		return b.owner.Close()
	}
	return nil
}
func (b *proofConverterBase) Cancel() error { return b.open() }
func (b *proofConverterBase) release() {
	b.budget.Release(b.ownerBytes)
	b.ownerBytes = 0
	b.owner = nil
	b.closed = true
}

type storedProofConverter struct {
	proofConverterBase
	manager *ast.ASTManager
	proof   ast.ASTRef
}

func NewStoredProofConverter(m *ast.ASTManager, proof ast.ASTHandle) (ProofConverter, error) {
	if proof.IsNull() {
		return nil, nil
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	ref, err := m.NewRef(proof)
	if err != nil {
		return nil, err
	}
	c := &storedProofConverter{manager: m, proof: ref}
	if err = c.init(c, m.Budget(), uint64(reflect.TypeOf(*c).Size())); err != nil {
		_ = m.DelRef(&ref)
		return nil, err
	}
	return c, nil
}
func (c *storedProofConverter) Convert(m *ast.ASTManager, sources []ast.ASTHandle, out ast.ASTRef) error {
	if err := c.open(); err != nil {
		return err
	}
	if m != c.manager || len(sources) != 0 {
		return ast.ErrASTDomain
	}
	v, err := m.RefGet(c.proof)
	if err != nil {
		return err
	}
	return m.SetRef(out, v)
}
func (c *storedProofConverter) Translate(tr *ast.ASTTranslation) (ProofConverter, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if tr == nil || tr.From() != c.manager {
		return nil, ast.ErrASTHandle
	}
	v, err := c.manager.RefGet(c.proof)
	if err != nil {
		return nil, err
	}
	v, err = tr.Translate(v)
	if err != nil {
		return nil, err
	}
	return NewStoredProofConverter(tr.To(), v)
}
func (c *storedProofConverter) Display(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return ast.ErrASTHandle
	}
	if _, err := io.WriteString(w, "(proof->proof-converter-wrapper\n"); err != nil {
		return err
	}
	p, err := ast.NewSimpleASTPrinterContext(c.manager, w, nil)
	if err != nil {
		return err
	}
	v, err := c.manager.RefGet(c.proof)
	if err != nil {
		return err
	}
	if err = p.DisplayExpr(w, v, 0); err != nil {
		return err
	}
	_, err = io.WriteString(w, ")\n")
	return err
}
func (c *storedProofConverter) Close() error {
	if c == nil || c.self != &c.proofConverterBase {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if err := c.manager.DelRef(&c.proof); err != nil {
		return err
	}
	c.release()
	return nil
}

type concatProofConverter struct {
	proofConverterBase
	first, second ProofConverter
}

func ConcatProofConverters(b *arena.Budget, first, second ProofConverter) (ProofConverter, error) {
	if first == nil {
		return second, nil
	}
	if second == nil {
		return first, nil
	}
	c := &concatProofConverter{}
	if err := c.init(c, b, uint64(reflect.TypeOf(*c).Size())); err != nil {
		return nil, err
	}
	if err := first.IncRef(); err != nil {
		_ = c.Close()
		return nil, err
	}
	c.first = first
	if err := second.IncRef(); err != nil {
		_ = c.Close()
		return nil, err
	}
	c.second = second
	return c, nil
}
func (c *concatProofConverter) Convert(m *ast.ASTManager, sources []ast.ASTHandle, out ast.ASTRef) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	tmp, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	if err = c.second.Convert(m, sources, tmp); err != nil {
		return err
	}
	v, err := m.RefGet(tmp)
	if err != nil {
		return err
	}
	return c.first.Convert(m, []ast.ASTHandle{v}, out)
}
func (c *concatProofConverter) Translate(tr *ast.ASTTranslation) (ProofConverter, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	a, err := c.first.Translate(tr)
	if err != nil {
		return nil, err
	}
	b, err := c.second.Translate(tr)
	if err != nil {
		_ = a.Close()
		return nil, err
	}
	r, err := ConcatProofConverters(tr.To().Budget(), a, b)
	if err != nil {
		if a != nil {
			_ = a.Close()
		}
		if b != nil {
			_ = b.Close()
		}
	}
	return r, err
}
func (c *concatProofConverter) Cancel() error {
	if err := c.open(); err != nil {
		return err
	}
	return errors.Join(c.second.Cancel(), c.first.Cancel())
}
func (c *concatProofConverter) Display(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "(concat-proof-converter\n"); err != nil {
		return err
	}
	if err := c.first.Display(w); err != nil {
		return err
	}
	if err := c.second.Display(w); err != nil {
		return err
	}
	_, err := io.WriteString(w, ")\n")
	return err
}
func (c *concatProofConverter) Close() error {
	if c == nil || c.self != &c.proofConverterBase {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if c.second != nil {
		if err := c.second.DecRef(); err != nil {
			return err
		}
		c.second = nil
	}
	if c.first != nil {
		if err := c.first.DecRef(); err != nil {
			return err
		}
		c.first = nil
	}
	c.release()
	return nil
}

type concatStarProofConverter struct {
	proofConverterBase
	first   ProofConverter
	seconds []ProofConverter
	sizes   []uint32
}

func ConcatStarProofConverters(b *arena.Budget, first ProofConverter, seconds []ProofConverter, sizes []uint32) (ProofConverter, error) {
	if len(seconds) == 0 || len(seconds) != len(sizes) {
		return nil, ast.ErrASTDomain
	}
	if len(seconds) == 1 {
		return ConcatProofConverters(b, first, seconds[0])
	}
	found := false
	for _, c := range seconds {
		if c != nil {
			found = true
			break
		}
	}
	if !found {
		return first, nil
	}
	c := &concatStarProofConverter{sizes: append([]uint32(nil), sizes...), seconds: make([]ProofConverter, len(seconds))}
	if err := c.init(c, b, uint64(reflect.TypeOf(*c).Size())); err != nil {
		return nil, err
	}
	if first != nil {
		if err := first.IncRef(); err != nil {
			_ = c.Close()
			return nil, err
		}
		c.first = first
	}
	for i, v := range seconds {
		if v != nil {
			if err := v.IncRef(); err != nil {
				_ = c.Close()
				return nil, err
			}
		}
		c.seconds[i] = v
	}
	return c, nil
}
func (c *concatStarProofConverter) Convert(m *ast.ASTManager, sources []ast.ASTHandle, out ast.ASTRef) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	sum := uint64(0)
	for _, n := range c.sizes {
		sum += uint64(n)
	}
	if sum != uint64(len(sources)) {
		return ast.ErrASTDomain
	}
	tmp := make([]ast.ASTRef, 0, len(c.seconds))
	proofs := make([]ast.ASTHandle, 0, len(c.seconds))
	defer func() {
		for i := range tmp {
			err = errors.Join(err, m.DelRef(&tmp[i]))
		}
	}()
	offset := 0
	for i, v := range c.seconds {
		n := int(c.sizes[i])
		ref, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		tmp = append(tmp, ref)
		if v != nil {
			if e = v.Convert(m, sources[offset:offset+n], ref); e != nil {
				return e
			}
		} else {
			if n != 1 {
				return ast.ErrASTDomain
			}
			if e = m.SetRef(ref, sources[offset]); e != nil {
				return e
			}
		}
		p, e := m.RefGet(ref)
		if e != nil {
			return e
		}
		proofs = append(proofs, p)
		offset += n
	}
	if c.first != nil {
		return c.first.Convert(m, proofs, out)
	}
	if len(proofs) != 1 {
		return ast.ErrASTDomain
	}
	return m.SetRef(out, proofs[0])
}
func (c *concatStarProofConverter) Translate(tr *ast.ASTTranslation) (ProofConverter, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	var first ProofConverter
	var err error
	if c.first != nil {
		first, err = c.first.Translate(tr)
		if err != nil {
			return nil, err
		}
	}
	seconds := make([]ProofConverter, len(c.seconds))
	for i, v := range c.seconds {
		if v != nil {
			seconds[i], err = v.Translate(tr)
			if err != nil {
				if first != nil {
					_ = first.Close()
				}
				for _, x := range seconds {
					if x != nil {
						_ = x.Close()
					}
				}
				return nil, err
			}
		}
	}
	r, err := ConcatStarProofConverters(tr.To().Budget(), first, seconds, c.sizes)
	if err != nil {
		if first != nil {
			_ = first.Close()
		}
		for _, x := range seconds {
			if x != nil {
				_ = x.Close()
			}
		}
	}
	return r, err
}
func (c *concatStarProofConverter) Cancel() error {
	if err := c.open(); err != nil {
		return err
	}
	var result error
	if c.first != nil {
		result = errors.Join(result, c.first.Cancel())
	}
	for _, v := range c.seconds {
		if v != nil {
			result = errors.Join(result, v.Cancel())
		}
	}
	return result
}
func (c *concatStarProofConverter) Display(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "(concat-star-proof-converter\n"); err != nil {
		return err
	}
	if c.first != nil {
		if err := c.first.Display(w); err != nil {
			return err
		}
	}
	for _, v := range c.seconds {
		if v != nil {
			if err := v.Display(w); err != nil {
				return err
			}
		}
	}
	_, err := io.WriteString(w, ")\n")
	return err
}
func (c *concatStarProofConverter) Close() error {
	if c == nil || c.self != &c.proofConverterBase {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	for i := 0; i < len(c.seconds); i++ {
		if c.seconds[i] != nil {
			if err := c.seconds[i].DecRef(); err != nil {
				return err
			}
			c.seconds[i] = nil
		}
	}
	if c.first != nil {
		if err := c.first.DecRef(); err != nil {
			return err
		}
		c.first = nil
	}
	c.seconds = nil
	c.sizes = nil
	c.release()
	return nil
}

func ApplyProofConverter(m *ast.ASTManager, c ProofConverter, proof ast.ASTRef) error {
	if c == nil {
		return nil
	}
	v, err := m.RefGet(proof)
	if err != nil {
		return err
	}
	return c.Convert(m, []ast.ASTHandle{v}, proof)
}
func ApplyProofConverters(m *ast.ASTManager, first ProofConverter, seconds []ProofConverter, out ast.ASTRef) (err error) {
	if first == nil || len(seconds) == 0 {
		return ast.ErrASTDomain
	}
	refs := make([]ast.ASTRef, 0, len(seconds))
	values := make([]ast.ASTHandle, 0, len(seconds))
	defer func() {
		for i := range refs {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
	}()
	for _, c := range seconds {
		if c == nil {
			return ast.ErrASTDomain
		}
		r, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		refs = append(refs, r)
		if e = c.Convert(m, nil, r); e != nil {
			return e
		}
		v, e := m.RefGet(r)
		if e != nil {
			return e
		}
		values = append(values, v)
	}
	return first.Convert(m, values, out)
}
