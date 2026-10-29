package qe

// Source: src/qe/qe_arith.cpp/.h arith_project_plugin.

import (
	"errors"
	"io"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/insdict"
)

// ArithmeticProjection linearizes model-selected arithmetic constraints for
// the existing concrete simplex.ModelBasedOpt algorithm. Config and manager
// are borrowed. Mutable variable/formula vectors remain caller-owned.
type ArithmeticProjection struct {
	self    *ArithmeticProjection
	manager *ast.ASTManager
	config  Config
	family  ast.FamilyID
	pending []*arithmeticProjectionCall
	bytes   uint64
	closed  bool
}
type arithmeticProjectionCall struct {
	owner     *ArithmeticProjection
	refs      []ast.ASTRef
	vectors   []*[]ast.ASTRef
	evaluator *model.ModelEvaluator
	mark      *ast.ASTExprMark
	mbo       *simplex.ModelBasedOpt
	ids       *insdict.Dict[ast.ASTHandle, uint32]
	values    map[ast.ASTHandle]ast.ASTHandle
	failures  []memory.ConstructionFailure
}
type ArithmeticProjectionFailure struct {
	owner          *ArithmeticProjection
	cause, cleanup error
}

func (e *ArithmeticProjectionFailure) Error() string        { return e.Unwrap().Error() }
func (e *ArithmeticProjectionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *ArithmeticProjectionFailure) OriginalCause() error { return e.cause }
func (e *ArithmeticProjectionFailure) CleanupCause() error  { return e.cleanup }
func (e *ArithmeticProjectionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *ArithmeticProjectionFailure) Cleanup() error { return e.owner.drain() }

func NewArithmeticProjection(m *ast.ASTManager, config Config) (*ArithmeticProjection, error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err := config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(ArithmeticProjection{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	p := &ArithmeticProjection{manager: m, config: config, family: family, bytes: bytes}
	p.self = p
	return p, nil
}
func (p *ArithmeticProjection) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	_, err := p.manager.ProofsEnabled()
	return err
}
func (p *ArithmeticProjection) FamilyID() (ast.FamilyID, error) {
	if err := p.open(); err != nil {
		return ast.NullFamilyID, err
	}
	return p.family, nil
}
func projectionHandleHash(h ast.ASTHandle) uint64 {
	return h.Origin() ^ uint64(h.Slot())*0x9e3779b97f4a7c15 ^ uint64(h.Generation())*0xbf58476d1ce4e5b9
}
func projectionTerms() *insdict.Dict[ast.ASTHandle, *big.Rat] {
	return insdict.NewDictFunc[ast.ASTHandle, *big.Rat](projectionHandleHash)
}
func (p *ArithmeticProjection) begin(md *model.ModelCore) (*arithmeticProjectionCall, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if md == nil || md.Manager() != p.manager {
		return nil, ast.ErrASTHandle
	}
	if err := p.drain(); err != nil {
		return nil, err
	}
	c := &arithmeticProjectionCall{owner: p, mbo: simplex.NewModelBasedOpt(), ids: insdict.NewDictFunc[ast.ASTHandle, uint32](projectionHandleHash), values: make(map[ast.ASTHandle]ast.ASTHandle)}
	p.pending = append(p.pending, c)
	return c, nil
}
func (c *arithmeticProjectionCall) startEvaluator(md *model.ModelCore) error {
	p := c.owner
	var err error
	c.evaluator, err = model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace})
	return err
}
func (c *arithmeticProjectionCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return h, err
}
func (c *arithmeticProjectionCall) retain(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		c.failures = append(c.failures, failure)
	}
}
func (c *arithmeticProjectionCall) close() error {
	for len(c.failures) != 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	if c.evaluator != nil {
		if err := c.evaluator.Close(); err != nil {
			c.retain(err)
			return err
		}
		c.evaluator = nil
	}
	if c.mark != nil {
		if err := c.mark.Close(); err != nil {
			c.retain(err)
			return err
		}
		c.mark = nil
	}
	for len(c.vectors) != 0 {
		i := len(c.vectors) - 1
		if err := c.owner.shrink(c.vectors[i], 0); err != nil {
			c.retain(err)
			return err
		}
		c.vectors = c.vectors[:i]
	}
	for len(c.refs) != 0 {
		i := len(c.refs) - 1
		if c.refs[i] != (ast.ASTRef{}) {
			if err := c.owner.manager.DelRef(&c.refs[i]); err != nil {
				c.retain(err)
				return err
			}
		}
		c.refs = c.refs[:i]
	}
	c.mbo = nil
	c.ids = nil
	return nil
}
func (p *ArithmeticProjection) drain() error {
	for len(p.pending) != 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *ArithmeticProjection) finish(c *arithmeticProjectionCall, err *error) {
	c.retain(*err)
	if cleanup := p.drain(); cleanup != nil {
		*err = &ArithmeticProjectionFailure{owner: p, cause: *err, cleanup: cleanup}
	}
}
func (p *ArithmeticProjection) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if err := p.drain(); err != nil {
		return err
	}
	p.manager.Budget().Release(p.bytes)
	p.bytes = 0
	p.closed = true
	return nil
}
func (p *ArithmeticProjection) emit(function string, line int, body func(io.Writer) error) error {
	tags := p.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe", function, "src/qe/qe_arith.cpp", line, func() bool { return true }, func(w io.Writer) error { return body(traceBodyWriter{w}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (p *ArithmeticProjection) arithmetic(h ast.ASTHandle) (integer, arith bool, err error) {
	s, err := p.manager.GetSort(h)
	if err != nil {
		return false, false, err
	}
	fid, err := p.manager.GetFamilyID(s)
	if err != nil || fid != p.family {
		return false, false, err
	}
	k, err := p.manager.GetDeclKind(s)
	return k == 1, k == 0 || k == 1, err
}
func (p *ArithmeticProjection) operation(h ast.ASTHandle, fid ast.FamilyID, k int32) ([]ast.ASTHandle, bool, error) {
	kind, err := p.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return nil, false, err
	}
	f, err := p.manager.GetFamilyID(h)
	if err != nil || f != fid {
		return nil, false, err
	}
	op, err := p.manager.GetDeclKind(h)
	if err != nil || op != k {
		return nil, false, err
	}
	n, err := p.manager.GetNumArgs(h)
	if err != nil {
		return nil, false, err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		args[i], err = p.manager.GetArg(h, uint32(i))
		if err != nil {
			return nil, false, err
		}
	}
	return args, true, nil
}
func (p *ArithmeticProjection) appendRef(v *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := p.manager.NewRef(h)
	if err == nil {
		*v = append(*v, r)
	}
	return err
}
func (p *ArithmeticProjection) shrink(v *[]ast.ASTRef, n int) error {
	for len(*v) > n {
		i := len(*v) - 1
		if (*v)[i] != (ast.ASTRef{}) {
			if err := p.manager.DelRef(&(*v)[i]); err != nil {
				return err
			}
		}
		*v = (*v)[:i]
	}
	return nil
}
func (c *arithmeticProjectionCall) eval(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.owner.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, r)
	if err = c.evaluator.Evaluate(h, r); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.owner.manager.RefGet(r)
}
func (p *ArithmeticProjection) numeral(h ast.ASTHandle) (*big.Rat, bool, error) {
	s, _, ok, err := p.manager.ArithmeticNumeralText(h)
	if err != nil || !ok {
		return nil, false, err
	}
	q, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return q, true, nil
}

