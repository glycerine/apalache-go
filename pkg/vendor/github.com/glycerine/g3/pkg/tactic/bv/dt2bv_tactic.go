package bv

// Source: src/tactic/bv/dt2bv_tactic.cpp/.h; src/ast/for_each_expr.h:27-139.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// DT2BVTactic eliminates enum sorts whose uses are all supported by enum2bv.
// Classification tables borrow sorts and persist until source cleanup().
type DT2BVTactic struct {
	self              *DT2BVTactic
	manager           *ast.ASTManager
	context           *uparams.ParamsContext
	services          model.ModelEvaluatorConfig
	parameters        uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	utility           *ast.DatatypeUtil
	finite, nonFinite map[ast.ASTHandle]bool
	pending           *dt2bvInvocation
	refs              uint32
	header            uint64
	closing, closed   bool
}

var _ tactic.Tactic = (*DT2BVTactic)(nil)

// Construction failure retains a concrete owner if destruction itself stops.
type DT2BVConstructionFailure struct {
	owner          *DT2BVTactic
	cause, cleanup error
}

func (e *DT2BVConstructionFailure) Error() string                 { return e.Unwrap().Error() }
func (e *DT2BVConstructionFailure) Unwrap() error                 { return errors.Join(e.cause, e.cleanup) }
func (e *DT2BVConstructionFailure) OriginalCause() error          { return e.cause }
func (e *DT2BVConstructionFailure) CleanupCause() error           { return e.cleanup }
func (e *DT2BVConstructionFailure) RuntimeOwner() *memory.Runtime { return e.owner.services.Memory }
func (e *DT2BVConstructionFailure) Cleanup() error                { return e.owner.Close() }

