package ackermannization

// Source: src/ackermannization/ackr_model_converter.cpp/.h;
// src/ackermannization/ackermannize_bv_model_converter.cpp/.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

type conversion struct {
	model            *model.Model
	destination      tactic.ModelRef
	destinationOwned bool
	evaluator        *model.ModelEvaluator
	functions        map[ast.ASTHandle]*model.FunctionInterpretation
	abstract, value  ast.ASTRef
	arguments        []ast.ASTHandle
	failures         []memory.ConstructionFailure
}
type ModelConverter struct {
	self          *ModelConverter
	manager       *ast.ASTManager
	info          *Info
	infoCleanup   bool
	services      model.ModelEvaluatorConfig
	fixed         bool
	abstractModel tactic.ModelRef
	work          []*conversion
	trace         *diagnostic.TraceTags
	refs          uint32
	header        uint64
	closed        bool
}

var _ tactic.ModelConverter = (*ModelConverter)(nil)

func NewModelConverter(m *ast.ASTManager, info *Info, services model.ModelEvaluatorConfig) (*ModelConverter, error) {
	if m == nil || info == nil || info.manager != m {
		return nil, ast.ErrASTHandle
	}
	if err := info.Check(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(ModelConverter{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	c := &ModelConverter{manager: m, info: info, services: services, header: n}
	c.self = c
	if err := info.IncRef(); err != nil {
		m.Budget().Release(n)
		return nil, err
	}
	return c, nil
}
func NewFixedModelConverter(m *ast.ASTManager, info *Info, md *model.ModelCore, services model.ModelEvaluatorConfig) (*ModelConverter, error) {
	c, err := NewModelConverter(m, info, services)
	if err != nil {
		return nil, err
	}
	c.fixed = true
	if err = c.abstractModel.Set(md); err != nil {
		return nil, errors.Join(err, c.Close())
	}
	return c, nil
}
func NewBVModelConverter(m *ast.ASTManager, info *Info, services model.ModelEvaluatorConfig) (*ModelConverter, error) {
	return NewModelConverter(m, info, services)
}
func (c *ModelConverter) open() error {
	if c == nil || c.self != c {
		return tactic.ErrModelConverterOwner
	}
	if c.closed {
		return arena.ErrClosed
	}
	return c.info.Check()
}
func (c *ModelConverter) IncRef() error {
	if err := c.open(); err != nil {
		return err
	}
	c.refs++
	return nil
}
func (c *ModelConverter) DecRef() error { _, err := c.DecRefConsumed(); return err }
func (c *ModelConverter) DecRefConsumed() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if c.refs == 0 {
		return false, arena.ErrBounds
	}
	c.refs--
	if c.refs == 0 {
		return true, c.Close()
	}
	return true, nil
}
func (c *ModelConverter) releaseArguments(w *conversion) error {
	for len(w.arguments) > 0 {
		i := len(w.arguments) - 1
		if err := c.manager.DecRef(w.arguments[i]); err != nil {
			return err
		}
		w.arguments = w.arguments[:i]
	}
	return nil
}
func (c *ModelConverter) closeWork(w *conversion) error {
	for len(w.failures) > 0 {
		i := len(w.failures) - 1
		if err := w.failures[i].Cleanup(); err != nil {
			return err
		}
		w.failures = w.failures[:i]
	}
	if w.evaluator != nil {
		if err := w.evaluator.Close(); err != nil {
			return err
		}
		w.evaluator = nil
	}
	if err := c.releaseArguments(w); err != nil {
		return err
	}
	for _, ref := range []*ast.ASTRef{&w.value, &w.abstract} {
		if *ref != (ast.ASTRef{}) {
			if err := c.manager.DelRef(ref); err != nil {
				return err
			}
			*ref = ast.ASTRef{}
		}
	}
	for d, f := range w.functions {
		if f != nil {
			if err := f.Close(); err != nil {
				return err
			}
			delete(w.functions, d)
		}
	}
	if w.destinationOwned {
		if err := w.destination.Close(); err != nil {
			return err
		}
		w.destinationOwned = false
		w.model = nil
	} else if w.model != nil {
		if err := w.model.Close(); err != nil {
			return err
		}
		w.model = nil
	}
	return nil
}
func (c *ModelConverter) drain() error {
	for len(c.work) > 0 {
		i := len(c.work) - 1
		if err := c.closeWork(c.work[i]); err != nil {
			return err
		}
		c.work = c.work[:i]
	}
	return nil
}
func (c *ModelConverter) Close() error {
	if c == nil || c.self != c {
		return tactic.ErrModelConverterOwner
	}
	if c.closed {
		return nil
	}
	if err := c.drain(); err != nil {
		return err
	}
	if err := c.abstractModel.Close(); err != nil {
		return err
	}
	if c.info != nil {
		if c.infoCleanup {
			if err := c.info.Close(); err != nil {
				return err
			}
		} else {
			consumed, err := c.info.DecRefConsumed()
			if consumed && err != nil {
				c.infoCleanup = true
			}
			if err != nil {
				return err
			}
		}
		c.info = nil
	}
	c.manager.Budget().Release(c.header)
	c.header = 0
	c.closed = true
	return nil
}
func (c *ModelConverter) addEntry(w *conversion, term, value ast.ASTHandle) error {
	m := c.manager
	if err := c.traceEntry(term, value); err != nil {
		return err
	}
	d, err := m.GetDecl(term)
	if err != nil {
		return err
	}
	n, err := m.GetArity(d)
	if err != nil {
		return err
	}
	fi := w.functions[d]
	if fi == nil {
		fi, err = model.NewFunctionInterpretation(m, n)
		if err != nil {
			return err
		}
		w.functions[d] = fi
	}
	if err = c.releaseArguments(w); err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		arg, err := m.GetArg(term, i)
		if err != nil {
			return err
		}
		if err = c.info.Abstract(arg, w.abstract); err != nil {
			return err
		}
		abstract, err := m.RefGet(w.abstract)
		if err != nil {
			return err
		}
		if err = w.evaluator.Evaluate(abstract, w.value); err != nil {
			return err
		}
		v, err := m.RefGet(w.value)
		if err != nil {
			return err
		}
		if err = m.IncRef(v); err != nil {
			return err
		}
		w.arguments = append(w.arguments, v)
	}
	entry, err := fi.FindEntry(w.arguments)
	if err != nil {
		return err
	}
	if entry == nil {
		if err = c.traceNewEntry(d, w.arguments, value); err != nil {
			return err
		}
		return fi.InsertNewEntry(w.arguments, value)
	}
	return c.emit("add_entry", 145, func(out io.Writer) error { _, err := io.WriteString(out, "entry already present\n"); return err })
}
func (c *ModelConverter) ApplyModel(out *tactic.ModelRef, index uint32) (err error) {
	if err = c.open(); err != nil {
		return err
	}
	if err = c.drain(); err != nil {
		return err
	}
	current, err := out.Get()
	if err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		if index != 0 {
			return ast.ErrASTDomain
		}
		if c.fixed && current != nil {
			n, e := current.NumConstants()
			if e != nil {
				return e
			}
			if n != 0 || current.NumFunctionDeclarations() != 0 {
				return ast.ErrASTDomain
			}
		}
	}
	source := current
	if c.fixed {
		source, err = c.abstractModel.Get()
		if err != nil {
			return err
		}
	}
	if source == nil || source.Manager() != c.manager || source.DerivedModel() == nil {
		return ast.ErrASTHandle
	}
	w := &conversion{functions: make(map[ast.ASTHandle]*model.FunctionInterpretation)}
	c.work = append(c.work, w)
	defer func() {
		var failure memory.ConstructionFailure
		if errors.As(err, &failure) {
			w.failures = append(w.failures, failure)
		}
		err = errors.Join(err, c.drain())
	}()
	w.model, err = model.NewModel(c.manager)
	if err != nil {
		return err
	}
	if err = w.destination.Set(w.model.Core()); err != nil {
		return err
	}
	w.destinationOwned = true
	if err = w.model.Core().CopyFunctionInterpretations(source); err != nil {
		return err
	}
	if err = w.model.CopyUniverseInterpretations(source.DerivedModel()); err != nil {
		return err
	}
	if err = c.emit("convert_constants", 84, func(out io.Writer) error { _, e := io.WriteString(out, "converting constants\n"); return e }); err != nil {
		return err
	}
	w.evaluator, err = model.NewModelEvaluator(source, c.services)
	if err != nil {
		return err
	}
	if err = w.evaluator.SetModelCompletion(true); err != nil {
		return err
	}
	w.abstract, err = c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	w.value, err = c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	// Completion can add constants to source; the source loop re-reads its size.
	for i := uint32(0); ; i++ {
		n, e := source.NumConstants()
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		d, e := source.Constant(i)
		if e != nil {
			return e
		}
		term, e := c.info.FindTerm(d)
		if e != nil {
			return e
		}
		value, e := source.ConstantInterpretation(d)
		if e != nil {
			return e
		}
		if term.IsNull() {
			e = w.model.Core().RegisterConstant(d, value)
		} else {
			e = c.addEntry(w, term, value)
		}
		if e != nil {
			return e
		}
	}
	for d, fi := range w.functions {
		sort, e := c.manager.GetRange(d)
		if e != nil {
			return e
		}
		value, e := c.manager.GetSomeValue(sort)
		if e != nil {
			return e
		}
		if e = fi.SetElse(value); e != nil {
			return e
		}
		if e = w.model.Core().RegisterFunction(d, fi); e != nil {
			return e
		}
		w.functions[d] = nil
	}
	if err = w.evaluator.Close(); err != nil {
		return err
	}
	w.evaluator = nil
	return out.Set(w.model.Core())
}
func (c *ModelConverter) ApplyUnindexed(out *tactic.ModelRef) error     { return c.ApplyModel(out, 0) }
func (c *ModelConverter) ApplyLabels(*tactic.ModelLabels, uint32) error { return c.open() }
func (c *ModelConverter) Cancel() error                                 { return c.open() }
func (c *ModelConverter) Display(io.Writer) error                       { return c.open() }
func (c *ModelConverter) Translate(tr *ast.ASTTranslation) (result tactic.ModelConverter, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	if tr == nil || tr.From() != c.manager {
		return nil, ast.ErrASTHandle
	}
	info, err := c.info.Translate(tr)
	if err != nil {
		return nil, err
	}
	if err = info.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, info.DecRef()) }()
	if !c.fixed {
		return NewModelConverter(tr.To(), info, c.services)
	}
	source, err := c.abstractModel.Get()
	if err != nil {
		return nil, err
	}
	if source == nil || source.DerivedModel() == nil {
		return nil, ast.ErrASTHandle
	}
	translated, err := source.DerivedModel().Translate(tr)
	if err != nil {
		return nil, err
	}
	var held tactic.ModelRef
	if err = held.Set(translated.Core()); err != nil {
		return nil, errors.Join(err, translated.Close())
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	return NewFixedModelConverter(tr.To(), info, translated.Core(), c.services)
}
