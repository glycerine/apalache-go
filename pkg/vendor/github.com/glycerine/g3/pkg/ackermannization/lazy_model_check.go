package ackermannization

// Source: src/ackermannization/lackr_model_constructor.cpp/.h, retained check path.

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type CongruenceConflict struct{ First, Second ast.ASTHandle }
type lazyValue struct{ value, source ast.ASTHandle }

// LazyModelChecker checks the abstract model used by lackr::lazy. The manager
// and evaluator services are borrowed. Info is counted. A nonnil owner returned
// with a construction error must be closed.
type LazyModelChecker struct {
	self        *LazyModelChecker
	manager     *ast.ASTManager
	info        *Info
	infoCleanup bool
	services    model.ModelEvaluatorConfig
	current     *lazyModelCheck
	conflicts   []CongruenceConflict
	state       uint8 // Internal UNKNOWN=0, CHECKED=1, CONFLICT=2.
	trace       *diagnostic.TraceTags
	header      uint64
	closed      bool
}
type lazyModelCheck struct {
	owner     *LazyModelChecker
	abstract  tactic.ModelRef
	empty     tactic.ModelRef
	evaluator *model.ModelEvaluator
	values    map[ast.ASTHandle]lazyValue
	apps      map[ast.ASTHandle]ast.ASTHandle
	visited   map[ast.ASTHandle]bool
	held      []ast.ASTHandle
	scratch   []ast.ASTHandle
	result    ast.ASTRef
	header    uint64
}

