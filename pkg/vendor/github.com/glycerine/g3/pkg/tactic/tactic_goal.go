package tactic

// Source: src/tactic/goal.h:39-143; src/tactic/goal.cpp:26-329,488-631,656-707.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type GoalPrecision uint8

const (
	GoalPrecise GoalPrecision = iota
	GoalUnder
	GoalOver
	GoalUnderOver
)

func tacticGoalPrecisionUnion(a, b GoalPrecision) GoalPrecision {
	if a == GoalPrecise {
		return b
	}
	if b == GoalPrecise {
		return a
	}
	if a != b {
		return GoalUnderOver
	}
	return a
}

// Native slices own formula, proof and dependency references. Copies acquire
// independent pins and backing storage. Borrowed services and managers outlive the goal.
type Goal struct {
	self                                                            *Goal
	manager                                                         *ast.ASTManager
	forms, proofs                                                   []ast.ASTHandle
	dependencies                                                    []ast.ASTDependency
	RefCount, depth                                                 uint32
	precision                                                       GoalPrecision
	modelsEnabled, proofsEnabled, coreEnabled, inconsistent, Closed bool
	ownerBytes                                                      uint64
	inequalitySourceAllocation                                      bool
}

func NewGoal(m *ast.ASTManager, proofs, models, cores bool) (*Goal, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	enabled, e := m.ProofsEnabled()
	if e != nil {
		return nil, e
	}
	if proofs && !enabled {
		return nil, ast.ErrASTDomain
	}
	amount := uint64(reflect.TypeOf(Goal{}).Size())
	if e = m.Budget().Reserve(amount); e != nil {
		return nil, e
	}
	g := &Goal{manager: m, proofsEnabled: proofs, modelsEnabled: models, coreEnabled: cores, ownerBytes: amount}
	g.self = g
	return g, nil
}

// NewDefaultGoal is goal(ast_manager&, bool, bool): proof production follows
// the manager's current proof mode.
func NewDefaultGoal(m *ast.ASTManager, models, cores bool) (*Goal, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	proofs, err := m.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	return NewGoal(m, proofs, models, cores)
}

// NewGoalCopy is goal(goal const&): copy configuration and assertions into an
// independently owning goal.
func NewGoalCopy(src *Goal) (result *Goal, err error) {
	if err = src.open(); err != nil {
		return nil, err
	}
	result, err = NewGoal(src.manager, src.proofsEnabled, src.modelsEnabled, src.coreEnabled)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	if err = src.CopyTo(result); err != nil {
		return nil, err
	}
	return result, nil
}

// NewGoalConfigCopy is goal(goal const&, bool): copy flags, depth and
// precision while leaving formulas, proofs and dependencies empty.
func NewGoalConfigCopy(src *Goal) (*Goal, error) {
	if err := src.open(); err != nil {
		return nil, err
	}
	g, err := NewGoal(src.manager, src.proofsEnabled, src.modelsEnabled, src.coreEnabled)
	if err != nil {
		return nil, err
	}
	g.depth, g.precision = src.depth, src.precision
	return g, nil
}

