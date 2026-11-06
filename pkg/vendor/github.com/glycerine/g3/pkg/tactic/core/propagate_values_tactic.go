package core

// Source: src/tactic/core/propagate_values_tactic.cpp/.h.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type PropagateValuesTactic struct {
	self            *PropagateValuesTactic
	manager         *ast.ASTManager
	context         *uparams.ParamsContext
	globals         *uparams.GlobalParamsService
	params          uparams.ParamsRef
	rewriter        *rewriter.TheoryRewriter
	occs            *tactic.GoalSharedOccurrences
	substitution    ast.ASTExprSubstitution
	hasSubstitution bool
	maxRounds       uint32
	refs            uint32
	ownerBytes      uint64
	trace           *diagnostic.TraceTags
	closed          bool
}

var _ tactic.Tactic = (*PropagateValuesTactic)(nil)

func NewPropagateValuesTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, params *uparams.ParamsRef) (*PropagateValuesTactic, error) {
	if m == nil || context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	stored, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(PropagateValuesTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, context.Del(&stored))
	}
	t := &PropagateValuesTactic{manager: m, context: context, globals: globals, params: stored, ownerBytes: amount, maxRounds: 4}
	t.self = t
	if params != nil {
		if err = context.Assign(&t.params, params); err != nil {
			return nil, errors.Join(err, t.Close())
		}
	}
	if err = t.initialize(); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *PropagateValuesTactic) initialize() (err error) {
	key := "max_rounds"
	if t.maxRounds, err = t.context.GetUIntName(&t.params, &key, 4); err != nil {
		return err
	}
	t.rewriter, err = rewriter.NewTheoryRewriter(t.manager, t.context, t.globals, &t.params)
	if err != nil {
		return err
	}
	t.occs, err = tactic.NewGoalSharedOccurrences(t.manager, true, true, false)
	return err
}

func (t *PropagateValuesTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.rewriter == nil || t.occs == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *PropagateValuesTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *PropagateValuesTactic) DecRef() error {
	// A failed cleanup may leave no executable rewriter/occurrence counter.
	// Counted source destruction still owns and must retire that partial state.
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
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

func (t *PropagateValuesTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	defer func() {
		if rewriter.IsRewriterException(err) {
			err = tactic.NewTacticException(err.Error())
		}
	}()
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if good, e := goal.IsWellSorted(); e != nil {
		return e
	} else if !good {
		return ast.ErrASTDomain
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("propagate-values", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = t.propagate(goal); err != nil {
		return err
	}
	if err = goal.ElimRedundancies(); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	if good, e := goal.IsWellSorted(); e != nil {
		return e
	} else if !good {
		return ast.ErrASTDomain
	}
	if err = t.traceGoal("propagate_values", 211, "end\n", goal, false); err != nil {
		return err
	}
	return t.traceGoal("propagate_values_core", 212, "", goal, true)
}

func (t *PropagateValuesTactic) propagate(goal *tactic.Goal) error {
	inconsistent, err := goal.Inconsistent()
	if err != nil || inconsistent || t.maxRounds == 0 {
		return err
	}
	if err = t.clearSubstitution(); err != nil {
		return err
	}
	cores, err := goal.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	t.substitution, err = t.manager.NewExprSubstitutionPolicy(cores, proofs)
	if err != nil {
		return err
	}
	t.hasSubstitution = true
	if err = t.rewriter.SetSubstitution(&t.substitution); err != nil {
		return err
	}
	if err = t.occs.Collect(goal); err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	index := uint32(0)
	forward, modified := true, false
	for round := uint32(0); ; {
		if err = t.traceGoal("propagate_values", 168, "while(true) loop\n", goal, true); err != nil {
			return err
		}
		if forward {
			for ; index < size; index++ {
				changed, e := t.processCurrent(goal, index)
				if e != nil {
					return e
				}
				modified = modified || changed
				if inconsistent, e = goal.Inconsistent(); e != nil || inconsistent {
					return e
				}
			}
			empty, e := t.manager.ExprSubstitutionEmpty(t.substitution)
			if e != nil {
				return e
			}
			if empty && !modified {
				return nil
			}
			if err = t.occs.Collect(goal); err != nil {
				return err
			}
			if index, err = goal.Size(); err != nil {
				return err
			}
			forward = false
			if err = t.resetSubstitution(); err != nil {
				return err
			}
		} else {
			for index > 0 {
				index--
				changed, e := t.processCurrent(goal, index)
				if e != nil {
					return e
				}
				modified = modified || changed
				if inconsistent, e = goal.Inconsistent(); e != nil || inconsistent {
					return e
				}
			}
			if !modified {
				return nil
			}
			if err = t.resetSubstitution(); err != nil {
				return err
			}
			modified = false
			if err = t.occs.Collect(goal); err != nil {
				return err
			}
			index = 0
			if size, err = goal.Size(); err != nil {
				return err
			}
			forward = true
		}
		round++
		if round >= t.maxRounds {
			return nil
		}
		if err = t.reportRound(goal); err != nil {
			return err
		}
		if err = t.traceGoal("propagate_values", 204, "round finished\n", goal, false); err != nil {
			return err
		}
	}
}

func (t *PropagateValuesTactic) resetSubstitution() error {
	if err := t.manager.ExprSubstitutionReset(t.substitution); err != nil {
		return err
	}
	return t.rewriter.SetSubstitution(&t.substitution)
}

func (t *PropagateValuesTactic) processCurrent(goal *tactic.Goal, index uint32) (modified bool, err error) {
	m := t.manager
	curr, err := goal.Form(index)
	if err != nil {
		return false, err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&out)) }()
	proof, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&proof)) }()
	empty, err := m.ExprSubstitutionEmpty(t.substitution)
	if err != nil {
		return false, err
	}
	managerProofs, err := m.ProofsEnabled()
	if err != nil {
		return false, err
	}
	if !empty {
		if managerProofs {
			err = t.rewriter.RewriteWithProof(curr, out, proof)
		} else {
			err = t.rewriter.Rewrite(curr, out)
		}
		if err != nil {
			return false, err
		}
	} else {
		if err = m.SetRef(out, curr); err != nil {
			return false, err
		}
		if managerProofs {
			reflexivity, e := m.MkProofReflexivity(curr)
			if e != nil {
				return false, e
			}
			if err = m.SetRef(proof, reflexivity); err != nil {
				return false, err
			}
		}
	}
	value, err := m.RefGet(out)
	if err != nil {
		return false, err
	}
	if err = t.traceRewrite(curr, value); err != nil {
		return false, err
	}
	if err = t.pushResult(goal, index, value, proof); err != nil {
		return false, err
	}
	return value != curr, nil
}

