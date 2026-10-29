package solver

// Source: src/solver/solver_na2as.h; src/solver/solver_na2as.cpp.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// NamedAssertionCore is the concrete derived-solver surface invoked by the
// named-assertion-to-assumption adapter.
type NamedAssertionCore interface {
	AssertFormula(ast.ASTHandle) error
	CheckSatCore([]ast.ASTHandle) (lbool.LBool, error)
	PushCore() error
	PopCore(uint32) error
	DisplayCore(io.Writer) error
}

type NamedAssertions struct {
	self        *NamedAssertions
	manager     *ast.ASTManager
	core        NamedAssertionCore
	trace       *diagnostic.TraceTags
	assumptions []ast.ASTRef
	scopes      []uint32
	pending     []*namedAssertionScope
	closing     bool
	closed      bool
}

func NewNamedAssertions(m *ast.ASTManager, core NamedAssertionCore) (*NamedAssertions, error) {
	return NewNamedAssertionsWithTrace(m, core, nil)
}

func NewNamedAssertionsWithTrace(m *ast.ASTManager, core NamedAssertionCore, trace *diagnostic.TraceTags) (*NamedAssertions, error) {
	if m == nil || core == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	if trace == nil {
		trace = diagnostic.BorrowProcessTraceTags()
	}
	n := &NamedAssertions{manager: m, core: core, trace: trace}
	n.self = n
	return n, nil
}

func (n *NamedAssertions) open() error {
	if n == nil || n.self != n {
		return ast.ErrASTHandle
	}
	if n.closed {
		return arena.ErrClosed
	}
	if n.closing || len(n.pending) != 0 {
		return ast.ErrASTPending
	}
	_, err := n.manager.ProofsEnabled()
	return err
}

func namedAssertionFormulaText(m *ast.ASTManager, h ast.ASTHandle, out io.Writer) error {
	printer, err := ast.NewSimpleASTPrinterContext(m, out, nil)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, h, 0)
}