func NewDT2BVTactic(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*DT2BVTactic, error) {
	if m == nil || c == nil || services.Context != c || services.Globals == nil || services.Globals.Context != c || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(DT2BVTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &DT2BVTactic{manager: m, context: c, services: services, finite: make(map[ast.ASTHandle]bool), nonFinite: make(map[ast.ASTHandle]bool), header: n}
	t.self = t
	var err error
	t.parameters, err = c.NewValue()
	if err == nil && p != nil {
		err = c.Assign(&t.parameters, p)
	}
	if err == nil {
		t.utility, err = ast.NewDatatypeUtil(m)
	}
	if err != nil {
		if cleanup := t.Close(); cleanup != nil {
			return nil, &DT2BVConstructionFailure{owner: t, cause: err, cleanup: cleanup}
		}
		return nil, err
	}
	return t, nil
}
func (t *DT2BVTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.closing || t.pending != nil {
		return ast.ErrASTPending
	}
	_, err := t.manager.ProofsEnabled()
	return err
}
func (t *DT2BVTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *DT2BVTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *DT2BVTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *DT2BVTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *DT2BVTactic) CollectStatistics() map[string]float64         { return nil }
func (t *DT2BVTactic) ResetStatistics() error                        { return t.open() }
func (t *DT2BVTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *DT2BVTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *DT2BVTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	clear(t.finite)
	clear(t.nonFinite)
	return nil
}
func (t *DT2BVTactic) Reset() error { return t.Cleanup() }
func (t *DT2BVTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewDT2BVTactic(m, t.context, &t.parameters, t.services)
}
func (t *DT2BVTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	t.closing = true
	if t.pending != nil {
		if err := t.pending.close(); err != nil {
			return err
		}
		t.pending = nil
	}
	t.finite, t.nonFinite = nil, nil
	if t.utility != nil {
		if err := t.utility.Close(); err != nil {
			return err
		}
		t.utility = nil
	}
	if t.parameters != (uparams.ParamsRef{}) {
		var err error
		if t.decrement.Invocation() != 0 {
			err = t.context.CompleteDecrement(&t.parameters, t.decrement)
		} else {
			err = t.context.DelTracked(&t.parameters, &t.decrement)
		}
		if err != nil {
			return err
		}
	}
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.closed = true
	return nil
}

// Concrete lexical owners for operator(); interrupted retirement is kept by
// the tactic before any of its members can be destroyed.
type dt2bvInvocation struct {
	owner                           *DT2BVTactic
	report                          *tactic.TacticReport
	visited                         *ast.ASTFastMark1
	extension                       *tactic.ExtensionModelConverter
	filter                          *tactic.FilterModelConverter
	extensionCleanup, filterCleanup bool
	rw                              *rewriter.Enum2BVRewriter
	current, proof                  ast.ASTRef
	bounds                          []ast.ASTRef
	concat                          tactic.ModelConverter
}

func (s *dt2bvInvocation) close() error {
	m := s.owner.manager
	if s.concat != nil {
		if err := s.concat.Close(); err != nil {
			return err
		}
		s.concat = nil
	}
	for i := range s.bounds {
		if s.bounds[i] != (ast.ASTRef{}) {
			if err := m.DelRef(&s.bounds[i]); err != nil {
				return err
			}
		}
	}
	s.bounds = nil
	for _, p := range []*ast.ASTRef{&s.proof, &s.current} {
		if *p != (ast.ASTRef{}) {
			if err := m.DelRef(p); err != nil {
				return err
			}
		}
	}
	if s.rw != nil {
		if err := s.rw.Close(); err != nil {
			return err
		}
		s.rw = nil
	}
	if s.filter != nil {
		if err := dt2bvReleaseConverter(s.filter, &s.filterCleanup); err != nil {
			return err
		}
		s.filter = nil
	}
	if s.extension != nil {
		if err := dt2bvReleaseConverter(s.extension, &s.extensionCleanup); err != nil {
			return err
		}
		s.extension = nil
	}
	if s.visited != nil {
		if err := s.visited.Close(); err != nil {
			return err
		}
		s.visited = nil
	}
	if s.report != nil {
		if err := s.report.Close(); err != nil {
			return err
		}
		s.report = nil
	}
	return nil
}
func dt2bvReleaseConverter(c tactic.ModelConverter, pending *bool) error {
	if *pending {
		return c.Close()
	}
	consumed, err := c.DecRefConsumed()
	if consumed && err != nil {
		*pending = true
	}
	if !consumed && err == nil {
		return tactic.ErrModelConverterOwner
	}
	return err
}
func (t *DT2BVTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || g.Manager() != t.manager || result == nil {
		return ast.ErrASTHandle
	}
	// Source clears converters/core, but appends to the caller's goal buffer.
	if err = result.BindManager(t.manager); err != nil {
		return err
	}
	if err = result.SetModelConverter(nil); err != nil {
		return err
	}
	if err = result.SetProofConverter(nil); err != nil {
		return err
	}
	if err = result.SetCore(ast.ASTDependency{}); err != nil {
		return err
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return err
	}
	s := &dt2bvInvocation{owner: t}
	defer func() {
		if cleanup := s.close(); cleanup != nil {
			t.pending = s
			err = errors.Join(err, cleanup)
		}
	}()
	s.report, err = tactic.NewTacticReport("dt2bv", g)
	if err != nil {
		return err
	}
	size, err := g.Size()
	if err != nil {
		return err
	}
	s.visited, err = ast.NewASTFastMark1(t.manager)
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		root, e := g.Form(i)
		if e != nil {
			return e
		}
		if e = t.classify(root, s.visited); e != nil {
			return e
		}
	}
	for sort := range t.nonFinite {
		delete(t.finite, sort)
	}
	if len(t.finite) != 0 {
		if err = t.convert(g, result, s, size, proofs); err != nil {
			return err
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(g); err != nil {
		return err
	}
	tags := diagnostic.BorrowProcessTraceTags()
	err = tags.EmitConditional("dt2bv", "operator()", "src/tactic/bv/dt2bv_tactic.cpp", 176, func() bool { return true }, func(w io.Writer) error { return g.Display(w) })
	if err != nil && !tags.IsStreamFailure(err) {
		return err
	}
	if diagnostic.DebugBuild {
		debug := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(debug)
		if e != nil {
			return e
		}
		if enabled {
			good, e := g.IsWellSorted()
			if e != nil {
				return e
			}
			if !good {
				return diagnostic.DebugAssertionFailure(debug, "src/tactic/bv/dt2bv_tactic.cpp", 177, "g->is_well_sorted()")
			}
		}
	}
	return nil
}
func (t *DT2BVTactic) convert(g *tactic.Goal, result *tactic.TacticResult, s *dt2bvInvocation, size uint32, proofs bool) (err error) {
	m := t.manager
	s.extension, err = tactic.NewExtensionModel(m, t.services)
	if err != nil {
		return err
	}
	if err = s.extension.IncRef(); err != nil {
		return err
	}
	s.filter, err = tactic.NewFilterModel(m)
	if err != nil {
		return err
	}
	if err = s.filter.IncRef(); err != nil {
		return err
	}
	s.rw, err = rewriter.NewEnum2BVRewriter(m, t.context, &t.parameters)
	if err != nil {
		return err
	}
	if err = s.rw.SetSortPredicate(func(sort ast.ASTHandle) (bool, error) { return t.finite[sort], nil }); err != nil {
		return err
	}
	s.current, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	s.proof, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		h, e := g.Form(i)
		if e != nil {
			return e
		}
		if e = s.rw.Rewrite(h, s.current, s.proof); e != nil {
			return e
		}
		if proofs {
			old, e := g.Proof(i)
			if e != nil {
				return e
			}
			step, e := m.RefGet(s.proof)
			if e != nil {
				return e
			}
			pr, e := m.MkProofModusPonens(old, step)
			if e != nil {
				return e
			}
			if e = m.SetRef(s.proof, pr); e != nil {
				return e
			}
		}
		current, e := m.RefGet(s.current)
		if e != nil {
			return e
		}
		pr, e := m.RefGet(s.proof)
		if e != nil {
			return e
		}
		dep, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if e = g.Update(i, current, pr, dep); e != nil {
			return e
		}
	}
	if err = s.rw.FlushSideConstraints(&s.bounds); err != nil {
		return err
	}
	for _, ref := range s.bounds {
		h, e := m.RefGet(ref)
		if e != nil {
			return e
		}
		if e = g.Assert(h); e != nil {
			return e
		}
	}
	for _, bv := range s.rw.EnumToBitVector() {
		if err = s.filter.Insert(bv); err != nil {
			return err
		}
	}
	for decl, def := range s.rw.EnumDefinitions() {
		if err = s.extension.Insert(decl, def); err != nil {
			return err
		}
	}
	s.concat, err = tactic.ConcatModels(m.Budget(), s.filter, s.extension)
	if err != nil {
		return err
	}
	if err = result.SetModelConverter(s.concat); err != nil {
		return err
	}
	s.concat = nil // result owns the published converter reference.
	n, err := s.rw.NumTranslated()
	if err != nil {
		return err
	}
	return tactic.ReportTacticProgress(":fd-num-translated", n)
}

func (t *DT2BVTactic) classify(root ast.ASTHandle, visited *ast.ASTFastMark1) error {
	m := t.manager
	// The source quick walk marks only shared nodes and visits children before
	// their parent, including quantifier patterns and no-patterns.
	enter := func(h ast.ASTHandle) (bool, error) {
		n, err := m.GetRefCount(h)
		if err != nil {
			return false, err
		}
		if n <= 1 {
			return true, nil
		}
		marked, err := visited.IsMarked(h)
		if err != nil || marked {
			return false, err
		}
		return true, visited.Mark(h, true)
	}
	yes, err := enter(root)
	if err != nil || !yes {
		return err
	}
	type frame struct {
		h    ast.ASTHandle
		next uint32
	}
	stack := []frame{{h: root}}
	for len(stack) > 0 {
		f := &stack[len(stack)-1]
		kind, err := m.GetKind(f.h)
		if err != nil {
			return err
		}
		var child ast.ASTHandle
		hasChild := false
		switch kind {
		case ast.ASTApp:
			count, e := m.GetNumArgs(f.h)
			if e != nil {
				return e
			}
			if f.next < count {
				child, err = m.GetArg(f.h, f.next)
				hasChild = true
			}
		case ast.ASTQuantifier:
			np, e := m.GetNumPatterns(f.h)
			if e != nil {
				return e
			}
			nn, e := m.GetNumNoPatterns(f.h)
			if e != nil {
				return e
			}
			if f.next < 1+np+nn {
				hasChild = true
				if f.next == 0 {
					child, err = m.GetQuantifierBody(f.h)
				} else if f.next <= np {
					child, err = m.GetPattern(f.h, f.next-1)
				} else {
					child, err = m.GetNoPattern(f.h, f.next-np-1)
				}
			}
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
		if err != nil {
			return err
		}
		if hasChild {
			f.next++
			yes, err := enter(child)
			if err != nil {
				return err
			}
			if yes {
				stack = append(stack, frame{h: child})
			}
			continue
		}
		h := f.h
		stack = stack[:len(stack)-1]
		if kind == ast.ASTQuantifier {
			continue
		}
		if err = t.classifyNode(h, kind); err != nil {
			return err
		}
	}
	return nil
}
func (t *DT2BVTactic) classifyNode(h ast.ASTHandle, kind ast.ASTKind) error {
	m := t.manager
	sort, err := m.GetSort(h)
	if err != nil {
		return err
	}
	if kind == ast.ASTVar {
		yes, e := t.utility.IsEnumSort(sort)
		if yes {
			t.finite[sort] = true
		}
		return e
	}
	decl, err := m.GetDecl(h)
	if err != nil {
		return err
	}
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return err
	}
	op, err := m.GetDeclKind(decl)
	if err != nil {
		return err
	}
	if family == 0 && (op == ast.ASTOpEq || op == ast.ASTOpDistinct) {
		return nil
	}
	count, err := m.GetNumArgs(h)
	if err != nil {
		return err
	}
	_, recognizer, err := m.DatatypeRecognizerConstructorIndex(decl)
	if err != nil {
		return err
	}
	if recognizer && count > 0 {
		arg, e := m.GetArg(h, 0)
		if e != nil {
			return e
		}
		s, e := m.GetSort(arg)
		if e != nil {
			return e
		}
		yes, e := t.utility.IsEnumSort(s)
		if e != nil {
			return e
		}
		if yes {
			t.finite[s] = true
			return nil
		}
	}
	yes, err := t.utility.IsEnumSort(sort)
	if err != nil {
		return err
	}
	if yes && count == 0 {
		t.finite[sort] = true
		return nil
	}
	if yes {
		t.nonFinite[sort] = true
	}
	for i := uint32(0); i < count; i++ {
		a, e := m.GetArg(h, i)
		if e != nil {
			return e
		}
		s, e := m.GetSort(a)
		if e != nil {
			return e
		}
		yes, e := t.utility.IsEnumSort(s)
		if e != nil {
			return e
		}
		if yes {
			t.nonFinite[s] = true
		}
	}
	return nil
}