func NewLazyModelChecker(m *ast.ASTManager, info *Info, services model.ModelEvaluatorConfig) (*LazyModelChecker, error) {
	if m == nil || info == nil || info.manager != m || services.Context == nil || services.Globals == nil || services.Memory == nil || services.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := info.Check(); err != nil {
		return nil, err
	}
	if err := services.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(LazyModelChecker{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	c := &LazyModelChecker{manager: m, services: services, header: n}
	c.self = c
	if err := info.IncRef(); err != nil {
		return c, err
	}
	c.info = info
	return c, nil
}
func (c *LazyModelChecker) open() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return arena.ErrClosed
	}
	if c.info == nil {
		return ast.ErrASTPending
	}
	return c.info.Check()
}
func (c *LazyModelChecker) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if c.current != nil {
		if err := c.current.close(); err != nil {
			return err
		}
		c.current = nil
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
	c.conflicts = nil
	c.manager.Budget().Release(c.header)
	c.header = 0
	c.closed = true
	return nil
}
func (c *LazyModelChecker) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.open(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}
func (c *LazyModelChecker) Conflicts() ([]CongruenceConflict, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if ast.ASTSourceDebug && c.state != 2 {
		return nil, ast.ErrASTDomain
	}
	return append([]CongruenceConflict(nil), c.conflicts...), nil
}
func (c *LazyModelChecker) Check(md *model.ModelCore) (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if md == nil || md.Manager() != c.manager {
		return false, ast.ErrASTHandle
	}
	c.conflicts = nil
	c.state = 0
	if c.current != nil {
		if err := c.current.close(); err != nil {
			return false, err
		}
		c.current = nil
	}
	n := uint64(reflect.TypeOf(lazyModelCheck{}).Size())
	if err := c.manager.Budget().Reserve(n); err != nil {
		return false, err
	}
	r := &lazyModelCheck{owner: c, header: n, values: make(map[ast.ASTHandle]lazyValue), apps: make(map[ast.ASTHandle]ast.ASTHandle), visited: make(map[ast.ASTHandle]bool)}
	c.current = r
	if err := r.abstract.Set(md); err != nil {
		return false, err
	}
	empty, err := model.NewModel(c.manager)
	if err != nil {
		return false, err
	}
	if err = r.empty.Set(empty.Core()); err != nil {
		return false, errors.Join(err, empty.Close())
	}
	r.result, err = c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	ok := true
	for i := uint32(0); ; i++ {
		n, err := md.NumConstants()
		if err != nil {
			return false, err
		}
		if i >= n {
			break
		}
		d, err := md.Constant(i)
		if err != nil {
			return false, err
		}
		term, err := c.info.FindTerm(d)
		if err != nil {
			return false, err
		}
		if term.IsNull() {
			term, err = c.manager.MkApp(d, nil)
			if err != nil {
				return false, err
			}
		}
		checked, err := r.checkTerm(term)
		if err != nil {
			return false, err
		}
		if !checked {
			ok = false
		}
	}
	c.state = 2
	if ok {
		c.state = 1
	}
	return ok, nil
}
func (r *lazyModelCheck) hold(h ast.ASTHandle, scratch bool) error {
	if err := r.owner.manager.IncRef(h); err != nil {
		return err
	}
	if scratch {
		r.scratch = append(r.scratch, h)
	} else {
		r.held = append(r.held, h)
	}
	return nil
}
func (r *lazyModelCheck) release(roots *[]ast.ASTHandle, limit int) error {
	for len(*roots) > limit {
		i := len(*roots) - 1
		consumed, err := r.owner.manager.DecRefConsumed((*roots)[i])
		if consumed {
			*roots = (*roots)[:i]
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func (r *lazyModelCheck) close() error {
	if r.evaluator != nil {
		if err := r.evaluator.Close(); err != nil {
			return err
		}
		r.evaluator = nil
	}
	if err := r.release(&r.scratch, 0); err != nil {
		return err
	}
	if r.result != (ast.ASTRef{}) {
		if err := r.owner.manager.DelRef(&r.result); err != nil {
			return err
		}
		r.result = ast.ASTRef{}
	}
	if err := r.release(&r.held, 0); err != nil {
		return err
	}
	if err := r.empty.Close(); err != nil {
		return err
	}
	if err := r.abstract.Close(); err != nil {
		return err
	}
	r.owner.manager.Budget().Release(r.header)
	r.header = 0
	return nil
}
func (r *lazyModelCheck) checkTerm(term ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	if r.evaluator == nil {
		empty, err := r.empty.Get()
		if err != nil {
			return false, err
		}
		r.evaluator, err = model.NewModelEvaluator(empty, r.owner.services)
		if err != nil {
			return false, err
		}
	}
	stack := []ast.ASTHandle{term}
	for len(stack) > 0 {
		a := stack[len(stack)-1]
		if r.visited[a] {
			stack = stack[:len(stack)-1]
			continue
		}
		kind, err := m.GetKind(a)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			if ast.ASTSourceDebug {
				return false, ast.ErrASTDomain
			}
			return false, nil
		}
		n, err := m.GetNumArgs(a)
		if err != nil {
			return false, err
		}
		ready := true
		for i := uint32(0); i < n; i++ {
			arg, err := m.GetArg(a, i)
			if err != nil {
				return false, err
			}
			if !r.visited[arg] {
				stack = append(stack, arg)
				ready = false
			}
		}
		if !ready {
			continue
		}
		r.visited[a] = true
		stack = stack[:len(stack)-1]
		ok, err := r.mkValue(a)
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}
func (r *lazyModelCheck) trace(function string, line int, label string, terms ...ast.ASTHandle) error {
	c := r.owner
	return emitAck(c.trace, "model_constructor", function, "src/ackermannization/lackr_model_constructor.cpp", line, func(w io.Writer) error {
		if _, err := io.WriteString(w, label); err != nil {
			return err
		}
		for i, h := range terms {
			if i != 0 {
				separator := "\n->"
				if label == "eval fail\n" {
					separator = ""
				} else if strings.HasPrefix(label, "arg val ") {
					separator = " : "
				}
				if _, err := io.WriteString(w, separator); err != nil {
					return err
				}
			}
			if err := writeAckExpr(c.manager, c.services.Context, c.services.Globals, w, h); err != nil {
				return err
			}
		}
		suffix := ")\n"
		if label == "eval fail\n" || label == "fail arg val(\n" || strings.HasPrefix(label, "arg val ") {
			suffix = "\n"
		}
		_, err := io.WriteString(w, suffix)
		return err
	})
}
func (r *lazyModelCheck) mkValue(a ast.ASTHandle) (ok bool, err error) {
	m := r.owner.manager
	value, err := m.IsValue(a)
	if err != nil || value {
		return value, err
	}
	if err = r.trace("mk_value", 255, "mk_value(\n", a); err != nil {
		return false, err
	}
	if ast.ASTSourceDebug {
		if _, found := r.apps[a]; found {
			return false, ast.ErrASTDomain
		}
	}
	start := len(r.scratch)
	defer func() { err = errors.Join(err, r.release(&r.scratch, start)) }()
	ok, err = r.evaluate(a)
	if err != nil || !ok {
		return ok, err
	}
	v, err := m.RefGet(r.result)
	if err != nil {
		return false, err
	}
	value, err = m.IsValue(v)
	if err != nil {
		return false, err
	}
	if ast.ASTSourceDebug && !value {
		return false, ast.ErrASTDomain
	}
	if err = r.trace("mk_value", 260, "map term(\n", a, v); err != nil {
		return false, err
	}
	if !value {
		if err = r.trace("mk_value", 263, "eval fail\n", a, v); err != nil {
			return false, err
		}
	}
	if err = r.hold(a, false); err != nil {
		return false, err
	}
	if err = r.hold(v, false); err != nil {
		return false, err
	}
	r.apps[a] = v
	return true, nil
}
func (r *lazyModelCheck) constantValue(d ast.ASTHandle) (ast.ASTHandle, error) {
	md, err := r.abstract.Get()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	v, err := md.ConstantInterpretation(d)
	if err != nil || !v.IsNull() {
		return v, err
	}
	sort, err := r.owner.manager.GetRange(d)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return md.GetSomeValue(sort)
}
func (r *lazyModelCheck) evaluate(a ast.ASTHandle) (bool, error) {
	m := r.owner.manager
	d, err := m.GetDecl(a)
	if err != nil {
		return false, err
	}
	n, err := m.GetNumArgs(a)
	if err != nil {
		return false, err
	}
	if n == 0 {
		v, err := r.constantValue(d)
		if err != nil {
			return false, err
		}
		return true, m.SetRef(r.result, v)
	}
	values := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		arg, err := m.GetArg(a, i)
		if err != nil {
			return false, err
		}
		isValue, err := m.IsValue(arg)
		if err != nil {
			return false, err
		}
		v, found := r.apps[arg]
		if isValue {
			v, found = arg, true
		}
		if !found {
			if len(r.owner.conflicts) == 0 {
				if err = r.trace("evaluate", 226, "fail arg val(\n", arg); err != nil {
					return false, err
				}
			}
			return false, nil
		}
		if err = r.trace("evaluate", 231, fmt.Sprintf("arg val %d(\n", i), arg, v); err != nil {
			return false, err
		}
		values[i] = v
	}
	term, err := m.MkApp(d, values)
	if err != nil {
		return false, err
	}
	if err = r.hold(term, true); err != nil {
		return false, err
	}
	yes, err := ShouldAckermannize(m, a)
	if err != nil {
		return false, err
	}
	if !yes {
		if err = r.evaluator.Evaluate(term, r.result); err != nil {
			return false, err
		}
		v, err := m.RefGet(r.result)
		if err != nil {
			return false, err
		}
		return true, r.trace("make_value_interpreted_function", 338, "eval(\n", term, v)
	}
	abstract, err := r.owner.info.Abstraction(a)
	if err != nil {
		return false, err
	}
	ad, err := m.GetDecl(abstract)
	if err != nil {
		return false, err
	}
	v, err := r.constantValue(ad)
	if err != nil {
		return false, err
	}
	if previous, found := r.values[term]; found {
		ok := previous.value == v
		if !ok {
			if err = r.trace("make_value_uninterpreted_function", 308, "already mapped by(\n", previous.source, previous.value); err != nil {
				return false, err
			}
			r.owner.conflicts = append(r.owner.conflicts, CongruenceConflict{a, previous.source})
		}
		return ok, m.SetRef(r.result, previous.value)
	}
	if err = m.SetRef(r.result, v); err != nil {
		return false, err
	}
	for _, h := range []ast.ASTHandle{a, v, term} {
		if err = r.hold(h, false); err != nil {
			return false, err
		}
	}
	r.values[term] = lazyValue{v, a}
	return true, nil
}