func (n *NamedAssertions) traceAssert(formula, tag ast.ASTHandle) error {
	tags := n.trace
	var operandErr error
	_ = tags.EmitConditional("solver_na2as", "assert_expr", "src/solver/solver_na2as.cpp", 38, func() bool { return true }, func(w io.Writer) error {
		if _, err := io.WriteString(w, "asserting\n"); err != nil {
			return err
		}
		if operandErr = namedAssertionFormulaText(n.manager, formula, w); operandErr != nil {
			return operandErr
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
		if operandErr = namedAssertionFormulaText(n.manager, tag, w); operandErr != nil {
			return operandErr
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
	return operandErr
}

func (n *NamedAssertions) AssertTracked(formula, tag ast.ASTHandle) (err error) {
	if err := n.open(); err != nil {
		return err
	}
	if tag.IsNull() {
		return n.core.AssertFormula(formula)
	}
	constant, err := isUninterpretedConstant(n.manager, tag)
	if err != nil {
		return err
	}
	boolean, err := isBoolean(n.manager, tag)
	if err != nil {
		return err
	}
	if !constant || !boolean {
		return ast.ErrASTDomain
	}
	if err = n.traceAssert(formula, tag); err != nil {
		return err
	}
	ref, err := n.manager.NewRef(tag)
	if err != nil {
		return err
	}
	n.assumptions = append(n.assumptions, ref)
	scope := &namedAssertionScope{manager: n.manager}
	defer n.finishAssertion(scope, &err)
	implies, err := n.manager.MkImplies(tag, formula)
	if err != nil {
		return err
	}
	scope.implies, err = n.manager.NewRef(implies)
	if err != nil {
		return err
	}
	return n.core.AssertFormula(implies)
}

func (n *NamedAssertions) assumptionHandles() ([]ast.ASTHandle, error) {
	values := make([]ast.ASTHandle, len(n.assumptions))
	for i := range n.assumptions {
		value, err := n.manager.RefGet(n.assumptions[i])
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

func (n *NamedAssertions) appendTemporary(values []ast.ASTHandle) (uint32, error) {
	old := uint32(len(n.assumptions))
	for _, value := range values {
		ref, err := n.manager.NewRef(value)
		if err != nil {
			return old, err
		}
		n.assumptions = append(n.assumptions, ref)
	}
	return old, nil
}

func (n *NamedAssertions) restore(size uint32) error {
	if uint64(size) > uint64(len(n.assumptions)) {
		return arena.ErrBounds
	}
	// ref_vector_core::shrink decrements the removed range forward. Keep
	// reached zero tokens until all remaining elements have retired so a
	// stopped Go cleanup can continue without replaying an earlier decrement.
	for i := int(size); i < len(n.assumptions); i++ {
		if n.assumptions[i] == (ast.ASTRef{}) {
			continue
		}
		if err := n.manager.DelRef(&n.assumptions[i]); err != nil {
			n.closing = true
			return err
		}
	}
	n.assumptions = n.assumptions[:size]
	return nil
}

func (n *NamedAssertions) CheckSat(extra []ast.ASTHandle) (status lbool.LBool, err error) {
	if err = n.open(); err != nil {
		return lbool.Undef, err
	}
	old, err := n.appendTemporary(extra)
	if err != nil {
		return lbool.Undef, errors.Join(err, n.restore(old))
	}
	defer func() { err = errors.Join(err, n.restore(old)) }()
	values, err := n.assumptionHandles()
	if err != nil {
		return lbool.Undef, err
	}
	tags := n.trace
	var operandErr error
	_ = tags.EmitConditional("solver_na2as", "check_sat", "src/solver/solver_na2as.cpp", 61, func() bool { return true }, func(w io.Writer) error {
		operandErr = n.core.DisplayCore(w)
		return operandErr
	})
	if operandErr != nil {
		return lbool.Undef, operandErr
	}
	return n.core.CheckSatCore(values)
}

func (n *NamedAssertions) WithAssumptions(extra []ast.ASTHandle, operation func([]ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error)) (status lbool.LBool, consequences []ast.ASTHandle, err error) {
	if err = n.open(); err != nil || operation == nil {
		if err == nil {
			err = ast.ErrASTHandle
		}
		return lbool.Undef, nil, err
	}
	old, err := n.appendTemporary(extra)
	if err != nil {
		return lbool.Undef, nil, errors.Join(err, n.restore(old))
	}
	defer func() { err = errors.Join(err, n.restore(old)) }()
	values, err := n.assumptionHandles()
	if err != nil {
		return lbool.Undef, nil, err
	}
	return operation(values)
}

func (n *NamedAssertions) Push() error {
	if err := n.open(); err != nil {
		return err
	}
	n.scopes = append(n.scopes, uint32(len(n.assumptions)))
	return n.core.PushCore()
}

func (n *NamedAssertions) Pop(count uint32) error {
	if err := n.open(); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if err := n.core.PopCore(count); err != nil {
		return err
	}
	level := uint32(len(n.scopes))
	if count > level {
		return arena.ErrBounds
	}
	newLevel := level - count
	if err := n.restore(n.scopes[newLevel]); err != nil {
		return err
	}
	n.scopes = n.scopes[:newLevel]
	return nil
}

func (n *NamedAssertions) ScopeLevel() (uint32, error) {
	if err := n.open(); err != nil {
		return 0, err
	}
	return uint32(len(n.scopes)), nil
}

func (n *NamedAssertions) NumAssumptions() (uint32, error) {
	if err := n.open(); err != nil {
		return 0, err
	}
	return uint32(len(n.assumptions)), nil
}

func (n *NamedAssertions) Assumption(index uint32) (ast.ASTHandle, error) {
	if err := n.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(index) >= uint64(len(n.assumptions)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return n.manager.RefGet(n.assumptions[index])
}

func (n *NamedAssertions) Close() error {
	if n == nil || n.self != n {
		return ast.ErrASTHandle
	}
	if n.closed {
		return nil
	}
	n.closing = true
	// The source assertion local must retire before member assumptions.
	for len(n.pending) != 0 {
		if err := n.pending[0].close(); err != nil {
			return err
		}
		n.pending = n.pending[1:]
	}
	if err := n.restore(0); err != nil {
		return err
	}
	n.scopes = nil
	n.closed = true
	return nil
}

// Source solver_na2as::assert_expr new_t, owned across the virtual call.
type namedAssertionScope struct {
	manager *ast.ASTManager
	implies ast.ASTRef
}

func (s *namedAssertionScope) close() error {
	if s.implies == (ast.ASTRef{}) {
		return nil
	}
	return s.manager.DelRef(&s.implies)
}
func (n *NamedAssertions) finishAssertion(scope *namedAssertionScope, err *error) {
	if cleanup := scope.close(); cleanup != nil {
		n.pending = append(n.pending, scope)
		*err = errors.Join(*err, cleanup)
	}
}
