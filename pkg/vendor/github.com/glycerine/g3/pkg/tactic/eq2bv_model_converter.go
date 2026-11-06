package tactic

// Source: src/tactic/arith/eq2bv_tactic.cpp, eq2bv_tactic::bvmc.

import (
	"errors"
	"io"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type eq2bvModelPair struct {
	fresh, original ast.ASTHandle
	negativeWidth   uint32
}
type Eq2BVModelConverter struct {
	tacticConverterBase
	manager *ast.ASTManager
	pairs   []eq2bvModelPair
}

func NewEq2BVModelConverter(m *ast.ASTManager) (*Eq2BVModelConverter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	c := &Eq2BVModelConverter{manager: m}
	if err := c.init(c, m.Budget(), uint64(reflect.TypeOf(*c).Size())); err != nil {
		return nil, err
	}
	return c, nil
}
func (c *Eq2BVModelConverter) Insert(fresh, original ast.ASTHandle) (err error) {
	return c.InsertWithNegativeSentinel(fresh, original, 0)
}

// InsertWithNegativeSentinel maps the all-ones value to -1 when width is nonzero.
func (c *Eq2BVModelConverter) InsertWithNegativeSentinel(fresh, original ast.ASTHandle, width uint32) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	for _, h := range []ast.ASTHandle{fresh, original} {
		kind, e := c.manager.GetKind(h)
		if e != nil {
			return e
		}
		if kind != ast.ASTFuncDecl {
			return ast.ErrASTDomain
		}
	}
	if width != 0 {
		rangeSort, e := c.manager.GetRange(fresh)
		if e != nil {
			return e
		}
		size, e := c.manager.BitVectorSize(rangeSort)
		if e != nil {
			return e
		}
		if size != width {
			return ast.ErrASTDomain
		}
	}
	if err = c.manager.IncRef(fresh); err != nil {
		return err
	}
	if err = c.manager.IncRef(original); err != nil {
		_, cleanup := c.manager.DecRefConsumed(fresh)
		return errors.Join(err, cleanup)
	}
	c.pairs = append(c.pairs, eq2bvModelPair{fresh: fresh, original: original, negativeWidth: width})
	return nil
}
func (c *Eq2BVModelConverter) ApplyModel(out *ModelRef, goal uint32) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	if goal != 0 {
		return errTacticModelConverterGoal
	}
	source, err := out.Get()
	if err != nil {
		return err
	}
	if source == nil || source.Manager() != c.manager || source.DerivedModel() == nil {
		return ast.ErrASTHandle
	}
	target, err := model.NewModel(c.manager)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, target.Close())
		}
	}()
	if err = target.Core().CopyFunctionInterpretations(source); err != nil {
		return err
	}
	if err = target.CopyUniverseInterpretations(source.DerivedModel()); err != nil {
		return err
	}
	count, err := source.NumConstants()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		decl, e := source.Constant(i)
		if e != nil {
			return e
		}
		value, e := source.ConstantInterpretation(decl)
		if e != nil {
			return e
		}
		for j := len(c.pairs) - 1; j >= 0; j-- {
			pair := c.pairs[j]
			if pair.fresh != decl {
				continue
			}
			text, found, e := c.manager.BitVectorNumeralText(value)
			if e != nil {
				return e
			}
			if found {
				if pair.negativeWidth != 0 {
					valueInt, ok := new(big.Int).SetString(text, 10)
					if !ok {
						return ast.ErrASTDomain
					}
					sentinel := new(big.Int).Lsh(big.NewInt(1), uint(pair.negativeWidth))
					sentinel.Sub(sentinel, big.NewInt(1))
					if valueInt.Cmp(sentinel) == 0 {
						text = "-1"
					}
				}
				value, e = c.manager.ArithmeticIntegerNumeralText(text)
				if e != nil {
					return e
				}
				decl = pair.original
			}
			break
		}
		if e = target.Core().RegisterConstant(decl, value); e != nil {
			return e
		}
	}
	if err = out.Set(target.Core()); err != nil {
		published = out.holds(target.Core())
		return err
	}
	published = true
	return nil
}
func (c *Eq2BVModelConverter) ApplyUnindexed(out *ModelRef) error         { return c.ApplyModel(out, 0) }
func (c *Eq2BVModelConverter) ApplyLabels(_ *ModelLabels, _ uint32) error { return c.open() }
func (c *Eq2BVModelConverter) Cancel() error                              { return c.open() }
func (c *Eq2BVModelConverter) Display(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	return nil
}
func (c *Eq2BVModelConverter) Translate(t *ast.ASTTranslation) (_ ModelConverter, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	if t == nil || t.From() != c.manager || t.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err := NewEq2BVModelConverter(t.To())
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, result.Close())
		}
	}()
	for _, pair := range c.pairs {
		a, e := t.Translate(pair.fresh)
		if e != nil {
			return nil, e
		}
		b, e := t.Translate(pair.original)
		if e != nil {
			return nil, e
		}
		if e = result.InsertWithNegativeSentinel(a, b, pair.negativeWidth); e != nil {
			return nil, e
		}
	}
	published = true
	return result, nil
}
func (c *Eq2BVModelConverter) Close() error {
	if c == nil || c.self != &c.tacticConverterBase {
		return ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	for i := range c.pairs {
		for _, h := range []*ast.ASTHandle{&c.pairs[i].fresh, &c.pairs[i].original} {
			if h.IsNull() {
				continue
			}
			consumed, e := c.manager.DecRefConsumed(*h)
			if consumed {
				*h = ast.ASTHandle{}
			}
			if e != nil {
				return e
			}
			if !consumed {
				return ast.ErrASTPending
			}
		}
	}
	c.pairs = nil
	c.release()
	return nil
}