func (g *Goal) Translate(translation *ast.ASTTranslation) (result *Goal, err error) {
	if err = g.open(); err != nil {
		return nil, err
	}
	if translation == nil || translation.From() != g.manager || translation.To() == nil {
		return nil, ast.ErrASTHandle
	}
	targetProofs, err := translation.To().ProofsEnabled()
	if err != nil {
		return nil, err
	}
	result, err = NewGoal(translation.To(), targetProofs && g.proofsEnabled, g.modelsEnabled, g.coreEnabled)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && result != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	for i, form := range g.forms {
		translatedForm, e := translation.Translate(form)
		if e != nil {
			return nil, e
		}
		var translatedProof ast.ASTHandle
		if result.proofsEnabled {
			translatedProof, e = translation.Translate(g.proofs[i])
			if e != nil {
				return nil, e
			}
		}
		var translatedDependency ast.ASTDependency
		if result.coreEnabled {
			translatedDependency, e = translation.TranslateDependency(g.dependencies[i])
			if e != nil {
				return nil, e
			}
		}
		if e = goalPushExpr(result.manager, &result.forms, translatedForm); e != nil {
			return nil, e
		}
		if result.proofsEnabled {
			if e = goalPushExpr(result.manager, &result.proofs, translatedProof); e != nil {
				return nil, e
			}
		}
		if result.coreEnabled {
			if e = goalPushDependency(result.manager, &result.dependencies, translatedDependency); e != nil {
				return nil, e
			}
		}
	}
	result.inconsistent = g.inconsistent
	result.depth = g.depth
	result.precision = g.precision
	return result, nil
}
func (g *Goal) open() error {
	if g == nil || g.self != g {
		return ast.ErrASTHandle
	}
	if g.Closed {
		return arena.ErrClosed
	}
	_, e := g.manager.ProofsEnabled()
	return e
}
func (g *Goal) Size() (uint32, error) {
	if e := g.open(); e != nil {
		return 0, e
	}
	return uint32(len(g.forms)), nil
}
func (g *Goal) Form(i uint32) (ast.ASTHandle, error) {
	if e := g.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if uint64(i) >= uint64(len(g.forms)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return g.forms[i], nil
}

// IsWellSorted follows goal.cpp's formula order and stops at the first
// ill-sorted expression. AST warnings use the borrowed process warning owner.
func (g *Goal) IsWellSorted() (bool, error) {
	if e := g.open(); e != nil {
		return false, e
	}
	for _, form := range g.forms {
		good, e := g.manager.IsWellSorted(form, nil)
		if e != nil || !good {
			return good, e
		}
	}
	return true, nil
}
func (g *Goal) Proof(i uint32) (ast.ASTHandle, error) {
	if e := g.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if !g.proofsEnabled {
		return ast.ASTHandle{}, nil
	}
	if uint64(i) >= uint64(len(g.proofs)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return g.proofs[i], nil
}
func (g *Goal) Dependency(i uint32) (ast.ASTDependency, error) {
	if e := g.open(); e != nil {
		return ast.ASTDependency{}, e
	}
	if !g.coreEnabled {
		return ast.ASTDependency{}, nil
	}
	if uint64(i) >= uint64(len(g.dependencies)) {
		return ast.ASTDependency{}, arena.ErrBounds
	}
	return g.dependencies[i], nil
}
func (g *Goal) SetDepth(d uint32) error {
	if e := g.open(); e != nil {
		return e
	}
	g.depth = d & ((1 << 26) - 1)
	return nil
}
func (g *Goal) IncDepth() error {
	if e := g.open(); e != nil {
		return e
	}
	return g.SetDepth(g.depth + 1)
}
func (g *Goal) SetPrecision(p GoalPrecision) error {
	if e := g.open(); e != nil {
		return e
	}
	g.precision = p & 3
	return nil
}
func (g *Goal) UpdatePrecision(p GoalPrecision) error {
	if e := g.open(); e != nil {
		return e
	}
	return g.SetPrecision(tacticGoalPrecisionUnion(g.precision, p))
}
func (g *Goal) SatPreserved() bool {
	return g.precision == GoalPrecise || g.precision == GoalUnder
}
func (g *Goal) UnsatPreserved() bool {
	return g.precision == GoalPrecise || g.precision == GoalOver
}
func (g *Goal) IsDecidedSat() bool {
	n, e := g.Size()
	return e == nil && n == 0 && g.SatPreserved()
}
func (g *Goal) IsDecidedUnsat() bool {
	return g != nil && !g.Closed && g.inconsistent && g.UnsatPreserved()
}
func (g *Goal) IsDecided() bool { return g.IsDecidedSat() || g.IsDecidedUnsat() }

// GoalsEqual implements the source is_equal helper. Formula order is ignored,
// but the total formula count and the number of unique formula identities must
// agree.
func GoalsEqual(first, second *Goal) (bool, error) {
	if first == nil || second == nil {
		return false, ast.ErrASTHandle
	}
	if err := first.open(); err != nil {
		return false, err
	}
	if err := second.open(); err != nil {
		return false, err
	}
	if first.manager != second.manager {
		return false, ast.ErrASTHandle
	}
	if len(first.forms) != len(second.forms) {
		return false, nil
	}
	left := make(map[ast.ASTHandle]bool, len(first.forms))
	for _, formula := range first.forms {
		left[formula] = true
	}
	right := make(map[ast.ASTHandle]bool, len(second.forms))
	for _, formula := range second.forms {
		if !left[formula] {
			return false, nil
		}
		right[formula] = true
	}
	return len(left) == len(right), nil
}
func (g *Goal) IncRef() error {
	if e := g.open(); e != nil {
		return e
	}
	g.RefCount++
	return nil
}
func (g *Goal) DecRef() error {
	_, e := g.DecRefConsumed()
	return e
}

// DecRefConsumed reports whether the source goal reference was decremented.
// Final destruction can stop after that decrement; the owning caller then
// resumes with Close instead of decrementing the same goal again.
func (g *Goal) DecRefConsumed() (bool, error) {
	if e := g.open(); e != nil {
		return false, e
	}
	g.RefCount--
	if g.RefCount == 0 {
		return true, g.Close()
	}
	return true, nil
}
func (g *Goal) resetCore() error {
	if e := goalDropExprs(g.manager, &g.forms); e != nil {
		return e
	}
	if e := goalDropExprs(g.manager, &g.proofs); e != nil {
		return e
	}
	return goalDropDependencies(g.manager, &g.dependencies)
}
func (g *Goal) Reset() error {
	if e := g.open(); e != nil {
		return e
	}
	if e := g.resetCore(); e != nil {
		return e
	}
	g.inconsistent = false
	return nil
}
func (g *Goal) ResetAll() error {
	if e := g.Reset(); e != nil {
		return e
	}
	g.depth = 0
	g.precision = GoalPrecise
	return nil
}
func (g *Goal) Close() error {
	if g == nil || g.self != g {
		return ast.ErrASTHandle
	}
	if g.Closed {
		return nil
	}
	if e := g.resetCore(); e != nil {
		return e
	}
	g.manager.Budget().Release(g.ownerBytes)
	g.ownerBytes = 0
	g.traceSourceMemoryDeallocated()
	g.Closed = true
	return nil
}
func (g *Goal) isOp(h ast.ASTHandle, op int32) (bool, error) {
	k, e := g.manager.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	family, e := g.manager.GetFamilyID(h)
	if e != nil || family != 0 {
		return false, e
	}
	kind, e := g.manager.GetDeclKind(h)
	return kind == op, e
}
func (g *Goal) notArgument(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	yes, e := g.isOp(h, ast.ASTOpNot)
	if e != nil || !yes {
		return ast.ASTHandle{}, false, e
	}
	arg, e := g.manager.GetArg(h, 0)
	return arg, true, e
}
func (g *Goal) pushBack(f, pr ast.ASTHandle, d ast.ASTDependency) (err error) {
	m := g.manager
	truth, err := m.MkTrue()
	if err != nil {
		return err
	}
	if f == truth {
		return nil
	}
	falsity, err := m.MkFalse()
	if err != nil {
		return err
	}
	if f == falsity {
		if err = m.IncRef(pr); err != nil {
			return err
		}
		defer func() { err = errors.Join(err, m.DecRef(pr)) }()
		if err = m.DependencyInc(d); err != nil {
			return err
		}
		defer func() { err = errors.Join(err, m.DependencyDec(d)) }()
		if err = g.resetCore(); err != nil {
			return err
		}
		g.inconsistent = true
	} else if g.inconsistent {
		return ast.ErrASTDomain
	}
	if err = goalPushExpr(m, &g.forms, f); err != nil {
		return err
	}
	if g.proofsEnabled {
		if err = goalPushExpr(m, &g.proofs, pr); err != nil {
			return err
		}
	}
	if g.coreEnabled {
		return goalPushDependency(m, &g.dependencies, d)
	}
	return nil
}
func (g *Goal) Assert(f ast.ASTHandle) (err error) {
	if err = g.open(); err != nil {
		return err
	}
	var pr ast.ASTHandle
	if g.proofsEnabled {
		if pr, err = g.manager.MkProofAsserted(f); err != nil {
			return err
		}
	}
	return g.AssertWithProof(f, pr, ast.ASTDependency{})
}
func (g *Goal) AssertWithProof(f, pr ast.ASTHandle, d ast.ASTDependency) (err error) {
	if err = g.open(); err != nil {
		return err
	}
	m := g.manager
	if err = m.IncRef(f); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DecRef(f)) }()
	if err = m.IncRef(pr); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DecRef(pr)) }()
	if err = m.DependencyInc(d); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DependencyDec(d)) }()
	undef, err := m.IsUndefProof(pr)
	if err != nil {
		return err
	}
	if g.proofsEnabled != (!pr.IsNull() && !undef) {
		return ast.ErrASTDomain
	}
	if g.inconsistent {
		return nil
	}
	if g.proofsEnabled {
		_, _, err = g.slowProcess(false, f, pr, d)
		return err
	}
	if err = m.IncRef(f); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DecRef(f)) }()
	_, err = g.quickProcess(false, f, d)
	return err
}

