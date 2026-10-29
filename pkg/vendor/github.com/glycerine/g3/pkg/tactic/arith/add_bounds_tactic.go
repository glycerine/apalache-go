package arith

// Source: src/tactic/arith/add_bounds_tactic.cpp/.h.

import (
	"errors"
	"io"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// visitArithmeticGoal is the concrete project-expression traversal used by
// arithmetic probes and bound insertion. It visits original goal roots only,
// in postorder, including quantifier bodies, patterns and no-patterns.
func visitArithmeticGoal(g *tactic.Goal, visit func(ast.ASTHandle, ast.ASTKind) (bool, error)) (bool, error) {
	if g == nil {
		return false, ast.ErrASTHandle
	}
	m := g.Manager()
	n, err := g.Size()
	if err != nil {
		return false, err
	}
	type frame struct {
		term ast.ASTHandle
		exit bool
	}
	visited := make(map[ast.ASTHandle]bool)
	for i := uint32(0); i < n; i++ {
		root, e := g.Form(i)
		if e != nil {
			return false, e
		}
		stack := []frame{{term: root}}
		for len(stack) > 0 {
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if !f.exit {
				if visited[f.term] {
					continue
				}
				visited[f.term] = true
			}
			kind, e := m.GetKind(f.term)
			if e != nil {
				return false, e
			}
			if f.exit || kind == ast.ASTVar {
				if stop, e := visit(f.term, kind); e != nil || stop {
					return stop, e
				}
				continue
			}
			stack = append(stack, frame{term: f.term, exit: true})
			switch kind {
			case ast.ASTApp:
				count, e := m.GetNumArgs(f.term)
				if e != nil {
					return false, e
				}
				for j := count; j > 0; j-- {
					child, e := m.GetArg(f.term, j-1)
					if e != nil {
						return false, e
					}
					stack = append(stack, frame{term: child})
				}
			case ast.ASTQuantifier:
				count, e := m.GetNumNoPatterns(f.term)
				if e != nil {
					return false, e
				}
				for j := count; j > 0; j-- {
					child, e := m.GetNoPattern(f.term, j-1)
					if e != nil {
						return false, e
					}
					stack = append(stack, frame{term: child})
				}
				count, e = m.GetNumPatterns(f.term)
				if e != nil {
					return false, e
				}
				for j := count; j > 0; j-- {
					child, e := m.GetPattern(f.term, j-1)
					if e != nil {
						return false, e
					}
					stack = append(stack, frame{term: child})
				}
				child, e := m.GetQuantifierBody(f.term)
				if e != nil {
					return false, e
				}
				stack = append(stack, frame{term: child})
			default:
				return false, ast.ErrASTDomain
			}
		}
	}
	return false, nil
}

func arithmeticConstant(m *ast.ASTManager, h ast.ASTHandle, family ast.FamilyID) (integer, found bool, err error) {
	fid, err := m.GetFamilyID(h)
	if err != nil {
		return false, false, err
	}
	if fid != ast.NullFamilyID {
		return false, false, nil
	}
	n, err := m.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, false, err
	}
	sort, err := m.GetSort(h)
	if err != nil {
		return false, false, err
	}
	fid, err = m.GetFamilyID(sort)
	if err != nil || fid != family {
		return false, false, err
	}
	kind, err := m.GetDeclKind(sort)
	return kind == 1, kind == 0 || kind == 1, err
}

type addBoundsImplementation struct {
	lower, upper big.Rat
	header       uint64
}
type AddBoundsTactic struct {
	self        *AddBoundsTactic
	manager     *ast.ASTManager
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	family      ast.FamilyID
	params      uparams.ParamsRef
	paramsReady bool
	current     *addBoundsImplementation
	numerals    []*unumeral.OwnedRational
	bounds      []*BoundManager
	trace       *diagnostic.TraceTags
	header      uint64
	refs        uint32
	closed      bool
}

var _ tactic.Tactic = (*AddBoundsTactic)(nil)

type AddBoundsConstructionFailure struct {
	owner          *AddBoundsTactic
	cause, cleanup error
}

func (e *AddBoundsConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *AddBoundsConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *AddBoundsConstructionFailure) OriginalCause() error { return e.cause }
func (e *AddBoundsConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *AddBoundsConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.context.Numerals().RuntimeOwner()
}
func (e *AddBoundsConstructionFailure) Cleanup() error { return e.owner.Close() }
func (t *AddBoundsTactic) constructionFailure(cause error) error {
	if cleanup := t.Close(); cleanup != nil {
		return &AddBoundsConstructionFailure{owner: t, cause: cause, cleanup: cleanup}
	}
	return cause
}

func NewAddBoundsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*AddBoundsTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(AddBoundsTactic{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &AddBoundsTactic{manager: m, context: c, globals: g, family: family, header: n}
	t.self = t
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.current, err = t.build()
	}
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return t, nil
}
func MakeAddBoundsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (tactic.Tactic, error) {
	t, err := NewAddBoundsTactic(m, c, g, p)
	if err != nil {
		return nil, err
	}
	w, err := tactic.NewCleanTactic(t)
	if err != nil {
		return nil, t.constructionFailure(err)
	}
	return w, nil
}
func (t *AddBoundsTactic) releaseNumerals(size int) error {
	n := t.context.Numerals()
	for len(t.numerals) > size {
		i := len(t.numerals) - 1
		v := t.numerals[i]
		if *v != (unumeral.OwnedRational{}) {
			if err := n.Del(v); err != nil {
				return err
			}
		}
		t.numerals = t.numerals[:i]
	}
	return nil
}
func (t *AddBoundsTactic) readRational(key string, fallback int64) (value *big.Rat, err error) {
	start := len(t.numerals)
	defer func() { err = errors.Join(err, t.releaseNumerals(start)) }()
	n := t.context.Numerals()
	def := new(unumeral.OwnedRational)
	out := new(unumeral.OwnedRational)
	t.numerals = append(t.numerals, def, out)
	if *def, err = n.NewValue(); err != nil {
		return nil, err
	}
	if err = n.SetInt64(def, fallback); err != nil {
		return nil, err
	}
	if *out, err = t.context.GetRatName(&t.params, &key, def); err != nil {
		return nil, err
	}
	text, err := n.String(out)
	if err != nil {
		return nil, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return value, nil
}
func (t *AddBoundsTactic) policy(r *addBoundsImplementation) error {
	v, err := t.readRational("add_bound_lower", -2)
	if v != nil {
		r.lower.Set(v)
	}
	if err != nil {
		return err
	}
	v, err = t.readRational("add_bound_upper", 2)
	if v != nil {
		r.upper.Set(v)
	}
	return err
}
func (t *AddBoundsTactic) build() (*addBoundsImplementation, error) {
	n := uint64(reflect.TypeOf(addBoundsImplementation{}).Size())
	if err := t.manager.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &addBoundsImplementation{header: n}
	if err := t.policy(r); err != nil {
		t.manager.Budget().Release(n)
		return nil, err
	}
	return r, nil
}
func (t *AddBoundsTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.current == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *AddBoundsTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *AddBoundsTactic) DecRef() error {
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
func (t *AddBoundsTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.policy(t.current)
}
func (t *AddBoundsTactic) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	l := "(default: -2) lower bound to be added to unbounded variables."
	if err := d.InsertName("add_bound_lower", uparams.ParamNumeral, &l, nil, nil); err != nil {
		return err
	}
	l = "(default: 2) upper bound to be added to unbounded variables."
	return d.InsertName("add_bound_upper", uparams.ParamNumeral, &l, nil, nil)
}
func (*AddBoundsTactic) CollectStatistics() map[string]float64    { return nil }
func (t *AddBoundsTactic) ResetStatistics() error                 { return t.open() }
func (t *AddBoundsTactic) Reset() error                           { return t.Cleanup() }
func (t *AddBoundsTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *AddBoundsTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *AddBoundsTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	copy, err := NewAddBoundsTactic(m, t.context, t.globals, &t.params)
	if copy != nil {
		copy.trace = t.trace
	}
	return copy, err
}
func (t *AddBoundsTactic) closeBounds() error {
	for len(t.bounds) > 0 {
		i := len(t.bounds) - 1
		if err := t.bounds[i].Close(); err != nil {
			return err
		}
		t.bounds = t.bounds[:i]
	}
	return nil
}
func (t *AddBoundsTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.releaseNumerals(0); err != nil {
		return err
	}
	if err := t.closeBounds(); err != nil {
		return err
	}
	r, err := t.build()
	if err != nil {
		return err
	}
	old := t.current
	t.current = r
	t.manager.Budget().Release(old.header)
	return nil
}
func (t *AddBoundsTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.releaseNumerals(0); err != nil {
		return err
	}
	if err := t.closeBounds(); err != nil {
		return err
	}
	if t.current != nil {
		t.manager.Budget().Release(t.current.header)
		t.current = nil
	}
	if t.paramsReady {
		if err := t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.closed = true
	return nil
}
func (t *AddBoundsTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *AddBoundsTactic) Apply(g *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if g == nil || result == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("add-bounds", g)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	bm, err := NewBoundManager(t.manager, t.context, t.globals)
	if err != nil {
		return err
	}
	t.bounds = append(t.bounds, bm)
	defer func() { err = errors.Join(err, t.closeBounds()) }()
	// Source deliberately does not call bm.CollectGoal(g). Its empty maps
	// cause both bounds to be added even to already bounded variables.
	var added uint32
	_, err = visitArithmeticGoal(g, func(h ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind != ast.ASTApp {
			return false, nil
		}
		integer, found, e := arithmeticConstant(t.manager, h, t.family)
		if e != nil || !found {
			return false, e
		}
		for _, lower := range []bool{true, false} {
			has, e := bm.HasLower(h)
			value, op := &t.current.upper, int32(2)
			if !lower {
				has, e = bm.HasUpper(h)
				value, op = &t.current.lower, 3
			}
			if e != nil {
				return false, e
			}
			if has {
				continue
			}
			number, e := t.manager.ArithmeticNumeralRat(value, integer)
			if e != nil {
				return false, e
			}
			pin, e := t.manager.NewRef(number)
			if e != nil {
				return false, e
			}
			bound, e := t.manager.MkFamilyApp(t.family, op, nil, []ast.ASTHandle{h, number})
			if e == nil {
				e = g.Assert(bound)
			}
			e = errors.Join(e, t.manager.DelRef(&pin))
			if e != nil {
				return false, e
			}
			added++
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(g); err != nil {
		return err
	}
	if added > 0 {
		if err = g.UpdatePrecision(tactic.GoalUnder); err != nil {
			return err
		}
	}
	if err = tactic.ReportTacticProgress(":added-bounds", added); err != nil {
		return err
	}
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err = tags.EmitConditional("add_bounds", "operator()", "src/tactic/arith/add_bounds_tactic.cpp", 132, func() bool { return true }, func(w io.Writer) error { return g.Display(w) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