// Source's syntactic numeral recognizer includes products, sums, unary minus
// and binary subtraction; division and coercions remain ordinary terms.
func (p *ArithmeticProjection) numeralExpression(h ast.ASTHandle) (*big.Rat, bool, error) {
	q, ok, err := p.numeral(h)
	if err != nil || ok {
		return q, ok, err
	}
	for _, op := range []int32{8, 9, 6, 7} {
		args, matched, err := p.operation(h, p.family, op)
		if err != nil {
			return nil, false, err
		}
		if !matched {
			continue
		}
		if (op == 8 && len(args) != 1) || (op == 7 && len(args) != 2) {
			return nil, false, nil
		}
		q = new(big.Rat)
		if op == 9 {
			q.SetInt64(1)
		}
		for i, arg := range args {
			v, ok, err := p.numeralExpression(arg)
			if err != nil || !ok {
				return nil, false, err
			}
			switch op {
			case 8:
				q.Neg(v)
			case 9:
				q.Mul(q, v)
			case 6:
				q.Add(q, v)
			case 7:
				if i == 0 {
					q.Set(v)
				} else {
					q.Sub(q, v)
				}
			}
		}
		return q, true, nil
	}
	return nil, false, nil
}
func (c *arithmeticProjectionCall) id(h ast.ASTHandle) (uint32, error) {
	if id, ok := c.ids.Get2(h); ok {
		return id, nil
	}
	v, err := c.eval(h)
	if err != nil {
		return 0, err
	}
	q, ok, err := c.owner.numeral(v)
	if err != nil {
		return 0, err
	}
	if !ok {
		q = new(big.Rat)
	} // source leaves the default rational at zero
	integer, _, err := c.owner.arithmetic(h)
	if err != nil {
		return 0, err
	}
	if _, err = c.hold(h, nil); err != nil {
		return 0, err
	}
	id := c.mbo.AddVariable(q, integer)
	c.ids.Set(h, id)
	c.values[h] = v
	return id, nil
}
func (c *arithmeticProjectionCall) coefficients(terms *insdict.Dict[ast.ASTHandle, *big.Rat]) ([]simplex.LinearTerm, error) {
	if err := c.evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	var result []simplex.LinearTerm
	for h, q := range terms.All() {
		id, err := c.id(h)
		if err != nil {
			return nil, err
		}
		if q.Sign() != 0 {
			result = append(result, simplex.NewLinearTerm(id, q))
		} else {
			if err := c.owner.emit("extract_coefficients", 520, func(w io.Writer) error {
				if e := writeExpression(c.owner.manager, c.owner.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " has coefficeint 0\n")
				return nil
			}); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// Solve is deliberately the source cheap-solve no-op. Projection is performed
// by Project/ProjectVariable, rather than silently extending this hook.
func (p *ArithmeticProjection) Solve(md *model.ModelCore, vars, lits *[]ast.ASTRef) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if md == nil || md.Manager() != p.manager || vars == nil || lits == nil {
		return false, ast.ErrASTHandle
	}
	return false, nil
}
