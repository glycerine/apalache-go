package tactic

// Source: src/sat/tactic/goal2sat.cpp:556-678.

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// SATModelConverter lifts the SAT elimination log to AST models. Atoms are
// indexed by SAT variable, including auxiliary variables removed after recovery.
type SATModelConverter struct {
	self            *SATModelConverter
	manager         *ast.ASTManager
	services        model.ModelEvaluatorConfig
	conversion      *sat.ModelConverter
	filter          *tactic.FilterModelConverter
	atoms           []ast.ASTHandle
	released        int
	refs            uint32
	ownerBytes      uint64
	evaluator       *model.ModelEvaluator
	value           ast.ASTRef
	valueLive       bool
	closing, closed bool
}

var _ tactic.ModelConverter = (*SATModelConverter)(nil)

func NewSATModelConverter(m *ast.ASTManager, s *sat.Solver, services model.ModelEvaluatorConfig) (*SATModelConverter, error) {
	if s == nil {
		return nil, sat.ErrSATOwner
	}
	if err := s.CheckOwner(); err != nil {
		return nil, err
	}
	r, err := newSATModelConverter(m, services)
	if err == nil {
		err = s.CopyModelConverter(r.conversion)
	}
	if err != nil && r != nil {
		if cleanup := r.Close(); cleanup != nil {
			return r, errors.Join(err, cleanup)
		}
		return nil, err
	}
	return r, err
}

func newSATModelConverter(m *ast.ASTManager, services model.ModelEvaluatorConfig) (*SATModelConverter, error) {
	if m == nil || services.Context == nil || services.Globals == nil || services.Globals.Context != services.Context || services.Memory == nil || services.Cooperation == nil {
		return nil, tactic.ErrModelConverterOwner
	}
	if err := services.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(SATModelConverter{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	// The source creates a default model_evaluator, independent of tactic params.
	services.Params = nil
	r := &SATModelConverter{manager: m, services: services, ownerBytes: bytes}
	r.self = r
	var err error
	r.conversion, err = sat.NewModelConverter(m.Budget())
	if err == nil {
		r.filter, err = tactic.NewFilterModel(m)
	}
	if err == nil {
		err = r.filter.BindTrace(services.Trace, services.Globals)
	}
	if err != nil {
		if cleanup := r.Close(); cleanup != nil {
			return r, errors.Join(err, cleanup)
		}
		return nil, err
	}
	return r, nil
}
func (r *SATModelConverter) open() error {
	if r == nil || r.self != r || r.closing || r.closed {
		return tactic.ErrModelConverterOwner
	}
	return nil
}
func (r *SATModelConverter) Insert(atom ast.ASTHandle, auxiliary bool) error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.manager.IncRef(atom); err != nil {
		return err
	}
	r.atoms = append(r.atoms, atom)
	if auxiliary {
		if ast.ASTSourceDebug {
			constant, err := r.uninterpretedConstant(atom)
			if err != nil {
				return err
			}
			sort, err := r.manager.GetSort(atom)
			if err != nil {
				return err
			}
			boolean, err := r.manager.MkBoolSort()
			if err != nil {
				return err
			}
			if !constant || sort != boolean {
				return sat.ErrSATAssertion
			}
		}
		d, err := r.manager.GetDecl(atom)
		if err != nil {
			return err
		}
		return r.filter.Insert(d)
	}
	return nil
}
func (r *SATModelConverter) uninterpretedConstant(atom ast.ASTHandle) (bool, error) {
	kind, err := r.manager.GetKind(atom)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	n, err := r.manager.GetNumArgs(atom)
	if err != nil || n != 0 {
		return false, err
	}
	family, err := r.manager.GetFamilyID(atom)
	return family == ast.NullFamilyID, err
}
func (r *SATModelConverter) IncRef() error {
	if err := r.open(); err != nil {
		return err
	}
	r.refs++
	return nil
}
func (r *SATModelConverter) DecRef() error { _, err := r.DecRefConsumed(); return err }
func (r *SATModelConverter) DecRefConsumed() (bool, error) {
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
func (r *SATModelConverter) drainEvaluation() error {
	if r.valueLive {
		if err := r.manager.DelRef(&r.value); err != nil {
			return err
		}
		r.valueLive = false
	}
	if r.evaluator != nil {
		if err := r.evaluator.Close(); err != nil {
			return err
		}
		r.evaluator = nil
	}
	return nil
}
func (r *SATModelConverter) Close() error {
	if r == nil || r.self != r {
		return tactic.ErrModelConverterOwner
	}
	if r.closed {
		return nil
	}
	r.closing = true
	if err := r.drainEvaluation(); err != nil {
		return err
	}
	if r.filter != nil {
		if err := r.filter.Close(); err != nil {
			return err
		}
		r.filter = nil
	}
	for r.released < len(r.atoms) {
		consumed, err := r.manager.DecRefConsumed(r.atoms[r.released])
		if consumed {
			r.released++
		}
		if err != nil {
			return err
		}
	}
	if r.conversion != nil {
		if err := r.conversion.Close(); err != nil {
			return err
		}
		r.conversion = nil
	}
	r.atoms = nil
	r.manager.Budget().Release(r.ownerBytes)
	r.ownerBytes = 0
	r.closed = true
	return nil
}
func (r *SATModelConverter) ApplyModel(out *tactic.ModelRef, index uint32) (err error) {
	if err = r.open(); err != nil {
		return err
	}
	if ast.ASTSourceDebug && index != 0 {
		return sat.ErrSATAssertion
	}
	md, err := out.Get()
	if err != nil {
		return err
	}
	if md == nil || md.Manager() != r.manager {
		return ast.ErrASTHandle
	}
	if err = r.traceModel("before sat_mc\n", 598, md, true); err != nil {
		return err
	}
	if err = r.drainEvaluation(); err != nil {
		return err
	}
	r.evaluator, err = model.NewModelEvaluator(md, r.services)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, r.drainEvaluation()) }()
	if err = r.evaluator.SetModelCompletion(false); err != nil {
		return err
	}
	r.value, err = r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	r.valueLive = true
	values := make(sat.Model, len(r.atoms))
	for v, atom := range r.atoms {
		if err = r.evaluator.Evaluate(atom, r.value); err != nil {
			return err
		}
		h, e := r.manager.RefGet(r.value)
		if e != nil {
			return e
		}
		yes := r.manager.IsTrue(h)
		no := r.manager.IsFalse(h)
		values[v] = lbool.Undef
		if yes {
			values[v] = lbool.True
		} else if no {
			values[v] = lbool.False
		}
	}
	if err = r.conversion.Apply(values); err != nil {
		return err
	}
	for v, atom := range r.atoms {
		constant, e := r.uninterpretedConstant(atom)
		if e != nil {
			return e
		}
		if !constant || values[v] == lbool.Undef {
			continue
		}
		d, e := r.manager.GetDecl(atom)
		if e != nil {
			return e
		}
		var value ast.ASTHandle
		if values[v] == lbool.True {
			value, e = r.manager.MkTrue()
		} else {
			value, e = r.manager.MkFalse()
		}
		if e != nil {
			return e
		}
		if e = md.RegisterConstant(d, value); e != nil {
			return e
		}
	}
	if err = r.filter.ApplyUnindexed(out); err != nil {
		return err
	}
	md, err = out.Get()
	if err != nil {
		return err
	}
	return r.traceModel("after sat_mc\n", 644, md, false)
}
func (r *SATModelConverter) ApplyUnindexed(out *tactic.ModelRef) error     { return r.ApplyModel(out, 0) }
func (r *SATModelConverter) ApplyLabels(*tactic.ModelLabels, uint32) error { return r.open() }
func (r *SATModelConverter) Cancel() error                                 { return r.open() }

