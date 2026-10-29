package interp

// Source checkpoint: src/interp/iz3base.cpp and src/interp/iz3base.h core state.

import (
	"errors"
	"io"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

type baseRanges struct {
	rangeValue    ScopeRange
	scope         ScopeRange
	scopeComputed bool
}

// Base is the retained shared interpolation state. It borrows Manager and owns
// references for constraints, theory formulas, and memoized simplifications.
type Base struct {
	self        *Base
	manager     *Manager
	scopes      *Scopes
	constraints []ast.ASTHandle
	theory      []ast.ASTHandle
	owned       []ast.ASTRef
	symbolRange map[ast.ASTHandle]ScopeRange
	astRanges   map[ast.ASTHandle]*baseRanges
	simplified  map[ast.ASTHandle]ast.ASTHandle
	frames      map[ast.ASTHandle]int
	weak        bool
	solver      solver.Factory
	evaluator   solver.ConsequenceEvaluatorFactory
	closed      bool
}

func (b *Base) ConfigureSolver(factory solver.Factory, evaluator solver.ConsequenceEvaluatorFactory) error {
	if err := b.open(); err != nil {
		return err
	}
	if factory == nil {
		return ast.ErrASTHandle
	}
	b.solver, b.evaluator = factory, evaluator
	return nil
}

func NewBase(manager *Manager, constraints []ast.ASTHandle, parents []int, theory []ast.ASTHandle) (result *Base, resultErr error) {
	if manager == nil || manager.open() != nil {
		return nil, ast.ErrASTHandle
	}
	scopes, err := NewScopes(parents)
	if err != nil {
		return nil, err
	}
	b := &Base{manager: manager, scopes: scopes, symbolRange: make(map[ast.ASTHandle]ScopeRange), astRanges: make(map[ast.ASTHandle]*baseRanges), simplified: make(map[ast.ASTHandle]ast.ASTHandle), frames: make(map[ast.ASTHandle]int)}
	b.self = b
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, b.Close())
		}
	}()
	if err = b.initialize(constraints, theory); err != nil {
		return nil, err
	}
	return b, nil
}

func NewBaseFromParts(manager *Manager, parts [][]ast.ASTHandle, parents []int, theory []ast.ASTHandle) (result *Base, resultErr error) {
	constraints := make([]ast.ASTHandle, len(parts))
	for i := range parts {
		conjunction, err := manager.Make(OperatorAnd, parts[i]...)
		if err != nil {
			return nil, err
		}
		constraints[i] = conjunction
	}
	result, err := NewBase(manager, constraints, parents, theory)
	if err != nil {
		return nil, err
	}
	for frame := range parts {
		for _, formula := range parts[frame] {
			if err = result.addFrameRange(frame, formula); err != nil {
				return nil, errors.Join(err, result.Close())
			}
			result.frames[formula] = frame
		}
	}
	return result, nil
}

func (b *Base) open() error {
	if b == nil || b.self != b || b.closed || b.manager == nil {
		return ast.ErrASTHandle
	}
	return b.manager.open()
}

func (b *Base) hold(handle ast.ASTHandle) error {
	ref, err := b.manager.ast.NewRef(handle)
	if err != nil {
		return err
	}
	b.owned = append(b.owned, ref)
	return nil
}

func (b *Base) initialize(constraints, theory []ast.ASTHandle) error {
	b.constraints = append([]ast.ASTHandle(nil), constraints...)
	b.theory = append([]ast.ASTHandle(nil), theory...)
	for i, formula := range b.constraints {
		if err := b.hold(formula); err != nil {
			return err
		}
		if err := b.addFrameRange(i, formula); err != nil {
			return err
		}
		b.frames[formula] = i
	}
	for _, formula := range b.theory {
		if err := b.hold(formula); err != nil {
			return err
		}
		if err := b.addFrameRange(ScopeBottom, formula); err != nil {
			return err
		}
		if err := b.addFrameRange(ScopeTop, formula); err != nil {
			return err
		}
		b.frames[formula] = math.MaxInt
	}
	return nil
}

func (b *Base) Close() (result error) {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	for i := len(b.owned) - 1; i >= 0; i-- {
		result = errors.Join(result, b.manager.ast.DelRef(&b.owned[i]))
	}
	b.owned = nil
	b.closed = true
	return result
}

func (b *Base) Manager() (*Manager, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	return b.manager, nil
}

func (b *Base) Scopes() (*Scopes, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	return b.scopes, nil
}

func (b *Base) SetOption(name, value string) error {
	if err := b.open(); err != nil {
		return err
	}
	if name == "weak" && value == "1" {
		b.weak = true
	}
	return nil
}