func (t *PropagateValuesTactic) pushResult(goal *tactic.Goal, index uint32, value ast.ASTHandle, proof ast.ASTRef) (err error) {
	m := t.manager
	newProof, err := m.RefGet(proof)
	if err != nil {
		return err
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		old, e := goal.Proof(index)
		if e != nil {
			return e
		}
		if newProof, err = m.MkProofModusPonens(old, newProof); err != nil {
			return err
		}
		if err = m.SetRef(proof, newProof); err != nil {
			return err
		}
	}
	var dependency tactic.DependencyRef
	defer func() { err = errors.Join(err, dependency.Close()) }()
	cores, err := goal.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	if cores {
		old, e := goal.Dependency(index)
		if e != nil {
			return e
		}
		if err = dependency.Set(m, old); err != nil {
			return err
		}
		used, e := t.rewriter.UsedDependencies()
		if e != nil {
			return e
		}
		if !used.IsNull() {
			joined, e := m.DependencyJoin(old, used)
			if e != nil {
				return e
			}
			if err = dependency.Set(m, joined); err != nil {
				return err
			}
			if err = t.rewriter.ResetUsedDependencies(); err != nil {
				return err
			}
		}
	}
	dep := dependency.Get()
	if err = goal.Update(index, value, newProof, dep); err != nil {
		return err
	}
	shared, err := t.occs.IsShared(value)
	if err != nil {
		return err
	}
	if shared {
		truth, e := m.MkTrue()
		if e != nil {
			return e
		}
		iff, e := m.MkProofIffTrue(newProof)
		if e != nil {
			return e
		}
		if err = m.ExprSubstitutionInsert(t.substitution, value, truth, iff, dep); err != nil {
			return err
		}
	}
	negative, atom, err := m.IsNot(value)
	if err != nil {
		return err
	}
	if negative {
		shared, e := t.occs.IsShared(atom)
		if e != nil {
			return e
		}
		if shared {
			falsity, e := m.MkFalse()
			if e != nil {
				return e
			}
			iff, e := m.MkProofIffFalse(newProof)
			if e != nil {
				return e
			}
			if err = m.ExprSubstitutionInsert(t.substitution, atom, falsity, iff, dep); err != nil {
				return err
			}
		}
	}
	nodeKind, err := m.GetKind(value)
	if err != nil {
		return err
	}
	if nodeKind != ast.ASTApp {
		return nil
	}
	family, err := m.GetFamilyID(value)
	if err != nil {
		return err
	}
	kind, err := m.GetDeclKind(value)
	if err != nil {
		return err
	}
	if family != m.BasicFamilyID() || kind != ast.ASTOpEq {
		return nil
	}
	lhs, err := m.GetArg(value, 0)
	if err != nil {
		return err
	}
	rhs, err := m.GetArg(value, 1)
	if err != nil {
		return err
	}
	for _, pair := range [][2]ast.ASTHandle{{lhs, rhs}, {rhs, lhs}} {
		isValue, e := m.IsValue(pair[0])
		if e != nil {
			return e
		}
		if !isValue {
			continue
		}
		shared, e := t.occs.IsShared(pair[1])
		if e != nil {
			return e
		}
		if !shared {
			continue
		}
		if err = t.traceEquality(value); err != nil {
			return err
		}
		return m.ExprSubstitutionInsert(t.substitution, pair[1], pair[0], newProof, dep)
	}
	return nil
}

