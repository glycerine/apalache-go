package qe

// Source: src/qe/qsat.h max_level/pred_abs; src/qe/qsat.cpp pred_abs.

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

// NoQuantifierLevel is the source UINT_MAX sentinel, below every actual level.
const NoQuantifierLevel = ^uint32(0)

type MaxLevel struct{ Existential, Universal uint32 }

func EmptyMaxLevel() MaxLevel { return MaxLevel{NoQuantifierLevel, NoQuantifierLevel} }
func maxQuantifierLevel(a, b uint32) uint32 {
	if a == NoQuantifierLevel {
		return b
	}
	if b == NoQuantifierLevel || a > b {
		return a
	}
	return b
}
func (l MaxLevel) Max() uint32 { return maxQuantifierLevel(l.Existential, l.Universal) }
func (l *MaxLevel) Merge(other MaxLevel) {
	l.Existential = maxQuantifierLevel(l.Existential, other.Existential)
	l.Universal = maxQuantifierLevel(l.Universal, other.Universal)
}
func (l MaxLevel) Display(out io.Writer) error {
	if out == nil {
		return ast.ErrASTHandle
	}
	if l.Existential != NoQuantifierLevel {
		if _, err := fmt.Fprintf(out, "e%d ", l.Existential); err != nil {
			return err
		}
	}
	if l.Universal != NoQuantifierLevel {
		_, err := fmt.Fprintf(out, "a%d ", l.Universal)
		return err
	}
	return nil
}

// PredicateAbstraction owns the predicates and scoped assumptions used by QSAT.
// Maps memoize identities; they have no iteration-order contract in the source.
// Config and the AST manager are borrowed. Results are caller-owned ASTRefs.
type PredicateAbstraction struct {
	self                                   *PredicateAbstraction
	manager                                *ast.ASTManager
	config                                 Config
	predicates                             [][]ast.ASTRef
	assumptions, trail                     []ast.ASTRef
	limits                                 []int
	pred2lit, lit2pred, asm2pred, pred2asm map[ast.ASTHandle]ast.ASTHandle
	predKeys, litKeys, asmKeys, defKeys    []ast.ASTRef
	exprLevels, declLevels                 map[ast.ASTHandle]MaxLevel
	filter                                 *tactic.FilterModelConverter
	filterPending                          bool
	pending                                []*predicateCall
	bytes                                  uint64
	closed                                 bool
}

type predicateCall struct {
	owner     *PredicateAbstraction
	refs      []ast.ASTRef
	evaluator *model.ModelEvaluator
	failures  []memory.ConstructionFailure
}
type PredicateAbstractionFailure struct {
	owner          *PredicateAbstraction
	cause, cleanup error
	constructor    bool
}

func (e *PredicateAbstractionFailure) Error() string        { return e.Unwrap().Error() }
func (e *PredicateAbstractionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *PredicateAbstractionFailure) OriginalCause() error { return e.cause }
func (e *PredicateAbstractionFailure) CleanupCause() error  { return e.cleanup }
func (e *PredicateAbstractionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *PredicateAbstractionFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}

