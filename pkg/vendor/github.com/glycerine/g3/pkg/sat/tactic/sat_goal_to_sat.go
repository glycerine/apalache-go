package tactic

// Source: src/sat/tactic/goal2sat.cpp:41-423,488-500;
// src/ast/expr2var.cpp:25-54.

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/insdict"
)

var ErrSATGoalUnsupported = errors.New("operator not supported, apply simplifier before invoking translator")
var ErrSATGoalFailed = errors.New("sat: translator failed; close before reuse")

type satGoalUnsupportedOperator struct{ formula string }

func (e satGoalUnsupportedOperator) Error() string {
	return "operator " + e.formula + " not supported, apply simplifier before invoking translator"
}
func (e satGoalUnsupportedOperator) Unwrap() error { return ErrSATGoalUnsupported }
func (e satGoalUnsupportedOperator) As(target interface{}) bool {
	return errors.As(tactic.NewTacticException(e.Error()), target)
}

type satGoalFrame struct {
	term       ast.ASTHandle
	index      uint32
	root, sign bool
}

// This implements the source array-of-formulas overload, using the actual CDCL
// solver. Goal dependencies, tactic/model converters and public API factories
// remain separate callers. Caller assertions must stay alive through Compile.
type GoalTranslator struct {
	self        *GoalTranslator
	manager     *ast.ASTManager
	sharedAtoms *ast.Expr2Var // Borrowed source atom2bool_var for incremental callers.
	solver      *sat.Solver
	// Borrow the AST manager's original resource limit for translation checkpoints.
	Limit                                     *control.Reslimit
	cooperation                               *cooperate.Cooperation
	traceTags                                 *diagnostic.TraceTags
	printer                                   *uparams.GlobalParamsService
	cache                                     map[ast.ASTHandle]uint32
	Atoms                                     insdict.Dict[ast.ASTHandle, uint32]
	interfaces                                map[ast.ASTHandle]bool
	dependencies                              insdict.Dict[ast.ASTHandle, uint32]
	dependencyTrail                           []ast.ASTHandle
	frames                                    []satGoalFrame
	results                                   []sat.Literal
	interpreted                               []ast.ASTHandle
	trueVar                                   sat.BoolVar
	boolSort                                  ast.ASTHandle
	maxMemory                                 uint64
	ownerBytes                                uint64
	iteExtra, defaultExternal, failed, closed bool
}

func NewGoalTranslator(m *ast.ASTManager, s *sat.Solver) (tr *GoalTranslator, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if err = s.CheckOwner(); err != nil {
		return nil, err
	}
	numerals := m.NumeralService()
	if numerals == nil || numerals.RuntimeOwner() == nil {
		return nil, ast.ErrASTHandle
	}
	solverProcess := s.RuntimeOwner().ProcessOwner()
	astProcess := numerals.RuntimeOwner().ProcessOwner()
	if (solverProcess != nil || astProcess != nil) && solverProcess != astProcess {
		return nil, sat.ErrSATOwner
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(GoalTranslator{}).Size())
	if err = s.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	tr = &GoalTranslator{manager: m, solver: s, Limit: m.ResourceLimit(), trueVar: sat.NullBoolVar, boolSort: boolSort, ownerBytes: amount}
	tr.self = tr
	defer func() {
		if err != nil {
			if cleanup := tr.Close(); cleanup != nil {
				err = errors.Join(err, cleanup)
			} else {
				tr = nil
			}
		}
	}()
	hashIdentity := func(h ast.ASTHandle) uint64 {
		return h.Origin() ^ uint64(h.Slot())*0x9e3779b97f4a7c15 ^ uint64(h.Generation())*0xbf58476d1ce4e5b9
	}
	tr.Atoms = *insdict.NewDictFunc[ast.ASTHandle, uint32](hashIdentity)
	tr.dependencies = *insdict.NewDictFunc[ast.ASTHandle, uint32](hashIdentity)
	tr.cache = make(map[ast.ASTHandle]uint32)
	tr.interfaces = make(map[ast.ASTHandle]bool)
	name := "ite_extra"
	if tr.iteExtra, err = s.BoolParameter(&name, true); err != nil {
		return tr, err
	}
	name = "max_memory"
	var mb uint32
	if mb, err = s.UIntParameter(&name, ^uint32(0)); err != nil {
		return tr, err
	}
	tr.maxMemory = umemory.MegabytesToBytes(mb)
	return tr, nil
}
func (tr *GoalTranslator) open() error {
	if tr == nil || tr.self != tr {
		return sat.ErrSATOwner
	}
	if tr.closed {
		return arena.ErrClosed
	}
	if tr.failed {
		return ErrSATGoalFailed
	}
	if tr.sharedAtoms != nil {
		if _, err := tr.sharedAtoms.Manager(); err != nil {
			return err
		}
	}
	return tr.solver.CheckOwner()
}