func (t *PropagateValuesTactic) clearSubstitution() error {
	// A failed constructor can leave a partial rewriter with no substitution.
	// Close can release that owner; its public methods require initialization.
	if t.hasSubstitution && t.rewriter != nil {
		if err := t.rewriter.SetSubstitution(nil); err != nil {
			return err
		}
		if err := t.rewriter.ResetUsedDependencies(); err != nil {
			return err
		}
	}
	if t.hasSubstitution {
		if err := t.manager.DelExprSubstitution(&t.substitution); err != nil {
			return err
		}
		t.hasSubstitution = false
	}
	return nil
}

func (t *PropagateValuesTactic) UpdateParams(params *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, params); err != nil {
		return err
	}
	if err := t.rewriter.UpdateParams(params); err != nil {
		return err
	}
	key := "max_rounds"
	var err error
	t.maxRounds, err = t.context.GetUIntName(params, &key, 4)
	return err
}
func (t *PropagateValuesTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := rewriter.TheoryRewriterParameterDescriptions(out); err != nil {
		return err
	}
	description := "(default: 2) maximum number of rounds."
	return out.InsertName("max_rounds", uparams.ParamUInt, &description, nil, nil)
}
func (*PropagateValuesTactic) CollectStatistics() map[string]float64    { return nil }
func (t *PropagateValuesTactic) ResetStatistics() error                 { return t.open() }
func (t *PropagateValuesTactic) Reset() error                           { return t.Cleanup() }
func (t *PropagateValuesTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *PropagateValuesTactic) SetProgressCallback(func(uint32)) error { return t.open() }

func (t *PropagateValuesTactic) releaseImplementation() error {
	if err := t.clearSubstitution(); err != nil {
		return err
	}
	if t.occs != nil {
		if err := t.occs.Close(); err != nil {
			return err
		}
		t.occs = nil
	}
	if t.rewriter != nil {
		if err := t.rewriter.Close(); err != nil {
			return err
		}
		t.rewriter = nil
	}
	return nil
}
func (t *PropagateValuesTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.releaseImplementation(); err != nil {
		return err
	}
	return t.initialize()
}
func (t *PropagateValuesTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	copy, err := NewPropagateValuesTactic(m, t.context, t.globals, &t.params)
	if err != nil {
		return nil, err
	}
	copy.trace = t.trace
	return copy, nil
}
func (t *PropagateValuesTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.releaseImplementation(); err != nil {
		return err
	}
	if err := t.context.Del(&t.params); err != nil {
		return err
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}

func (t *PropagateValuesTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if err := t.open(); err != nil {
		return err
	}
	t.trace = tags
	return nil
}
func (t *PropagateValuesTactic) emit(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, "src/tactic/core/propagate_values_tactic.cpp", line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (t *PropagateValuesTactic) traceGoal(tag string, line int, prefix string, goal *tactic.Goal, dependencies bool) error {
	return t.emit(tag, "operator()", line, func(out io.Writer) error {
		if _, err := io.WriteString(out, prefix); err != nil {
			return err
		}
		if dependencies {
			return goal.DisplayWithDependencies(out)
		}
		if err := goal.Display(out); err != nil {
			return err
		}
		if line == 204 {
			_, err := io.WriteString(out, "\n")
			return err
		}
		return nil
	})
}
func (t *PropagateValuesTactic) writeExpression(out io.Writer, term ast.ASTHandle) (err error) {
	format, err := t.manager.SMT2PPExprTree(term, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	empty := t.context.EmptyValue()
	if err = t.manager.PrintFormat(stream, format, t.context, &empty, t.globals); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}
func (t *PropagateValuesTactic) traceRewrite(before, after ast.ASTHandle) error {
	return t.emit("shallow_context_simplifier_bug", "process_current", 131, func(out io.Writer) error {
		if err := t.writeExpression(out, before); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n---->\n"); err != nil {
			return err
		}
		if err := t.writeExpression(out, after); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}
func (t *PropagateValuesTactic) traceEquality(term ast.ASTHandle) error {
	return t.emit("shallow_context_simplifier_bug", "push_result", 112, func(out io.Writer) error {
		if _, err := io.WriteString(out, "found eq:\n"); err != nil {
			return err
		}
		if err := t.writeExpression(out, term); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}
func (t *PropagateValuesTactic) reportRound(goal *tactic.Goal) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(100)
	if err != nil || !enabled {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = stream.AppendCString("starting new round, goal size: "); err != nil {
		return err
	}
	count, err := goal.NumExprs()
	if err != nil {
		return err
	}
	if err = stream.AppendUint32(count); err != nil {
		return err
	}
	return stream.AppendCString("\n")
}
