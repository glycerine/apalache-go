package bv

// Source: src/tactic/bv/bit_blaster_model_converter.cpp/.h.

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// BitBlasterBinding associates an original BV declaration with its MKBV of
// Boolean constants, or its CONCAT of one-bit BV constants for the BV1 factory.
type BitBlasterBinding struct{ Variable, Bits ast.ASTHandle }

// BitBlasterModelConverter implements the two retained source Boolean template
// instantiations with concrete AST handles. It owns both declaration and bit
// expression references; model conversion publishes a separately owned model.
type BitBlasterModelConverter struct {
	self       *BitBlasterModelConverter
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	variables  []ast.ASTHandle
	bits       []ast.ASTHandle
	held       []ast.ASTHandle
	released   int
	refs       uint32
	toBool     bool
	ownerBytes uint64
	trace      *diagnostic.TraceTags
	scratch    *tactic.ModelRef
	closed     bool
}

var _ tactic.ModelConverter = (*BitBlasterModelConverter)(nil)

func NewBitBlasterModelConverter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, bindings []BitBlasterBinding) (*BitBlasterModelConverter, error) {
	return newBitBlasterModelConverter(m, c, g, bindings, true)
}
func NewBV1BlasterModelConverter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, bindings []BitBlasterBinding) (*BitBlasterModelConverter, error) {
	return newBitBlasterModelConverter(m, c, g, bindings, false)
}
func newBitBlasterModelConverter(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, bindings []BitBlasterBinding, toBool bool) (*BitBlasterModelConverter, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(BitBlasterModelConverter{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	r := &BitBlasterModelConverter{manager: m, context: c, globals: g, toBool: toBool, ownerBytes: bytes}
	r.self = r
	for _, pair := range bindings {
		kind, err := m.GetKind(pair.Variable)
		if err == nil && kind != ast.ASTFuncDecl {
			err = ast.ErrASTDomain
		}
		if err == nil && ast.ASTSourceDebug {
			err = r.checkAssembly(pair.Bits)
		}
		if err == nil {
			err = r.hold(pair.Variable)
		}
		if err == nil {
			r.variables = append(r.variables, pair.Variable)
			err = r.hold(pair.Bits)
		}
		if err != nil {
			return nil, errors.Join(err, r.Close())
		}
		r.bits = append(r.bits, pair.Bits)
	}
	return r, nil
}

func (r *BitBlasterModelConverter) checkAssembly(bits ast.ASTHandle) error {
	op := int32(37) // OP_CONCAT
	if r.toBool {
		op = 56 // OP_MKBV
	}
	matched, err := r.manager.IsBitVectorOperation(bits, op)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	return nil
}
func (r *BitBlasterModelConverter) hold(h ast.ASTHandle) error {
	if err := r.manager.IncRef(h); err != nil {
		return err
	}
	r.held = append(r.held, h)
	return nil
}
func (r *BitBlasterModelConverter) open() error {
	if r == nil || r.self != r || r.closed {
		return tactic.ErrModelConverterOwner
	}
	return nil
}
func (r *BitBlasterModelConverter) IncRef() error {
	if err := r.open(); err != nil {
		return err
	}
	r.refs++
	return nil
}
func (r *BitBlasterModelConverter) DecRef() error { _, err := r.DecRefConsumed(); return err }
func (r *BitBlasterModelConverter) DecRefConsumed() (bool, error) {
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
func (r *BitBlasterModelConverter) drainModel() error {
	if r.scratch != nil {
		if err := r.scratch.Close(); err != nil {
			return err
		}
		r.scratch = nil
	}
	return nil
}
func (r *BitBlasterModelConverter) Close() error {
	if r == nil || r.self != r {
		return tactic.ErrModelConverterOwner
	}
	if r.closed {
		return nil
	}
	if err := r.drainModel(); err != nil {
		return err
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
	r.variables, r.bits, r.held = nil, nil, nil
	r.manager.Budget().Release(r.ownerBytes)
	r.ownerBytes = 0
	r.closed = true
	return nil
}
func (r *BitBlasterModelConverter) ApplyUnindexed(out *tactic.ModelRef) error {
	return r.ApplyModel(out, 0)
}
func (r *BitBlasterModelConverter) ApplyLabels(*tactic.ModelLabels, uint32) error { return r.open() }
func (r *BitBlasterModelConverter) Cancel() error                                 { return r.open() }

func (r *BitBlasterModelConverter) collectBits() (map[ast.ASTHandle]bool, error) {
	bits := make(map[ast.ASTHandle]bool)
	var ordered []ast.ASTHandle
	for _, bs := range r.bits {
		if ast.ASTSourceDebug {
			if err := r.checkAssembly(bs); err != nil {
				return nil, err
			}
		}
		n, err := r.manager.GetNumArgs(bs)
		if err != nil {
			return nil, err
		}
		for j := uint32(0); j < n; j++ {
			bit, err := r.manager.GetArg(bs, j)
			if err != nil {
				return nil, err
			}
			if ast.ASTSourceDebug {
				if err = r.checkBit(bit, false); err != nil {
					return nil, err
				}
			}
			d, err := r.manager.GetDecl(bit)
			if err != nil {
				return nil, err
			}
			if !bits[d] {
				ordered = append(ordered, d)
			}
			bits[d] = true
		}
	}
	err := r.emit("collect_bits", 67, func(w io.Writer) error {
		if _, err := io.WriteString(w, "bits that should not be included in the model:\n"); err != nil {
			return err
		}
		for _, d := range ordered {
			name, err := r.name(d)
			if err != nil {
				return err
			}
			if _, err = io.WriteString(w, name+" "); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
	return bits, err
}
func (r *BitBlasterModelConverter) checkBit(bit ast.ASTHandle, width bool) error {
	d, err := r.manager.GetDecl(bit)
	if err != nil {
		return err
	}
	f, err := r.manager.GetFamilyID(d)
	if err != nil {
		return err
	}
	n, err := r.manager.GetNumArgs(bit)
	if err != nil {
		return err
	}
	if f != ast.NullFamilyID || n != 0 {
		return ast.ErrASTDomain
	}
	sort, err := r.manager.GetSort(bit)
	if err != nil {
		return err
	}
	if r.toBool {
		boolean, err := r.manager.MkBoolSort()
		if err != nil {
			return err
		}
		if sort != boolean {
			return ast.ErrASTDomain
		}
	} else {
		bv, err := r.manager.IsBitVectorSort(sort)
		if err != nil {
			return err
		}
		if !bv {
			return ast.ErrASTDomain
		}
		if width {
			n, err := r.manager.BitVectorSize(sort)
			if err != nil {
				return err
			}
			if n != 1 {
				return ast.ErrASTDomain
			}
		}
	}
	return nil
}

func (r *BitBlasterModelConverter) ApplyModel(out *tactic.ModelRef, index uint32) (err error) {
	if err = r.open(); err != nil {
		return err
	}
	if ast.ASTSourceDebug && index != 0 {
		return ast.ErrASTDomain
	}
	old, err := out.Get()
	if err != nil {
		return err
	}
	if old == nil || old.Manager() != r.manager {
		return ast.ErrASTHandle
	}
	if err = r.drainModel(); err != nil {
		return err
	}
	fresh, err := model.NewModel(r.manager)
	if err != nil {
		return err
	}
	r.scratch = &tactic.ModelRef{}
	if err = r.scratch.Set(fresh.Core()); err != nil {
		return errors.Join(err, fresh.Close())
	}
	defer func() { err = errors.Join(err, r.drainModel()) }()
	bits, err := r.collectBits()
	if err != nil {
		return err
	}
	n, err := old.NumConstants()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		d, e := old.Constant(i)
		if e != nil {
			return e
		}
		if bits[d] {
			continue
		}
		if e = r.emit("copy_non_bits", 84, func(w io.Writer) error {
			name, e := r.name(d)
			if e != nil {
				return e
			}
			_, e = io.WriteString(w, "non-bit: "+name+"\n")
			return e
		}); e != nil {
			return e
		}
		v, e := old.ConstantInterpretation(d)
		if e != nil {
			return e
		}
		if e = fresh.Core().RegisterConstant(d, v); e != nil {
			return e
		}
	}
	if err = r.traceModel("after copy non bits:\n", 88, fresh.Core()); err != nil {
		return err
	}
	if err = fresh.Core().CopyFunctionInterpretations(old); err != nil {
		return err
	}
	if derived := old.DerivedModel(); derived != nil {
		if err = fresh.CopyUniverseInterpretations(derived); err != nil {
			return err
		}
	}
	if err = r.traceModel("after copying functions and sorts:\n", 91, fresh.Core()); err != nil {
		return err
	}
	for i, d := range r.variables {
		value, e := old.ConstantInterpretation(d)
		if e != nil {
			return e
		}
		if value.IsNull() {
			value, e = r.reconstruct(old, r.bits[i])
			if e != nil {
				return e
			}
		}
		if e = fresh.Core().RegisterConstant(d, value); e != nil {
			return e
		}
	}
	return out.Set(fresh.Core())
}

func (r *BitBlasterModelConverter) reconstruct(md *model.ModelCore, bs ast.ASTHandle) (_ ast.ASTHandle, err error) {
	n, err := r.manager.GetNumArgs(bs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var value big.Int
	for k := uint32(0); k < n; k++ {
		j := k
		if r.toBool {
			j = n - 1 - k
		}
		value.Lsh(&value, 1)
		bit, e := r.manager.GetArg(bs, j)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if ast.ASTSourceDebug {
			if e = r.checkBit(bit, true); e != nil {
				return ast.ASTHandle{}, e
			}
		}
		d, e := r.manager.GetDecl(bit)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		v, e := md.ConstantInterpretation(d)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		one := !v.IsNull() && r.manager.IsTrue(v)
		if !r.toBool && !v.IsNull() {
			text, matched, e := r.manager.BitVectorNumeralText(v)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			one = !matched || text != "0"
		}
		if one {
			value.SetBit(&value, 0, 1)
		}
	}
	q := r.manager.NumeralService()
	number, err := q.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, q.Del(&number)) }()
	if err = q.SetString(&number, value.String()); err != nil {
		return ast.ASTHandle{}, err
	}
	return r.manager.BitVectorNumeral(&number, n)
}

func (r *BitBlasterModelConverter) name(decl ast.ASTHandle) (string, error) {
	sym, err := r.manager.GetASTName(decl)
	if err != nil {
		return "", err
	}
	return r.manager.Symbols.String(sym)
}
func (r *BitBlasterModelConverter) emit(function string, line int, body func(io.Writer) error) error {
	tags := r.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("blaster_mc", function, "src/tactic/bv/bit_blaster_model_converter.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (r *BitBlasterModelConverter) traceModel(prefix string, line int, md *model.ModelCore) error {
	return r.emit("copy_non_bits", line, func(w io.Writer) error {
		if _, err := io.WriteString(w, prefix); err != nil {
			return err
		}
		return md.WriteDebug(w, r.context, r.globals)
	})
}
func (r *BitBlasterModelConverter) Display(out io.Writer) (err error) {
	if err = r.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if _, err = io.WriteString(out, "(bit-blaster-model-converter"); err != nil {
		return err
	}
	for i, d := range r.variables {
		name, e := r.name(d)
		if e != nil {
			return e
		}
		if _, e = fmt.Fprintf(out, "\n  (%s ", name); e != nil {
			return e
		}
		f, e := r.manager.SMT2PPExprTree(r.bits[i], nil)
		if e != nil {
			return e
		}
		f, e = r.manager.FormatIndent(uint32(len(name))+4, f)
		if e != nil {
			return e
		}
		stream, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: r.manager.Budget()})
		if e != nil {
			return e
		}
		p := r.context.EmptyValue()
		e = r.manager.PrintFormat(stream, f, r.context, &p, r.globals)
		if e == nil {
			var text string
			text, e = stream.String()
			if e == nil {
				_, e = io.WriteString(out, text)
			}
		}
		e = errors.Join(e, stream.Close())
		if e != nil {
			return e
		}
		if _, e = io.WriteString(out, ")"); e != nil {
			return e
		}
	}
	_, err = io.WriteString(out, ")\n")
	return err
}
func (r *BitBlasterModelConverter) Translate(tr *ast.ASTTranslation) (_ tactic.ModelConverter, err error) {
	if err = r.open(); err != nil {
		return nil, err
	}
	if tr == nil || tr.From() != r.manager || tr.To() == nil {
		return nil, ast.ErrASTHandle
	}
	other, err := newBitBlasterModelConverter(tr.To(), r.context, r.globals, nil, r.toBool)
	if err != nil {
		return nil, err
	}
	complete := false
	defer func() {
		if !complete {
			err = errors.Join(err, other.Close())
		}
	}()
	// Source translation owns all variables before translating any bits.
	for _, d := range r.variables {
		h, e := tr.Translate(d)
		if e != nil {
			return nil, e
		}
		if e = other.hold(h); e != nil {
			return nil, e
		}
		other.variables = append(other.variables, h)
	}
	for _, bs := range r.bits {
		h, e := tr.Translate(bs)
		if e != nil {
			return nil, e
		}
		if e = other.hold(h); e != nil {
			return nil, e
		}
		other.bits = append(other.bits, h)
	}
	other.trace = r.trace
	complete = true
	return other, nil
}
