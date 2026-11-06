package tactic

// Source: src/tactic/filter_model_converter.cpp:23-57;
// src/tactic/filter_model_converter.h:26-49; src/util/ref_vector.h:96-100.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Original filter converter copies unmarked constants/functions and universes.
type tacticFilterModelConverter struct {
	tacticConverterBase
	manager      *ast.ASTManager
	declarations []ast.ASTHandle // owning entries
	trace        *diagnostic.TraceTags
	globals      *uparams.GlobalParamsService
}

// FilterModelConverter exposes the concrete retained arithmetic-tactic owner.
type FilterModelConverter = tacticFilterModelConverter

func NewFilterModel(m *ast.ASTManager) (*tacticFilterModelConverter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	c := &tacticFilterModelConverter{manager: m}
	if e := c.init(c, m.Budget(), uint64(reflect.TypeOf(*c).Size())); e != nil {
		return nil, e
	}
	return c, nil
}

func (c *tacticFilterModelConverter) Insert(d ast.ASTHandle) error {
	if e := c.open(); e != nil {
		return e
	}
	kind, e := c.manager.GetKind(d)
	if e != nil {
		return e
	}
	if kind != ast.ASTFuncDecl {
		return ast.ErrASTDomain
	}
	// Each published declaration owns its acquired AST reference.
	if e = c.manager.IncRef(d); e != nil {
		return e
	}
	c.declarations = append(c.declarations, d)
	return nil
}

func (c *tacticFilterModelConverter) ApplyModel(out *ModelRef, _ uint32) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	old, e := out.Get()
	if e != nil {
		return e
	}
	if old == nil || old.Manager() != c.manager || old.DerivedModel() == nil {
		return ast.ErrASTHandle
	}
	if e = c.traceFilterModel(old, 26); e != nil {
		return e
	}
	m := c.manager
	marks := make(map[ast.ASTHandle]bool, len(c.declarations))
	for _, d := range c.declarations {
		marks[d] = true
	}
	next, e := model.NewFilteredModel(m)
	if e != nil {
		return e
	}
	// The source raw new_model is published only after copying. Explicitly
	// retire the reached new owner on a Go error before that publication.
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, next.Close())
		}
	}()
	n, e := old.NumConstants()
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		d, e := old.Constant(i)
		if e != nil {
			return e
		}
		if marks[d] {
			continue
		}
		v, e := old.ConstantInterpretation(d)
		if e != nil {
			return e
		}
		if e = next.Core().RegisterConstant(d, v); e != nil {
			return e
		}
	}
	for i := uint32(0); i < old.NumFunctionDeclarations(); i++ {
		d, e := old.Function(i)
		if e != nil {
			return e
		}
		if marks[d] {
			continue
		}
		fi, e := old.FunctionInterpretation(d)
		if e != nil {
			return e
		}
		if fi == nil {
			return ast.ErrASTHandle
		}
		if e = next.Core().CopyFunctionInterpretation(d, fi); e != nil {
			return e
		}
	}
	if e = next.CopyUniverseInterpretations(old.DerivedModel()); e != nil {
		return e
	}
	if e = out.Set(next.Core()); e != nil {
		// SET may acquire next before old-model deletion stops. In that case
		// the output reference retains next as a pending owning receipt;
		// closing this raw local would destroy an already-owned model.
		published = out.holds(next.Core())
		return e
	}
	published = true
	return c.traceFilterModel(next.Core(), 51)
}

func (c *tacticFilterModelConverter) ApplyUnindexed(out *ModelRef) error {
	return c.ApplyModel(out, 0)
}

func (c *tacticFilterModelConverter) Display(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	return c.displayFilter(out)
}

func (c *tacticFilterModelConverter) Translate(translator *ast.ASTTranslation) (_ ModelConverter, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	if translator == nil || translator.From() != c.manager || translator.To() == nil {
		return nil, ast.ErrASTHandle
	}
	result, err := NewFilterModel(translator.To())
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, result.Close())
		}
	}()
	for _, declaration := range c.declarations {
		translated, e := translator.Translate(declaration)
		if e != nil {
			return nil, e
		}
		if e = result.Insert(translated); e != nil {
			return nil, e
		}
	}
	published = true
	return result, nil
}

func (c *tacticFilterModelConverter) ApplyLabels(_ *ModelLabels, _ uint32) error {
	return c.open()
}
func (c *tacticFilterModelConverter) Cancel() error { return c.open() }
func (c *tacticFilterModelConverter) Close() error {
	if c == nil || c.self != &c.tacticConverterBase {
		return ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	// func_decl_ref_vector releases its entries in forward order.
	for i, d := range c.declarations {
		if !d.IsNull() {
			consumed, e := c.manager.DecRefConsumed(d)
			if consumed {
				c.declarations[i] = ast.ASTHandle{}
			}
			if e != nil {
				return e
			}
			if !consumed {
				return ast.ErrASTPending
			}
		}
	}
	c.declarations = nil
	c.release()
	return nil
}