func (r *SATModelConverter) Translate(tr *ast.ASTTranslation) (result tactic.ModelConverter, err error) {
	if err = r.open(); err != nil {
		return nil, err
	}
	if tr == nil || tr.From() != r.manager || tr.To() == nil {
		return nil, ast.ErrASTHandle
	}
	other, err := newSATModelConverter(tr.To(), r.services)
	if err != nil {
		if other == nil {
			return nil, err
		}
		return other, err
	}
	complete := false
	defer func() {
		if !complete {
			if cleanup := other.Close(); cleanup != nil {
				result = other
				err = errors.Join(err, cleanup)
			}
		}
	}()
	filter, err := r.filter.Translate(tr)
	if err != nil {
		return nil, err
	}
	if err = other.filter.Close(); err != nil {
		return nil, errors.Join(err, filter.Close())
	}
	other.filter = filter.(*tactic.FilterModelConverter)
	for _, atom := range r.atoms {
		h, e := tr.Translate(atom)
		if e != nil {
			return nil, e
		}
		if e = other.Insert(h, false); e != nil {
			return nil, e
		}
	}
	// Source translation deliberately does not copy m_mc.
	complete = true
	return other, nil
}
func (r *SATModelConverter) Display(out io.Writer) (err error) {
	if err = r.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if _, err = io.WriteString(out, "(sat-model-converter\n"); err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: r.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	if err = r.conversion.DisplayInto(stream); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	if _, err = io.WriteString(out, text+"(atoms"); err != nil {
		return err
	}
	vars, err := r.conversion.AppendVariables(nil)
	if err != nil {
		return err
	}
	used := make(map[sat.BoolVar]bool)
	for _, v := range vars {
		used[v] = true
	}
	for i, atom := range r.atoms {
		if !used[sat.BoolVar(i)] {
			continue
		}
		if _, err = fmt.Fprintf(out, "\n (%d\n  ", i); err != nil {
			return err
		}
		if err = r.writeAtom(out, atom); err != nil {
			return err
		}
		if _, err = io.WriteString(out, ")"); err != nil {
			return err
		}
	}
	if _, err = io.WriteString(out, ")\n"); err != nil {
		return err
	}
	if err = r.filter.Display(out); err != nil {
		return err
	}
	_, err = io.WriteString(out, ")\n")
	return err
}
func (r *SATModelConverter) writeAtom(out io.Writer, atom ast.ASTHandle) (err error) {
	f, err := r.manager.SMT2PPExprTree(atom, nil)
	if err != nil {
		return err
	}
	f, err = r.manager.FormatIndent(2, f)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: r.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	p := r.services.Context.EmptyValue()
	if err = r.manager.PrintFormat(stream, f, r.services.Context, &p, r.services.Globals); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}
func (r *SATModelConverter) traceModel(prefix string, line int, md *model.ModelCore, display bool) error {
	tags := r.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("sat_mc", "operator()", "src/sat/tactic/goal2sat.cpp", line, func() bool { return true }, func(w io.Writer) error {
		if _, err := io.WriteString(w, prefix); err != nil {
			return err
		}
		if err := md.WriteDebug(w, r.services.Context, r.services.Globals); err != nil {
			return err
		}
		if display {
			return r.Display(w)
		}
		return nil
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
