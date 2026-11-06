package arith

// Source: src/tactic/arith/lia2card_tactic.cpp/.h.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type LIA2CardTactic struct {
	self                                 *LIA2CardTactic
	manager                              *ast.ASTManager
	context                              *uparams.ParamsContext
	globals                              *uparams.GlobalParamsService
	params                               uparams.ParamsRef
	bounds                               *BoundManager
	trace                                *diagnostic.TraceTags
	ownerBytes                           uint64
	refs                                 uint32
	compileEquality, paramsReady, closed bool
}

var _ tactic.Tactic = (*LIA2CardTactic)(nil)

func NewLIA2CardTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*LIA2CardTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(LIA2CardTactic{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &LIA2CardTactic{manager: m, context: c, globals: g, ownerBytes: bytes}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.bounds, err = NewBoundManager(m, c, g)
	}
	if err == nil {
		key := "compile_equality"
		t.compileEquality, err = c.GetBoolName(&t.params, &key, false)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func MakeLIA2CardTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (tactic.Tactic, error) {
	child, err := NewLIA2CardTactic(m, c, g, p)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}

func (t *LIA2CardTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.bounds == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *LIA2CardTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *LIA2CardTactic) DecRef() error {
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
func (t *LIA2CardTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.bounds != nil {
		if err := t.bounds.Close(); err != nil {
			return err
		}
		t.bounds = nil
	}
	if t.paramsReady {
		if err := t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *LIA2CardTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.bounds.Reset()
}
func (t *LIA2CardTactic) Reset() error { return t.Cleanup() }
func (t *LIA2CardTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	key := "compile_equality"
	value, err := t.context.GetBoolName(&t.params, &key, false)
	if err == nil {
		t.compileEquality = value
	}
	return err
}
func (t *LIA2CardTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	description := "(default:false) compile equalities into pseudo-Boolean equality"
	return out.InsertName("compile_equality", uparams.ParamBool, &description, nil, nil)
}
func (t *LIA2CardTactic) CollectStatistics() map[string]float64  { return nil }
func (t *LIA2CardTactic) ResetStatistics() error                 { return t.open() }
func (t *LIA2CardTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *LIA2CardTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *LIA2CardTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	next, err := NewLIA2CardTactic(m, t.context, t.globals, &t.params)
	if err != nil {
		return nil, err
	}
	next.trace, next.bounds.trace = t.trace, t.trace
	return next, nil
}

func (t *LIA2CardTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("cardinality-intro", goal)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, report.Close()) }()
	if err = t.bounds.Reset(); err != nil {
		return err
	}
	if err = t.bounds.CollectGoal(goal); err != nil {
		return err
	}
	variables, err := t.bounds.Variables()
	if err != nil {
		return err
	}
	zeroOne := make(map[ast.ASTHandle]bool)
	for _, variable := range variables {
		sort, e := t.manager.GetSort(variable)
		if e != nil {
			return e
		}
		family, e := t.manager.GetFamilyID(sort)
		if e != nil {
			return e
		}
		arith, e := t.manager.RegisterArithmeticPlugin()
		if e != nil {
			return e
		}
		kind, e := t.manager.GetDeclKind(sort)
		if e != nil {
			return e
		}
		if family != arith || kind != 1 {
			continue
		}
		lower, strictLower, hasLower, e := t.bounds.Lower(variable)
		if e != nil {
			return e
		}
		upper, strictUpper, hasUpper, e := t.bounds.Upper(variable)
		if e != nil {
			return e
		}
		if hasLower && hasUpper && !strictLower && !strictUpper && lower.Sign() == 0 && upper.Cmp(big.NewRat(1, 1)) == 0 {
			zeroOne[variable] = true
			if e = t.traceBound(variable); e != nil {
				return e
			}
		}
	}
	arith, err := t.manager.RegisterArithmeticPlugin()
	if err != nil {
		return err
	}
	walk, err := rewriter.NewTraversal(t.manager, &lia2cardRewriteConfig{manager: t.manager, arithmetic: arith, zeroOne: zeroOne, compileEquality: t.compileEquality, trace: t.trace, owner: t})
	if err != nil {
		if walk != nil {
			err = errors.Join(err, walk.Close())
		}
		return err
	}
	defer func() { err = errors.Join(err, walk.Close()) }()
	output, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&output)) }()
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	cores, err := goal.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	var originalProofs []ast.ASTRef
	if proofs {
		originalProofs, err = lia2cardOriginalProofs(t.manager, goal, size)
		if err != nil {
			return err
		}
		defer func() {
			for i := len(originalProofs) - 1; i >= 0; i-- {
				err = errors.Join(err, t.manager.DelRef(&originalProofs[i]))
			}
		}()
	}
	used := make(map[ast.ASTHandle]bool)
	for index := uint32(0); index < size; index++ {
		original, e := goal.Form(index)
		if e != nil {
			return e
		}
		if e = walk.Rewrite(original, output); e != nil {
			return e
		}
		value, e := t.manager.RefGet(output)
		if e != nil {
			return e
		}
		var proof ast.ASTHandle
		if proofs {
			if original == value {
				proof, e = goal.Proof(index)
			} else {
				proof, e = lia2cardGoalLemma(t.manager, arith, value, originalProofs)
			}
			if e != nil {
				return e
			}
		}
		dep, e := goal.Dependency(index)
		if e != nil {
			return e
		}
		if cores && value != original {
			dep, e = t.lia2cardDependencies(original, variables, zeroOne, dep)
			if e != nil {
				return e
			}
			e = errors.Join(goal.Update(index, value, proof, dep), t.manager.DependencyDec(dep))
		} else {
			e = goal.Update(index, value, proof, dep)
		}
		if e != nil {
			return e
		}
		for _, variable := range variables {
			if zeroOne[variable] && !used[variable] {
				used[variable], e = t.manager.OccursExpr(variable, value)
				if e != nil {
					return e
				}
			}
		}
	}
	zero, err := t.manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return err
	}
	one, err := t.manager.ArithmeticIntegerNumeralText("1")
	if err != nil {
		return err
	}
	for _, variable := range variables {
		if !used[variable] {
			continue
		}
		upper, e := t.manager.MkArithmeticLE(variable, one)
		if e != nil {
			return e
		}
		var upperDep, lowerDep ast.ASTDependency
		if cores {
			upperDep, e = t.bounds.UpperDependency(variable)
			if e != nil {
				return e
			}
			lowerDep, e = t.bounds.LowerDependency(variable)
			if e != nil {
				return e
			}
		}
		var upperProof ast.ASTHandle
		if proofs {
			upperProof, e = lia2cardGoalLemma(t.manager, arith, upper, originalProofs)
			if e != nil {
				return e
			}
		}
		if e = goal.AssertWithProof(upper, upperProof, upperDep); e != nil {
			return e
		}
		lower, e := t.manager.MkArithmeticLE(zero, variable)
		if e != nil {
			return e
		}
		var lowerProof ast.ASTHandle
		if proofs {
			lowerProof, e = lia2cardGoalLemma(t.manager, arith, lower, originalProofs)
			if e != nil {
				return e
			}
		}
		if e = goal.AssertWithProof(lower, lowerProof, lowerDep); e != nil {
			return e
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		wellSorted, e := goal.IsWellSorted()
		if e != nil {
			return e
		}
		if !wellSorted {
			return ast.ErrASTDomain
		}
	}
	return t.traceGoal(goal)
}