func (b *Base) WeakMode() (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	return b.weak, nil
}

func (b *Base) astRangeEntry(handle ast.ASTHandle) *baseRanges {
	entry := b.astRanges[handle]
	if entry == nil {
		entry = &baseRanges{rangeValue: EmptyScopeRange()}
		b.astRanges[handle] = entry
	}
	return entry
}

func (b *Base) addFrameRange(frame int, term ast.ASTHandle) error {
	entry := b.astRangeEntry(term)
	inside, err := b.scopes.InRange(frame, entry.rangeValue)
	if err != nil {
		return err
	}
	if inside {
		return nil
	}
	if err = b.scopes.RangeAdd(frame, &entry.rangeValue); err != nil {
		return err
	}
	count, err := b.manager.NumArgs(term)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		child, e := b.manager.Arg(term, i)
		if e != nil {
			return e
		}
		if e = b.addFrameRange(frame, child); e != nil {
			return e
		}
	}
	if op, err := b.manager.Operator(term); err != nil {
		return err
	} else if op == OperatorUninterpreted {
		declaration, e := b.manager.Symbol(term)
		if e != nil {
			return e
		}
		r, found := b.symbolRange[declaration]
		if !found {
			r = EmptyScopeRange()
		}
		if e = b.scopes.RangeAdd(frame, &r); e != nil {
			return e
		}
		b.symbolRange[declaration] = r
	}
	return nil
}

func (b *Base) ASTRange(term ast.ASTHandle) (ScopeRange, error) {
	if err := b.open(); err != nil {
		return ScopeRange{}, err
	}
	return b.astRangeEntry(term).rangeValue, nil
}

func (b *Base) SymbolRange(declaration ast.ASTHandle) (ScopeRange, error) {
	if err := b.open(); err != nil {
		return ScopeRange{}, err
	}
	r, found := b.symbolRange[declaration]
	if !found {
		return EmptyScopeRange(), nil
	}
	return r, nil
}

func (b *Base) ASTScope(term ast.ASTHandle) (ScopeRange, error) {
	if err := b.open(); err != nil {
		return ScopeRange{}, err
	}
	entry := b.astRangeEntry(term)
	if entry.scopeComputed {
		return entry.scope, nil
	}
	rangeValue := FullScopeRange()
	count, err := b.manager.NumArgs(term)
	if err != nil {
		return ScopeRange{}, err
	}
	for i := uint32(0); i < count; i++ {
		child, e := b.manager.Arg(term, i)
		if e != nil {
			return ScopeRange{}, e
		}
		childScope, e := b.ASTScope(child)
		if e != nil {
			return ScopeRange{}, e
		}
		rangeValue, e = b.scopes.RangeGLB(rangeValue, childScope)
		if e != nil {
			return ScopeRange{}, e
		}
	}
	op, err := b.manager.Operator(term)
	if err != nil {
		return ScopeRange{}, err
	}
	if op == OperatorUninterpreted && (!b.scopes.TreeMode() || count == 0) {
		declaration, e := b.manager.Symbol(term)
		if e != nil {
			return ScopeRange{}, e
		}
		symbolScope, e := b.SymbolRange(declaration)
		if e != nil {
			return ScopeRange{}, e
		}
		rangeValue, e = b.scopes.RangeGLB(rangeValue, symbolScope)
		if e != nil {
			return ScopeRange{}, e
		}
	}
	entry.scope, entry.scopeComputed = rangeValue, true
	return rangeValue, nil
}

func (b *Base) IsLocal(term ast.ASTHandle) (bool, error) {
	r, err := b.ASTScope(term)
	return !r.IsEmpty(), err
}

func (b *Base) FrameOfAssertion(assertion ast.ASTHandle) (int, error) {
	if err := b.open(); err != nil {
		return 0, err
	}
	frame, found := b.frames[assertion]
	if !found {
		return 0, NewError("unknown assertion")
	}
	return frame, nil
}

func (b *Base) ProofCheckAssumption(scope ScopeRange) (ast.ASTHandle, error) {
	if err := b.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if scope.Hi < 0 || scope.Hi >= len(b.constraints) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	values := append([]ast.ASTHandle(nil), b.theory...)
	values = append(values, b.constraints[scope.Hi])
	return b.manager.Make(OperatorAnd, values...)
}

func (b *Base) Print(out io.Writer) error {
	if err := b.open(); err != nil {
		return err
	}
	formula, err := b.manager.Make(OperatorAnd, b.constraints...)
	if err != nil {
		return err
	}
	return b.manager.PrintSATProblem(out, formula)
}
