package arith

// Source: src/tactic/arith/pb2bv_model_converter.cpp/.h.

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// PB2BVBinding maps an original Int declaration to its Boolean declaration.
// A null Boolean declaration represents the source don't-care value zero.
type PB2BVBinding struct{ Integer, Boolean ast.ASTHandle }

type PB2BVModelConverter struct {
	self       *PB2BVModelConverter
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	bindings   []PB2BVBinding
	held       []ast.ASTHandle
	released   int
	refs       uint32
	ownerBytes uint64
	trace      *diagnostic.TraceTags
	closed     bool
}

var _ tactic.ModelConverter = (*PB2BVModelConverter)(nil)

func NewPB2BVModelConverter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, bindings []PB2BVBinding, bounds *BoundManager) (_ *PB2BVModelConverter, err error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(PB2BVModelConverter{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	r := &PB2BVModelConverter{manager: m, context: c, globals: g, ownerBytes: bytes}
	r.self = r
	complete := false
	defer func() {
		if !complete {
			err = errors.Join(err, r.Close())
		}
	}()
	seen := make(map[ast.ASTHandle]bool)
	for _, pair := range bindings {
		if err = r.appendPair(pair); err != nil {
			return nil, err
		}
		seen[pair.Integer] = true
	}
	if bounds != nil {
		if bounds.Manager() != m {
			return nil, ast.ErrASTHandle
		}
		for _, variable := range bounds.variables {
			constant, e := bounds.constant(variable)
			if e != nil {
				return nil, e
			}
			if !constant {
				return nil, ast.ErrASTDomain
			}
			decl, e := m.GetDecl(variable)
			if e != nil {
				return nil, e
			}
			if !seen[decl] {
				if err = r.appendPair(PB2BVBinding{Integer: decl}); err != nil {
					return nil, err
				}
			}
		}
	}
	complete = true
	return r, nil
}

func (r *PB2BVModelConverter) appendPair(pair PB2BVBinding) error {
	for _, decl := range []ast.ASTHandle{pair.Integer, pair.Boolean} {
		if decl.IsNull() {
			continue
		}
		kind, err := r.manager.GetKind(decl)
		if err != nil {
			return err
		}
		if kind != ast.ASTFuncDecl {
			return ast.ErrASTDomain
		}
		if err = r.manager.IncRef(decl); err != nil {
			return err
		}
		r.held = append(r.held, decl)
	}
	r.bindings = append(r.bindings, pair)
	return nil
}
func (r *PB2BVModelConverter) open() error {
	if r == nil || r.self != r || r.closed {
		return tactic.ErrModelConverterOwner
	}
	return nil
}
func (r *PB2BVModelConverter) IncRef() error {
	if err := r.open(); err != nil {
		return err
	}
	r.refs++
	return nil
}
func (r *PB2BVModelConverter) DecRef() error {
	_, err := r.DecRefConsumed()
	return err
}
func (r *PB2BVModelConverter) DecRefConsumed() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	if r.refs == 0 {
		return false, tactic.ErrModelConverterOwner
	}
	r.refs--
	if r.refs == 0 {
		return true, r.Close()
	}
	return true, nil
}
func (r *PB2BVModelConverter) Close() error {
	if r == nil || r.self != r {
		return tactic.ErrModelConverterOwner
	}
	if r.closed {
		return nil
	}
	for r.released < len(r.held) {
		consumed, err := r.manager.DecRefConsumed(r.held[r.released])
		if consumed {
			r.released++
		}
		if err != nil {
			return err
		}
	}
	r.held, r.bindings = nil, nil
	r.manager.Budget().Release(r.ownerBytes)
	r.ownerBytes = 0
	r.closed = true
	return nil
}
func (r *PB2BVModelConverter) ApplyUnindexed(out *tactic.ModelRef) error {
	return r.ApplyModel(out, 0)
}
func (r *PB2BVModelConverter) ApplyLabels(*tactic.ModelLabels, uint32) error { return r.open() }
func (r *PB2BVModelConverter) Cancel() error                                 { return r.open() }
func (r *PB2BVModelConverter) ApplyModel(out *tactic.ModelRef, index uint32) error {
	if err := r.open(); err != nil {
		return err
	}
	if ast.ASTSourceDebug && index != 0 {
		return ast.ErrASTDomain
	}
	md, err := out.Get()
	if err != nil {
		return err
	}
	if md == nil || md.Manager() != r.manager {
		return ast.ErrASTHandle
	}
	tags := r.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err = tags.EmitConditional("pb2bv", "operator()", "src/tactic/arith/pb2bv_model_converter.cpp", 66, func() bool { return true }, func(out io.Writer) error {
		if _, err := io.WriteString(out, "converting model:\n"); err != nil {
			return err
		}
		if err := md.WriteV2(out, r.context, r.globals, false); err != nil {
			return err
		}
		return r.Display(out)
	})
	if err != nil && !tags.IsStreamFailure(err) {
		return err
	}
	for _, pair := range r.bindings {
		value := "0"
		if !pair.Boolean.IsNull() {
			v, err := md.ConstantInterpretation(pair.Boolean)
			if err != nil {
				return err
			}
			if !v.IsNull() && !r.manager.IsFalse(v) {
				value = "1"
			}
		}
		number, err := r.manager.ArithmeticIntegerNumeralText(value)
		if err != nil {
			return err
		}
		if err = md.RegisterConstant(pair.Integer, number); err != nil {
			return err
		}
	}
	return nil
}
func (r *PB2BVModelConverter) Display(out io.Writer) error {
	if err := r.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if _, err := io.WriteString(out, "(pb2bv-model-converter"); err != nil {
		return err
	}
	for _, pair := range r.bindings {
		names := [2]string{"", "0"}
		for i, decl := range []ast.ASTHandle{pair.Integer, pair.Boolean} {
			if decl.IsNull() {
				continue
			}
			sym, err := r.manager.GetASTName(decl)
			if err != nil {
				return err
			}
			names[i], err = r.manager.Symbols.String(sym)
			if err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "\n  (%s %s)", names[0], names[1]); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")\n")
	return err
}
func (r *PB2BVModelConverter) Translate(translator *ast.ASTTranslation) (_ tactic.ModelConverter, err error) {
	if err = r.open(); err != nil {
		return nil, err
	}
	if translator == nil || translator.From() != r.manager || translator.To() == nil {
		return nil, ast.ErrASTHandle
	}
	other, err := NewPB2BVModelConverter(translator.To(), r.context, r.globals, nil, nil)
	if err != nil {
		return nil, err
	}
	for _, pair := range r.bindings {
		first, e := translator.Translate(pair.Integer)
		if e != nil {
			return nil, errors.Join(e, other.Close())
		}
		// Pin the first result before translating another node can reset caches.
		ref, e := other.manager.NewRef(first)
		if e != nil {
			return nil, errors.Join(e, other.Close())
		}
		second, e := translator.Translate(pair.Boolean)
		if e == nil {
			e = other.appendPair(PB2BVBinding{first, second})
		}
		e = errors.Join(e, other.manager.DelRef(&ref))
		if e != nil {
			return nil, errors.Join(e, other.Close())
		}
	}
	other.trace = r.trace
	return other, nil
}