func NewPredicateAbstraction(m *ast.ASTManager, config Config) (result *PredicateAbstraction, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err = m.ProofsEnabled(); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(PredicateAbstraction{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	p := &PredicateAbstraction{manager: m, config: config, bytes: bytes,
		pred2lit: make(map[ast.ASTHandle]ast.ASTHandle), lit2pred: make(map[ast.ASTHandle]ast.ASTHandle),
		asm2pred: make(map[ast.ASTHandle]ast.ASTHandle), pred2asm: make(map[ast.ASTHandle]ast.ASTHandle),
		exprLevels: make(map[ast.ASTHandle]MaxLevel), declLevels: make(map[ast.ASTHandle]MaxLevel)}
	p.self = p
	defer func() {
		if err != nil {
			if cleanup := p.Close(); cleanup != nil {
				result = p
				err = &PredicateAbstractionFailure{owner: p, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	p.filter, err = tactic.NewFilterModel(m)
	if err != nil {
		return nil, err
	}
	if err = p.filter.IncRef(); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *PredicateAbstraction) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	_, err := p.manager.ProofsEnabled()
	return err
}
func (p *PredicateAbstraction) appendRef(dst *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := p.manager.NewRef(h)
	if err == nil {
		*dst = append(*dst, r)
	}
	return err
}
func (p *PredicateAbstraction) closeRefs(refs *[]ast.ASTRef) error {
	for len(*refs) != 0 {
		i := len(*refs) - 1
		if (*refs)[i] != (ast.ASTRef{}) {
			if err := p.manager.DelRef(&(*refs)[i]); err != nil {
				return err
			}
		}
		*refs = (*refs)[:i]
	}
	return nil
}
func (c *predicateCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return h, c.owner.appendRef(&c.refs, h)
}
func (c *predicateCall) close() error {
	for len(c.failures) != 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	if c.evaluator != nil {
		if err := c.evaluator.Close(); err != nil {
			return err
		}
		c.evaluator = nil
	}
	return c.owner.closeRefs(&c.refs)
}
func (p *PredicateAbstraction) drain() error {
	for len(p.pending) != 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *PredicateAbstraction) begin() (*predicateCall, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if err := p.drain(); err != nil {
		return nil, err
	}
	c := &predicateCall{owner: p}
	p.pending = append(p.pending, c)
	return c, nil
}
func (p *PredicateAbstraction) finish(c *predicateCall, err *error) {
	var failure memory.ConstructionFailure
	if errors.As(*err, &failure) {
		c.failures = append(c.failures, failure)
	}
	if cleanup := p.drain(); cleanup != nil {
		*err = &PredicateAbstractionFailure{owner: p, cause: *err, cleanup: cleanup}
	}
}
func (p *PredicateAbstraction) FilterModelConverter() (*tactic.FilterModelConverter, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return p.filter, nil
}

func (p *PredicateAbstraction) emit(function string, line int, body func(io.Writer) error) error {
	tags := p.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe", function, "src/qe/qsat.cpp", line, func() bool { return true }, func(out io.Writer) error { return body(traceBodyWriter{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (p *PredicateAbstraction) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.drain(); err != nil {
		return err
	}
	for _, refs := range []*[]ast.ASTRef{&p.trail, &p.predKeys, &p.litKeys, &p.asmKeys, &p.defKeys, &p.assumptions} {
		if err := p.closeRefs(refs); err != nil {
			return err
		}
	}
	for i := range p.predicates {
		if err := p.closeRefs(&p.predicates[i]); err != nil {
			return err
		}
	}
	clear(p.pred2lit)
	clear(p.lit2pred)
	clear(p.asm2pred)
	clear(p.pred2asm)
	clear(p.exprLevels)
	p.predicates = nil
	p.limits = nil
	// Source reset intentionally preserves declaration levels and the filter.
	return nil
}
func (p *PredicateAbstraction) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if err := p.Reset(); err != nil {
		return err
	}
	if p.filter != nil {
		if p.filterPending {
			if err := p.filter.Close(); err != nil {
				return err
			}
		} else {
			consumed, err := p.filter.DecRefConsumed()
			if consumed && err != nil {
				p.filterPending = true
			}
			if err != nil {
				return err
			}
		}
		p.filter = nil
	}
	clear(p.declLevels)
	p.manager.Budget().Release(p.bytes)
	p.bytes = 0
	p.closed = true
	return nil
}

func (p *PredicateAbstraction) SetExpressionLevel(h ast.ASTHandle, level MaxLevel) error {
	if err := p.open(); err != nil {
		return err
	}
	if _, err := p.manager.GetDecl(h); err != nil {
		return err
	}
	p.exprLevels[h] = level
	return nil
}
func (p *PredicateAbstraction) SetDeclarationLevel(h ast.ASTHandle, level MaxLevel) error {
	if err := p.open(); err != nil {
		return err
	}
	if _, err := p.manager.GetArity(h); err != nil {
		return err
	}
	p.declLevels[h] = level
	return nil
}
func (p *PredicateAbstraction) ComputeLevel(h ast.ASTHandle) (MaxLevel, error) {
	if err := p.open(); err != nil {
		return EmptyMaxLevel(), err
	}
	return p.computeLevel(h)
}
func (p *PredicateAbstraction) computeLevel(h ast.ASTHandle) (MaxLevel, error) {
	todo := []ast.ASTHandle{h}
	for len(todo) != 0 {
		a := todo[len(todo)-1]
		if _, ok := p.exprLevels[a]; ok {
			todo = todo[:len(todo)-1]
			continue
		}
		d, err := p.manager.GetDecl(a)
		if err != nil {
			return EmptyMaxLevel(), err
		}
		level := EmptyMaxLevel()
		if l, ok := p.declLevels[d]; ok {
			level.Merge(l)
		}
		n, err := p.manager.GetNumArgs(a)
		if err != nil {
			return EmptyMaxLevel(), err
		}
		found := false
		for i := uint32(0); i < n; i++ {
			arg, err := p.manager.GetArg(a, i)
			if err != nil {
				return EmptyMaxLevel(), err
			}
			if l, ok := p.exprLevels[arg]; ok {
				level.Merge(l)
			} else {
				todo = append(todo, arg)
				found = true
			}
		}
		if !found {
			p.exprLevels[a] = level
			todo = todo[:len(todo)-1]
		}
	}
	return p.exprLevels[h], nil
}
func (p *PredicateAbstraction) addPredicate(a, lit ast.ASTHandle) error {
	if err := p.appendRef(&p.predKeys, a); err != nil {
		return err
	}
	p.pred2lit[a] = lit
	if _, ok := p.lit2pred[lit]; !ok {
		if err := p.appendRef(&p.litKeys, lit); err != nil {
			return err
		}
		p.lit2pred[lit] = a
	}
	return nil
}
func (p *PredicateAbstraction) AddPredicate(a, lit ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if _, err := p.manager.GetDecl(a); err != nil {
		return err
	}
	if _, err := p.manager.GetDecl(lit); err != nil {
		return err
	}
	return p.addPredicate(a, lit)
}
func (p *PredicateAbstraction) Insert(a ast.ASTHandle, level MaxLevel) error {
	if err := p.open(); err != nil {
		return err
	}
	if _, err := p.manager.GetDecl(a); err != nil {
		return err
	}
	l := level.Max()
	if l == NoQuantifierLevel {
		l = 0
	}
	for uint64(len(p.predicates)) <= uint64(l) {
		p.predicates = append(p.predicates, nil)
	}
	return p.appendRef(&p.predicates[l], a)
}
func (p *PredicateAbstraction) fresh(c *predicateCall, name string) (ast.ASTHandle, error) {
	sort, err := p.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := c.hold(p.manager.MkFreshConst(name, sort))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d, err := p.manager.GetDecl(r)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r, p.filter.Insert(d)
}
func (p *PredicateAbstraction) FreshBoolean(name string, out ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	r, err := p.fresh(c, name)
	if err != nil {
		return err
	}
	return p.manager.SetRef(out, r)
}

func (p *PredicateAbstraction) AbstractAtoms(formula ast.ASTHandle, level *MaxLevel, defs *[]ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if level == nil || defs == nil {
		return ast.ErrASTHandle
	}
	return p.abstractAtoms(c, formula, level, defs)
}
func (p *PredicateAbstraction) abstractAtoms(c *predicateCall, formula ast.ASTHandle, level *MaxLevel, defs *[]ast.ASTRef) error {
	if err := p.appendRef(&p.trail, formula); err != nil {
		return err
	}
	seen := make(map[ast.ASTHandle]bool)
	todo := []ast.ASTHandle{formula}
	for len(todo) != 0 {
		a := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[a] {
			continue
		}
		seen[a] = true
		d, err := p.manager.GetDecl(a)
		if err != nil {
			return err
		}
		if pred, ok := p.lit2pred[a]; ok {
			if err := p.emit("abstract_atoms", 239, func(out io.Writer) error {
				if e := writeExpression(p.manager, p.config, out, a); e != nil {
					return e
				}
				_, _ = io.WriteString(out, " ")
				if e := writeExpression(p.manager, p.config, out, pred); e != nil {
					return e
				}
				_, _ = io.WriteString(out, "\n")
				return nil
			}); err != nil {
				return err
			}
			l, ok := p.exprLevels[pred]
			if !ok {
				return ast.ErrASTDomain
			}
			level.Merge(l)
			continue
		}
		n, err := p.manager.GetNumArgs(a)
		if err != nil {
			return err
		}
		fid, err := p.manager.GetFamilyID(d)
		if err != nil {
			return err
		}
		isBool, err := p.boolean(a)
		if err != nil {
			return err
		}
		if fid == ast.NullFamilyID && n == 0 && isBool {
			l, ok := p.exprLevels[a]
			if !ok {
				return ast.ErrASTDomain
			}
			level.Merge(l)
			if _, ok := p.pred2lit[a]; !ok {
				if err := p.addPredicate(a, a); err != nil {
					return err
				}
				if err := p.Insert(a, l); err != nil {
					return err
				}
			}
			continue
		}
		var first ast.ASTHandle
		for i := uint32(0); i < n; i++ {
			arg, err := p.manager.GetArg(a, i)
			if err != nil {
				return err
			}
			if i == 0 {
				first = arg
			}
			if !seen[arg] {
				todo = append(todo, arg)
			}
		}
		boolOp := fid == p.manager.BasicFamilyID()
		if boolOp {
			op, err := p.manager.GetDeclKind(a)
			if err != nil {
				return err
			}
			if op == ast.ASTOpEq || op == ast.ASTOpDistinct {
				boolOp, err = p.boolean(first)
				if err != nil {
					return err
				}
			}
		}
		if !boolOp && isBool {
			if err := p.emit("abstract_atoms", 268, func(out io.Writer) error {
				if e := writeExpression(p.manager, p.config, out, a); e != nil {
					return e
				}
				_, _ = io.WriteString(out, "\n")
				return nil
			}); err != nil {
				return err
			}
			r, err := p.fresh(c, "p")
			if err != nil {
				return err
			}
			l, err := p.computeLevel(a)
			if err != nil {
				return err
			}
			if err := p.addPredicate(r, a); err != nil {
				return err
			}
			p.exprLevels[r] = l
			eq, err := c.hold(p.manager.MkEq(r, a))
			if err != nil {
				return err
			}
			if err := p.appendRef(defs, eq); err != nil {
				return err
			}
			declLevel, ok := p.declLevels[d]
			if !ok || !(declLevel.Max() < l.Max()) {
				if err := p.Insert(r, l); err != nil {
					return err
				}
			}
			level.Merge(l)
		}
	}
	return nil
}

func (p *PredicateAbstraction) boolean(h ast.ASTHandle) (bool, error) {
	s, err := p.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	b, err := p.manager.MkBoolSort()
	return s == b, err
}

// substitute uses the source bottom-up simultaneous replacement: replacement
// values are final leaves, including identity entries. It never recurses into
// a replacement, and preserves each unchanged application handle.
func (p *PredicateAbstraction) substitute(c *predicateCall, forms []ast.ASTHandle, replacements map[ast.ASTHandle]ast.ASTHandle) ([]ast.ASTHandle, error) {
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	todo := append([]ast.ASTHandle(nil), forms...)
	for len(todo) != 0 {
		a := todo[len(todo)-1]
		if _, ok := cache[a]; ok {
			todo = todo[:len(todo)-1]
			continue
		}
		d, err := p.manager.GetDecl(a)
		if err != nil {
			return nil, err
		}
		if r, ok := replacements[a]; ok {
			cache[a] = r
			todo = todo[:len(todo)-1]
			continue
		}
		n, err := p.manager.GetNumArgs(a)
		if err != nil {
			return nil, err
		}
		args := make([]ast.ASTHandle, 0, n)
		diff := false
		for i := uint32(0); i < n; i++ {
			arg, err := p.manager.GetArg(a, i)
			if err != nil {
				return nil, err
			}
			if v, ok := cache[arg]; ok {
				args = append(args, v)
				diff = diff || v != arg
			} else {
				todo = append(todo, arg)
			}
		}
		if len(args) == int(n) {
			r := a
			if diff {
				r, err = c.hold(p.manager.MkApp(d, args))
				if err != nil {
					return nil, err
				}
			}
			cache[a] = r
			todo = todo[:len(todo)-1]
		}
	}
	result := make([]ast.ASTHandle, len(forms))
	for i, h := range forms {
		result[i] = cache[h]
	}
	return result, nil
}
func (p *PredicateAbstraction) MakeAbstract(formula ast.ASTHandle, out ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	forms, err := p.substitute(c, []ast.ASTHandle{formula}, p.lit2pred)
	if err != nil {
		return err
	}
	return p.manager.SetRef(out, forms[0])
}
func (p *PredicateAbstraction) PredicatesToLiterals(forms *[]ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if forms == nil {
		return ast.ErrASTHandle
	}
	hs := make([]ast.ASTHandle, len(*forms))
	for i, r := range *forms {
		hs[i], err = p.manager.RefGet(r)
		if err != nil {
			return err
		}
	}
	hs, err = p.substitute(c, hs, p.pred2lit)
	if err != nil {
		return err
	}
	for i, h := range hs {
		if err = p.manager.SetRef((*forms)[i], h); err != nil {
			return err
		}
	}
	return nil
}
func (p *PredicateAbstraction) PredicatesToAssumptions(formula ast.ASTHandle, out ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	hs, err := p.substitute(c, []ast.ASTHandle{formula}, p.pred2asm)
	if err != nil {
		return err
	}
	return p.manager.SetRef(out, hs[0])
}