type tacticGoalPolarity struct {
	term     ast.ASTHandle
	positive bool
}
type tacticGoalTodo struct{ values []tacticGoalPolarity }

func (v *tacticGoalTodo) push(_ *ast.ASTManager, p tacticGoalPolarity) error {
	v.values = append(v.values, p)
	return nil
}
func (v *tacticGoalTodo) close(_ *ast.ASTManager) {
	clear(v.values)
	v.values = nil
}
func (g *Goal) quickProcess(saveFirst bool, f ast.ASTHandle, d ast.ASTDependency) (out ast.ASTHandle, err error) {
	out = f
	m := g.manager
	and, err := g.isOp(f, ast.ASTOpAnd)
	if err != nil {
		return out, err
	}
	negOr := false
	if child, not, e := g.notArgument(f); e != nil {
		return out, e
	} else if not {
		negOr, err = g.isOp(child, ast.ASTOpOr)
		if err != nil {
			return out, err
		}
	}
	if !and && !negOr {
		if !saveFirst {
			err = g.pushBack(f, ast.ASTHandle{}, d)
		}
		return out, err
	}
	var todo tacticGoalTodo
	defer todo.close(m)
	var tmp []ast.ASTHandle
	defer func() {
		for _, h := range tmp {
			err = errors.Join(err, m.DecRef(h))
		}
	}()
	if err = todo.push(m, tacticGoalPolarity{f, true}); err != nil {
		return out, err
	}
	for len(todo.values) != 0 {
		if g.inconsistent {
			return out, nil
		}
		last := len(todo.values) - 1
		p := todo.values[last]
		todo.values[last] = tacticGoalPolarity{}
		todo.values = todo.values[:last]
		curr := p.term
		split, err := g.isOp(curr, ast.ASTOpAnd)
		if err != nil {
			return out, err
		}
		if !p.positive {
			split, err = g.isOp(curr, ast.ASTOpOr)
			if err != nil {
				return out, err
			}
		}
		if split {
			n, e := m.GetNumArgs(curr)
			if e != nil {
				return out, e
			}
			for n > 0 {
				n--
				child, e := m.GetArg(curr, n)
				if e != nil {
					return out, e
				}
				if e = todo.push(m, tacticGoalPolarity{child, p.positive}); e != nil {
					return out, e
				}
			}
		} else if child, not, e := g.notArgument(curr); e != nil {
			return out, e
		} else if not {
			if e = todo.push(m, tacticGoalPolarity{child, !p.positive}); e != nil {
				return out, e
			}
		} else {
			if !p.positive {
				curr, err = m.MkNot(curr)
				if err != nil {
					return out, err
				}
				if err = m.IncRef(curr); err != nil {
					return out, err
				}
				tmp = append(tmp, curr)
			}
			if saveFirst {
				// The source assigns its expr_ref here, before later formulas
				// can clear the goal. Acquire before publishing the replacement.
				if curr != f {
					if err = m.IncRef(curr); err != nil {
						return out, err
					}
				}
				out = curr
				saveFirst = false
			} else if err = g.pushBack(curr, ast.ASTHandle{}, d); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}

// The proof-enabled source branch recursively applies AND/NOT-OR elimination.
// No polarity rewrite is substituted for its distinct proof construction path.
func (g *Goal) slowProcess(saveFirst bool, f, pr ast.ASTHandle, d ast.ASTDependency) (out, outProof ast.ASTHandle, err error) {
	m := g.manager
	and, err := g.isOp(f, ast.ASTOpAnd)
	if err != nil {
		return out, outProof, err
	}
	target := f
	notOr := false
	if !and {
		if child, not, e := g.notArgument(f); e != nil {
			return out, outProof, e
		} else if not {
			notOr, err = g.isOp(child, ast.ASTOpOr)
			target = child
			if err != nil {
				return out, outProof, err
			}
		}
	}
	if and || notOr {
		n, e := m.GetNumArgs(target)
		if e != nil {
			return out, outProof, e
		}
		for i := uint32(0); i < n; i++ {
			if g.inconsistent {
				return out, outProof, nil
			}
			child, e := m.GetArg(target, i)
			if e != nil {
				return out, outProof, e
			}
			var childProof ast.ASTHandle
			if and {
				childProof, e = m.MkProofAndElim(pr, i)
			} else {
				childProof, e = m.MkProofNotOrElim(pr, i)
			}
			if e != nil {
				return out, outProof, e
			}
			temporary := false
			if notOr {
				arg, not, e := g.notArgument(child)
				if e != nil {
					return out, outProof, e
				}
				if not {
					child = arg
				} else {
					child, e = m.MkNot(child)
					if e != nil {
						return out, outProof, e
					}
					if e = m.IncRef(child); e != nil {
						return out, outProof, e
					}
					temporary = true
				}
			}
			replacement, rp, e := g.slowProcess(saveFirst && i == 0, child, childProof, d)
			if temporary {
				e = errors.Join(e, m.DecRef(child))
			}
			if e != nil {
				return out, outProof, e
			}
			if saveFirst && i == 0 {
				out, outProof = replacement, rp
			}
		}
		return out, outProof, nil
	}
	if saveFirst {
		if err = m.IncRef(f); err != nil {
			return out, outProof, err
		}
		if err = m.IncRef(pr); err != nil {
			err = errors.Join(err, m.DecRef(f))
			return out, outProof, err
		}
		out, outProof = f, pr
	} else {
		err = g.pushBack(f, pr, d)
	}
	return out, outProof, err
}
func (g *Goal) Update(i uint32, f, pr ast.ASTHandle, d ast.ASTDependency) (err error) {
	if err = g.open(); err != nil {
		return err
	}
	if g.inconsistent {
		return nil
	}
	m := g.manager
	var out, rp ast.ASTHandle
	if g.proofsEnabled {
		out, rp, err = g.slowProcess(true, f, pr, d)
		defer func() { err = errors.Join(err, m.DecRef(out), m.DecRef(rp)) }()
		if err != nil {
			return err
		}
	} else {
		if err = m.IncRef(f); err != nil {
			return err
		}
		defer func() { err = errors.Join(err, m.DecRef(f)) }()
		out, err = g.quickProcess(true, f, d)
		if out != f {
			defer func() { err = errors.Join(err, m.DecRef(out)) }()
		}
		if err != nil {
			return err
		}
	}
	if g.inconsistent {
		return nil
	}
	falsity, err := m.MkFalse()
	if err != nil {
		return err
	}
	if out == falsity {
		if g.proofsEnabled {
			return g.pushBack(out, rp, d)
		}
		return g.pushBack(f, ast.ASTHandle{}, d)
	}
	if err = goalSetExpr(m, g.forms, i, out); err != nil {
		return err
	}
	if g.proofsEnabled {
		if err = goalSetExpr(m, g.proofs, i, rp); err != nil {
			return err
		}
	}
	if g.coreEnabled {
		return goalSetDependency(m, g.dependencies, i, d)
	}
	return nil
}
func (g *Goal) CopyTo(target *Goal) error {
	if e := g.open(); e != nil {
		return e
	}
	if e := target.open(); e != nil {
		return e
	}
	if g.manager != target.manager || g.proofsEnabled != target.proofsEnabled || g.coreEnabled != target.coreEnabled {
		return ast.ErrASTDomain
	}
	if g == target {
		return nil
	}
	m := g.manager
	if e := goalCopyExprs(m, g.forms, &target.forms); e != nil {
		return e
	}
	if e := goalCopyExprs(m, g.proofs, &target.proofs); e != nil {
		return e
	}
	if e := goalCopyDependencies(m, g.dependencies, &target.dependencies); e != nil {
		return e
	}
	if g.depth > target.depth {
		target.depth = g.depth
	}
	target.inconsistent = g.inconsistent
	target.precision = tacticGoalPrecisionUnion(g.precision, target.precision)
	return nil
}
func (g *Goal) shrink(j uint32) error {
	sz, e := g.Size()
	if e != nil {
		return e
	}
	if j > sz {
		return ast.ErrASTDomain
	}
	m := g.manager
	for i := j; i < sz; i++ {
		if e = goalPopExpr(m, &g.forms); e != nil {
			return e
		}
	}
	if g.proofsEnabled {
		for i := j; i < sz; i++ {
			if e = goalPopExpr(m, &g.proofs); e != nil {
				return e
			}
		}
	}
	if g.coreEnabled {
		for i := j; i < sz; i++ {
			if e = goalPopDependency(m, &g.dependencies); e != nil {
				return e
			}
		}
	}
	return nil
}
func (g *Goal) move(i, j uint32, f ast.ASTHandle) error {
	m := g.manager
	if e := goalSetExpr(m, g.forms, j, f); e != nil {
		return e
	}
	if g.proofsEnabled {
		pr, e := g.Proof(i)
		if e != nil {
			return e
		}
		if e = goalSetExpr(m, g.proofs, j, pr); e != nil {
			return e
		}
	}
	if g.coreEnabled {
		d, e := g.Dependency(i)
		if e != nil {
			return e
		}
		if e = goalSetDependency(m, g.dependencies, j, d); e != nil {
			return e
		}
	}
	return nil
}
func (g *Goal) ElimTrue() error {
	sz, e := g.Size()
	if e != nil {
		return e
	}
	truth, e := g.manager.MkTrue()
	if e != nil {
		return e
	}
	j := uint32(0)
	for i := uint32(0); i < sz; i++ {
		f, e := g.Form(i)
		if e != nil {
			return e
		}
		if f == truth {
			continue
		}
		if i != j {
			if e = g.move(i, j, f); e != nil {
				return e
			}
		}
		j++
	}
	return g.shrink(j)
}
func (g *Goal) getIndex(f ast.ASTHandle, negated bool) (uint32, error) {
	sz, e := g.Size()
	if e != nil {
		return 0, e
	}
	for i := uint32(0); i < sz; i++ {
		h, e := g.Form(i)
		if e != nil {
			return 0, e
		}
		if negated {
			arg, not, e := g.notArgument(h)
			if e != nil {
				return 0, e
			}
			if not && arg == f {
				return i, nil
			}
		} else if h == f {
			return i, nil
		}
	}
	return ^uint32(0), nil
}
func (g *Goal) ElimRedundancies() (err error) {
	if err = g.open(); err != nil {
		return err
	}
	if g.inconsistent {
		return nil
	}
	m := g.manager
	neg := make(map[ast.ASTHandle]bool)
	pos := make(map[ast.ASTHandle]bool)
	sz, err := g.Size()
	if err != nil {
		return err
	}
	j := uint32(0)
	truth, err := m.MkTrue()
	if err != nil {
		return err
	}
	for i := uint32(0); i < sz; i++ {
		f, e := g.Form(i)
		if e != nil {
			return e
		}
		if f == truth {
			continue
		}
		atom, not, e := g.notArgument(f)
		if e != nil {
			return e
		}
		if !not {
			atom = f
		}
		var duplicate, opposite bool
		if not {
			duplicate, opposite = neg[atom], pos[atom]
		} else {
			duplicate, opposite = pos[atom], neg[atom]
		}
		if duplicate {
			continue
		}
		if opposite {
			index, e := g.getIndex(atom, !not)
			if e != nil {
				return e
			}
			if index == ^uint32(0) {
				return ast.ErrASTDomain
			}
			var pr ast.ASTHandle
			if g.proofsEnabled {
				a, e := g.Proof(index)
				if e != nil {
					return e
				}
				b, e := g.Proof(i)
				if e != nil {
					return e
				}
				pr, e = m.MkProofUnitResolution([]ast.ASTHandle{a, b})
				if e != nil {
					return e
				}
			}
			var dep ast.ASTDependency
			if g.coreEnabled {
				a, e := g.Dependency(index)
				if e != nil {
					return e
				}
				b, e := g.Dependency(i)
				if e != nil {
					return e
				}
				dep, e = m.DependencyJoin(a, b)
				if e != nil {
					return e
				}
				if e = m.DependencyInc(dep); e != nil {
					return e
				}
				defer func() { err = errors.Join(err, m.DependencyDec(dep)) }()
			}
			falsity, e := m.MkFalse()
			if e != nil {
				return e
			}
			return g.pushBack(falsity, pr, dep)
		}
		if not {
			neg[atom] = true
		} else {
			pos[atom] = true
		}
		if i != j {
			if e = g.move(i, j, f); e != nil {
				return e
			}
		}
		j++
	}
	return g.shrink(j)
}
func (g *Goal) NumExprs() (total uint32, err error) {
	if err = g.open(); err != nil {
		return 0, err
	}
	visited := make(map[ast.ASTHandle]bool)
	n, err := g.Size()
	if err != nil {
		return 0, err
	}
	for i := uint32(0); i < n; i++ {
		h, e := g.Form(i)
		if e != nil {
			return total, e
		}
		count, e := g.manager.GetNumExprsWithVisited(h, visited)
		if e != nil {
			return total, e
		}
		total += count
	}
	return total, nil
}

// Source: src/tactic/goal.cpp:259-264. The returned handles are borrowed; the
// caller owns the output slice and must retain ASTs before outliving the goal.
func (g *Goal) AppendFormulas(dst []ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, e := g.Size()
	if e != nil {
		return dst, e
	}
	for i := uint32(0); i < n; i++ {
		h, e := g.Form(i)
		if e != nil {
			return dst, e
		}
		dst = append(dst, h)
	}
	return dst, nil
}