// BindCooperation borrows the source process owner used by goal2sat loops.
func (tr *GoalTranslator) BindCooperation(owner *cooperate.Cooperation) error {
	if err := tr.open(); err != nil {
		return err
	}
	if owner == nil {
		return arena.ErrConfig
	}
	if _, err := owner.Enabled(); err != nil {
		return err
	}
	tr.cooperation = owner
	return nil
}
func (tr *GoalTranslator) Close() error {
	if tr == nil || tr.self != tr {
		return sat.ErrSATOwner
	}
	if tr.closed {
		return nil
	}
	for len(tr.dependencyTrail) > 0 {
		i := len(tr.dependencyTrail) - 1
		consumed, e := tr.manager.DecRefConsumed(tr.dependencyTrail[i])
		if consumed {
			tr.dependencyTrail[i] = ast.ASTHandle{}
			tr.dependencyTrail = tr.dependencyTrail[:i]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	for len(tr.interpreted) > 0 {
		i := len(tr.interpreted) - 1
		consumed, e := tr.manager.DecRefConsumed(tr.interpreted[i])
		if consumed {
			tr.interpreted[i] = ast.ASTHandle{}
			tr.interpreted = tr.interpreted[:i]
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	for h := range tr.Atoms.All() {
		consumed, e := tr.manager.DecRefConsumed(h)
		if consumed {
			tr.Atoms.Del(h)
		}
		if e != nil {
			return e
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	tr.cooperation = nil
	b := tr.solver.Budget()
	tr.cache = nil
	tr.Atoms = insdict.Dict[ast.ASTHandle, uint32]{}
	tr.interfaces = nil
	tr.dependencies = insdict.Dict[ast.ASTHandle, uint32]{}
	tr.dependencyTrail = nil
	tr.interpreted = nil
	tr.frames = nil
	tr.results = nil
	tr.sharedAtoms = nil
	b.Release(tr.ownerBytes)
	tr.ownerBytes = 0
	tr.closed = true
	return nil
}
func (tr *GoalTranslator) HasInterpretedAtoms() (bool, error) {
	if e := tr.open(); e != nil {
		return false, e
	}
	return len(tr.interpreted) != 0, nil
}
func (tr *GoalTranslator) retainInterpreted(h ast.ASTHandle) error {
	if e := tr.manager.IncRef(h); e != nil {
		return e
	}
	tr.interpreted = append(tr.interpreted, h)
	return nil
}
func (tr *GoalTranslator) pushFrame(fr satGoalFrame) error {
	tr.frames = append(tr.frames, fr)
	return nil
}
func (tr *GoalTranslator) pushResult(l sat.Literal) error {
	tr.results = append(tr.results, l)
	return nil
}
func (tr *GoalTranslator) Variable(h ast.ASTHandle) (sat.BoolVar, error) {
	if e := tr.open(); e != nil {
		return sat.NullBoolVar, e
	}
	if _, e := tr.manager.GetKind(h); e != nil {
		return sat.NullBoolVar, e
	}
	v, found, err := tr.atomVariable(h)
	if err != nil {
		return sat.NullBoolVar, err
	}
	if found {
		return sat.BoolVar(v), nil
	}
	return sat.NullBoolVar, nil
}
func (tr *GoalTranslator) isBool(h ast.ASTHandle) (bool, error) {
	sort, e := tr.manager.GetSort(h)
	return sort == tr.boolSort, e
}
func (tr *GoalTranslator) isUninterpretedConstant(h ast.ASTHandle) (bool, error) {
	k, e := tr.manager.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	n, e := tr.manager.GetNumArgs(h)
	if e != nil || n != 0 {
		return false, e
	}
	family, e := tr.manager.GetFamilyID(h)
	return family == -1, e
}
func (tr *GoalTranslator) emit(root, sign bool, l sat.Literal) error {
	if sign {
		l = l.Negated()
	}
	if root {
		return tr.mkClause([]sat.Literal{l}, false)
	}
	return tr.pushResult(l)
}
func (tr *GoalTranslator) cached(h ast.ASTHandle, root, sign bool) (bool, error) {
	v, found := tr.cache[h]
	if !found {
		return false, nil
	}
	return true, tr.emit(root, sign, sat.Literal(v))
}
func (tr *GoalTranslator) convertAtom(h ast.ASTHandle, root, sign bool) error {
	isBool, e := tr.isBool(h)
	if e != nil {
		return e
	}
	if !isBool {
		return ast.ErrASTDomain
	}
	value, found, e := tr.atomVariable(h)
	if e != nil {
		return e
	}
	var l sat.Literal
	if found {
		l = sat.MakeLiteral(sat.BoolVar(value), false)
	} else {
		// Compare canonical basic-plugin nodes, exactly as is_true/is_false.
		if tr.manager.IsTrue(h) || tr.manager.IsFalse(h) {
			if tr.trueVar == sat.NullBoolVar {
				tr.trueVar, e = tr.solver.MkVar(false, true)
				if e != nil {
					return e
				}
				if e = tr.mkClause([]sat.Literal{sat.MakeLiteral(tr.trueVar, false)}, false); e != nil {
					return e
				}
			}
			l = sat.MakeLiteral(tr.trueVar, tr.manager.IsFalse(h))
		} else {
			uninterpreted, e := tr.isUninterpretedConstant(h)
			if e != nil {
				return e
			}
			inInterface := tr.interfaces[h]
			v, e := tr.solver.MkVar(tr.defaultExternal || !uninterpreted || inInterface, true)
			if e != nil {
				return e
			}
			if !uninterpreted && tr.sharedAtoms == nil {
				if e = tr.traceInterpretedAtom(h); e != nil {
					return e
				}
			}
			if e = tr.insertAtom(h, uint32(v)); e != nil {
				return e
			}
			l = sat.MakeLiteral(v, false)
			if e = tr.traceGoalNewVar(v, h); e != nil {
				return e
			}
			if !uninterpreted {
				if e = tr.retainInterpreted(h); e != nil {
					return e
				}
			}
		}
	}
	return tr.emit(root, sign, l)
}
func (tr *GoalTranslator) visit(h ast.ASTHandle, root, sign bool) (bool, error) {
	kind, e := tr.manager.GetKind(h)
	if e != nil {
		return true, e
	}
	if kind != ast.ASTApp {
		return true, tr.convertAtom(h, root, sign)
	}
	if found, e := tr.cached(h, root, sign); found || e != nil {
		return true, e
	}
	family, e := tr.manager.GetFamilyID(h)
	if e != nil {
		return true, e
	}
	if family != 0 {
		return true, tr.convertAtom(h, root, sign)
	}
	op, e := tr.manager.GetDeclKind(h)
	if e != nil {
		return true, e
	}
	switch op {
	case ast.ASTOpNot, ast.ASTOpOr, ast.ASTOpAnd, ast.ASTOpIff:
	case ast.ASTOpIte, ast.ASTOpEq:
		arg, e := tr.manager.GetArg(h, 1)
		if e != nil {
			return true, e
		}
		ok, e := tr.isBool(arg)
		if e != nil {
			return true, e
		}
		if !ok {
			return true, tr.convertAtom(h, root, sign)
		}
	case ast.ASTOpXor, ast.ASTOpImplies, ast.ASTOpDistinct:
		if e := tr.traceGoalUnhandled(h); e != nil {
			return true, e
		}
		formula, e := tr.traceGoalFormula(h)
		if e == nil {
			return true, satGoalUnsupportedOperator{formula: formula}
		}
		if !errors.Is(e, uparams.ErrGlobalParamsOwner) {
			return true, e
		}
		// Standalone managers may have no process parameter owner yet. Preserve
		// the originating node for that explicit unbound state.
		return true, fmt.Errorf("%w (AST %d, operation %d)", ErrSATGoalUnsupported, h.Slot(), op)
	default:
		return true, tr.convertAtom(h, root, sign)
	}
	return false, tr.pushFrame(satGoalFrame{term: h, root: root, sign: sign})
}
func (tr *GoalTranslator) Compile(formulas []ast.ASTHandle) (err error) {
	if err = tr.open(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tr.failed = true
		}
	}()
	clear(tr.interfaces)
	if err = tr.collectInterface(formulas); err != nil {
		return err
	}
	for _, h := range formulas {
		if err = tr.process(h); err != nil {
			return err
		}
	}
	return nil
}
func (tr *GoalTranslator) process(h ast.ASTHandle) error {
	if e := tr.traceGoalStart(h); e != nil {
		return e
	}
	done, e := tr.visit(h, true, false)
	if e != nil || done {
		return e
	}
	for len(tr.frames) != 0 {
		if tr.cooperation != nil {
			if e = tr.cooperation.Cooperate("goal2sat"); e != nil {
				return e
			}
		}
		if !tr.Limit.Inc() {
			return tactic.NewTacticException(tr.Limit.CancelMessage())
		}
		size, e := tr.solver.RuntimeOwner().AllocationSize()
		if e != nil {
			return e
		}
		if size > tr.maxMemory {
			return tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
		}
		i := len(tr.frames) - 1
		fr := tr.frames[i]
		if e = tr.traceGoalResultStack(396, "result stack\n", fr); e != nil {
			return e
		}
		if fr.index == 0 {
			if found, e := tr.cached(fr.term, fr.root, fr.sign); found || e != nil {
				if e != nil {
					return e
				}
				tr.frames = tr.frames[:i]
				continue
			}
		}
		op, e := tr.manager.GetDeclKind(fr.term)
		if e != nil {
			return e
		}
		if op == ast.ASTOpNot {
			tr.frames = tr.frames[:i]
			arg, e := tr.manager.GetArg(fr.term, 0)
			if e != nil {
				return e
			}
			if _, e = tr.visit(arg, fr.root, !fr.sign); e != nil {
				return e
			}
			continue
		}
		n, e := tr.manager.GetNumArgs(fr.term)
		if e != nil {
			return e
		}
		descended := false
		for tr.frames[i].index < n {
			arg, e := tr.manager.GetArg(fr.term, tr.frames[i].index)
			if e != nil {
				return e
			}
			tr.frames[i].index++
			done, e := tr.visit(arg, false, false)
			if e != nil {
				return e
			}
			if !done {
				descended = true
				break
			}
		}
		if descended {
			continue
		}
		if e = tr.traceGoalResultStack(415, "converting\n", fr); e != nil {
			return e
		}
		if e = tr.convert(fr, op, int(n)); e != nil {
			return e
		}
		tr.frames = tr.frames[:i]
	}
	if e := tr.traceGoalRemaining(); e != nil {
		return e
	}
	if len(tr.results) != 0 {
		return ast.ErrASTDomain
	}
	return nil
}
func (tr *GoalTranslator) convert(fr satGoalFrame, op int32, n int) error {
	if err := tr.traceGoalConversion(op, fr); err != nil {
		return err
	}
	sz := len(tr.results)
	if sz < n || fr.root && sz != n {
		return ast.ErrASTDomain
	}
	base := sz - n
	var l sat.Literal
	if !fr.root {
		v, e := tr.solver.MkVar(false, true)
		if e != nil {
			return e
		}
		l = sat.MakeLiteral(v, false)
		tr.cache[fr.term] = uint32(l)
	}
	switch op {
	case ast.ASTOpOr, ast.ASTOpAnd:
		if fr.root {
			units := op == ast.ASTOpAnd && !fr.sign || op == ast.ASTOpOr && fr.sign
			if units {
				for _, child := range tr.results {
					if fr.sign {
						child = child.Negated()
					}
					if e := tr.mkClause([]sat.Literal{child}, false); e != nil {
						return e
					}
				}
			} else {
				if fr.sign {
					for i := range tr.results {
						tr.results[i] = tr.results[i].Negated()
					}
				}
				if e := tr.mkClause(tr.results, true); e != nil {
					return e
				}
			}
		} else {
			for _, child := range tr.results[base:] {
				pair := [2]sat.Literal{child.Negated(), l}
				if op == ast.ASTOpAnd {
					pair = [2]sat.Literal{l.Negated(), child}
				}
				if e := tr.mkClause(pair[:], false); e != nil {
					return e
				}
			}
			extra := l.Negated()
			if op == ast.ASTOpAnd {
				extra = l
				for i := base; i < sz; i++ {
					tr.results[i] = tr.results[i].Negated()
				}
			}
			if e := tr.pushResult(extra); e != nil {
				return e
			}
			if e := tr.mkClause(tr.results[base:], true); e != nil {
				return e
			}
		}
	case ast.ASTOpIte:
		if n != 3 {
			return ast.ErrASTDomain
		}
		c, t, e := tr.results[base], tr.results[base+1], tr.results[base+2]
		if fr.root {
			if fr.sign {
				t = t.Negated()
				e = e.Negated()
			}
			if err := tr.mkClause([]sat.Literal{c.Negated(), t}, false); err != nil {
				return err
			}
			if err := tr.mkClause([]sat.Literal{c, e}, false); err != nil {
				return err
			}
		} else {
			triples := [6][3]sat.Literal{{l.Negated(), c.Negated(), t}, {l.Negated(), c, e}, {l, c.Negated(), t.Negated()}, {l, c, e.Negated()}, {t.Negated(), e.Negated(), l}, {t, e, l.Negated()}}
			count := 4
			if tr.iteExtra {
				count = 6
			}
			for i := 0; i < count; i++ {
				if err := tr.mkClause(triples[i][:], false); err != nil {
					return err
				}
			}
		}
	case ast.ASTOpIff, ast.ASTOpEq:
		if n != 2 {
			return ast.ErrASTDomain
		}
		l1, l2 := tr.results[base+1], tr.results[base]
		if fr.root {
			pairs := [2][2]sat.Literal{{l1, l2.Negated()}, {l1.Negated(), l2}}
			if fr.sign {
				pairs = [2][2]sat.Literal{{l1, l2}, {l1.Negated(), l2.Negated()}}
			}
			for i := range pairs {
				if e := tr.mkClause(pairs[i][:], false); e != nil {
					return e
				}
			}
		} else {
			triples := [4][3]sat.Literal{{l.Negated(), l1, l2.Negated()}, {l.Negated(), l1.Negated(), l2}, {l, l1, l2}, {l, l1.Negated(), l2.Negated()}}
			for i := range triples {
				if e := tr.mkClause(triples[i][:], false); e != nil {
					return e
				}
			}
		}
	default:
		return ast.ErrASTDomain
	}
	tr.results = tr.results[:base]
	if !fr.root {
		if err := tr.emit(false, fr.sign, l); err != nil {
			return err
		}
		if op == ast.ASTOpAnd {
			return tr.traceGoalAndResult()
		}
	}
	return nil
}
