package solver

// Source: src/solver/solver.h; src/solver/solver.cpp.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/messages"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Solver is the incremental solver surface used by the API, command context,
// interpolation layer and tactic adapters.
type Solver interface {
	CheckSatResult
	Close() error
	Translate(*ast.ASTManager, *uparams.ParamsRef) (Solver, error)
	UpdateParams(*uparams.ParamsRef) error
	Params() (*uparams.ParamsRef, error)
	CollectParamDescrs(*uparams.ParamDescrs) error
	SetProduceModels(bool) error
	Assert(ast.ASTHandle) error
	AssertTracked(ast.ASTHandle, ast.ASTHandle) error
	Push() error
	Pop(uint32) error
	ScopeLevel() (uint32, error)
	CheckSat([]ast.ASTHandle) (lbool.LBool, error)
	SetProgressCallback(ProgressCallback) error
	NumAssertions() (uint32, error)
	Assertion(uint32) (ast.ASTHandle, error)
	NumAssumptions() (uint32, error)
	Assumption(uint32) (ast.ASTHandle, error)
	GetConsequences([]ast.ASTHandle, []ast.ASTHandle) (lbool.LBool, []ast.ASTHandle, error)
	FindMutexes([]ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error)
	PreferredSAT([]ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error)
	Display(io.Writer, []ast.ASTHandle) error
}

// CandidateModelProvider is implemented by solvers that can distinguish an
// inconclusive check whose current assignment is suitable as a candidate
// model. PrepareCandidateModel publishes that model through AcquireModel when
// model production is enabled.
type CandidateModelProvider interface {
	PrepareCandidateModel() (bool, error)
}

type Factory interface {
	New(*ast.ASTManager, *uparams.ParamsRef, bool, bool, bool, string) (Solver, error)
	Close() error
}

type FactoryFunc func(*ast.ASTManager, *uparams.ParamsRef, bool, bool, bool, string) (Solver, error)

func (f FactoryFunc) New(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, logic string) (Solver, error) {
	if f == nil || m == nil {
		return nil, ast.ErrASTHandle
	}
	return f(m, p, proofs, models, cores, logic)
}
func (FactoryFunc) Close() error { return nil }

func Assertions(s Solver) ([]ast.ASTHandle, error) {
	if s == nil {
		return nil, ast.ErrASTHandle
	}
	n, err := s.NumAssertions()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		result[i], err = s.Assertion(i)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func DefaultDisplay(s Solver, out io.Writer, assumptions []ast.ASTHandle) error {
	if s == nil || out == nil {
		return ast.ErrASTHandle
	}
	m, err := s.Manager()
	if err != nil {
		return err
	}
	formulas, err := Assertions(s)
	if err != nil {
		return err
	}
	printer, err := ast.NewASTPPUtil(m)
	if err != nil {
		return err
	}
	if err = printer.CollectAll(formulas); err != nil {
		return err
	}
	if err = printer.CollectAll(assumptions); err != nil {
		return err
	}
	if err = printer.DisplayDeclarations(out); err != nil {
		return err
	}
	return printer.DisplayAssertions(out, formulas, true)
}

func DefaultFindMutexes([]ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error) {
	return lbool.True, nil, nil
}

func DefaultPreferredSAT(s Solver) (lbool.LBool, [][]ast.ASTHandle, error) {
	if s == nil {
		return lbool.Undef, nil, ast.ErrASTHandle
	}
	status, err := s.CheckSat(nil)
	return status, nil, err
}

// ScopedPush is the explicit Go counterpart of solver::scoped_push.
type ScopedPush struct {
	solver   Solver
	disabled bool
	closed   bool
}

func NewScopedPush(s Solver) (*ScopedPush, error) {
	if s == nil {
		return nil, ast.ErrASTHandle
	}
	if err := s.Push(); err != nil {
		return nil, err
	}
	return &ScopedPush{solver: s}, nil
}

func (p *ScopedPush) DisablePop() error {
	if p == nil || p.closed {
		return arena.ErrClosed
	}
	p.disabled = true
	return nil
}

func (p *ScopedPush) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	if p.disabled {
		return nil
	}
	return p.solver.Pop(1)
}

// ConsequenceEvaluator is the narrow model-evaluator operation used by the
// source default consequence algorithm.
type ConsequenceEvaluator interface {
	EvaluateValue(ast.ASTHandle) (ast.ASTRef, error)
	Close() error
}

type ConsequenceEvaluatorFactory func(*model.ModelCore) (ConsequenceEvaluator, error)

type consequenceOwner interface {
	RetainConsequence(ast.ASTHandle) (ast.ASTHandle, error)
}

func isUninterpretedConstant(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, err
	}
	decl, err := m.GetDecl(h)
	if err != nil {
		return false, err
	}
	family, err := m.GetFamilyID(decl)
	return family == ast.NullFamilyID, err
}

func IsLiteral(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	if m == nil {
		return false, ast.ErrASTHandle
	}
	constant, err := isUninterpretedConstant(m, h)
	if err != nil || constant {
		return constant, err
	}
	isNot, argument, err := m.IsNot(h)
	if err != nil || !isNot {
		return false, err
	}
	return isUninterpretedConstant(m, argument)
}

func isBoolean(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	sort, err := m.GetSort(h)
	if err != nil {
		return false, err
	}
	boolean, err := m.MkBoolSort()
	return sort == boolean, err
}

func appendConsequence(m *ast.ASTManager, core []ast.ASTHandle, literal ast.ASTHandle) (ast.ASTHandle, error) {
	antecedent, err := m.MkAndArray(core)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkImplies(antecedent, literal)
}

func retainConsequence(s Solver, consequence ast.ASTHandle) (ast.ASTHandle, error) {
	if owner, ok := s.(consequenceOwner); ok {
		return owner.RetainConsequence(consequence)
	}
	return consequence, nil
}

// DefaultGetConsequences preserves solver::get_consequences and its core
// algorithm. The factory makes the port's process services explicit.
func DefaultGetConsequences(s Solver, assumptions, variables []ast.ASTHandle, factory ConsequenceEvaluatorFactory) (status lbool.LBool, consequences []ast.ASTHandle, err error) {
	if s == nil || factory == nil {
		return lbool.Undef, nil, ast.ErrASTHandle
	}
	m, err := s.Manager()
	if err != nil {
		return lbool.Undef, nil, err
	}
	defer func() {
		if err == nil {
			return
		}
		canceled, check := m.Canceled()
		if check == nil && canceled {
			err = s.SetReasonUnknown(messages.Canceled)
			status = lbool.Undef
			return
		}
		_ = s.SetReasonUnknown(err.Error())
	}()
	status, err = s.CheckSat(assumptions)
	if err != nil || status != lbool.True {
		return status, nil, err
	}
	md, err := s.AcquireModel()
	if err != nil {
		return lbool.Undef, nil, err
	}
	if md == nil {
		return lbool.Undef, nil, errors.New("solver: satisfiable result has no model")
	}
	defer func() { err = errors.Join(err, md.DecRef()) }()
	evaluator, err := factory(md)
	if err != nil {
		return lbool.Undef, nil, err
	}
	defer func() { err = errors.Join(err, evaluator.Close()) }()
	manager, err := s.Manager()
	if err != nil {
		return lbool.Undef, nil, err
	}
	for _, variable := range variables {
		valueRef, e := evaluator.EvaluateValue(variable)
		if e != nil {
			return lbool.Undef, consequences, e
		}
		value, e := manager.RefGet(valueRef)
		if e != nil {
			_ = manager.DelRef(&valueRef)
			return lbool.Undef, consequences, e
		}
		isValue, e := manager.IsValue(value)
		if e != nil {
			return lbool.Undef, consequences, errors.Join(e, manager.DelRef(&valueRef))
		}
		if !isValue {
			if e = manager.DelRef(&valueRef); e != nil {
				return lbool.Undef, consequences, e
			}
			continue
		}
		boolean, e := isBoolean(manager, variable)
		if e != nil {
			return lbool.Undef, consequences, errors.Join(e, manager.DelRef(&valueRef))
		}
		constant, e := isUninterpretedConstant(manager, variable)
		if e != nil {
			return lbool.Undef, consequences, errors.Join(e, manager.DelRef(&valueRef))
		}
		var literal, negated ast.ASTHandle
		if boolean && constant {
			switch {
			case manager.IsTrue(value):
				literal = variable
				negated, e = manager.MkNot(variable)
			case manager.IsFalse(value):
				negated = variable
				literal, e = manager.MkNot(variable)
			default:
				e = manager.DelRef(&valueRef)
				if e != nil {
					return lbool.Undef, consequences, e
				}
				continue
			}
			e = errors.Join(e, manager.DelRef(&valueRef))
			if e != nil {
				return lbool.Undef, consequences, e
			}
			literalRef, e := manager.NewRef(literal)
			if e != nil {
				return lbool.Undef, consequences, e
			}
			negatedRef, e := manager.NewRef(negated)
			if e != nil {
				return lbool.Undef, consequences, errors.Join(e, manager.DelRef(&literalRef))
			}
			releaseLiterals := func(cause error) error {
				return errors.Join(cause, manager.DelRef(&negatedRef), manager.DelRef(&literalRef))
			}
			extended := append(append([]ast.ASTHandle(nil), assumptions...), negated)
			status, e = s.CheckSat(extended)
			if e != nil || status == lbool.Undef {
				return status, consequences, releaseLiterals(e)
			}
			if status == lbool.False {
				core, e := s.UnsatCore()
				if e != nil {
					return lbool.Undef, consequences, releaseLiterals(e)
				}
				filtered := core[:0]
				for _, item := range core {
					if item != negated {
						filtered = append(filtered, item)
					}
				}
				consequence, e := appendConsequence(manager, filtered, literal)
				if e != nil {
					return lbool.Undef, consequences, releaseLiterals(e)
				}
				consequence, e = retainConsequence(s, consequence)
				if e != nil {
					return lbool.Undef, consequences, releaseLiterals(e)
				}
				consequences = append(consequences, consequence)
			}
			if e = releaseLiterals(nil); e != nil {
				return lbool.Undef, consequences, e
			}
			continue
		}
		literal, e = manager.MkEq(variable, value)
		e = errors.Join(e, manager.DelRef(&valueRef))
		if e != nil {
			return lbool.Undef, consequences, fmt.Errorf("consequence equality: %w", e)
		}
		literalRef, e := manager.NewRef(literal)
		if e != nil {
			return lbool.Undef, consequences, fmt.Errorf("consequence equality root: %w", e)
		}
		negated, e = manager.MkNot(literal)
		if e != nil {
			return lbool.Undef, consequences, fmt.Errorf("consequence negation: %w", errors.Join(e, manager.DelRef(&literalRef)))
		}
		scope, e := NewScopedPush(s)
		if e != nil {
			return lbool.Undef, consequences, fmt.Errorf("consequence scope: %w", errors.Join(e, manager.DelRef(&literalRef)))
		}
		if e = s.Assert(negated); e == nil {
			status, e = s.CheckSat(assumptions)
		}
		if e == nil && status == lbool.False {
			core, e := s.UnsatCore()
			if e == nil {
				var consequence ast.ASTHandle
				consequence, e = appendConsequence(manager, core, literal)
				if e == nil {
					consequence, e = retainConsequence(s, consequence)
					if e == nil {
						consequences = append(consequences, consequence)
					}
				}
			}
		}
		e = errors.Join(e, scope.Close(), manager.DelRef(&literalRef))
		if e != nil {
			return lbool.Undef, consequences, fmt.Errorf("consequence query or cleanup: %w", e)
		}
		if status == lbool.Undef {
			return status, consequences, nil
		}
	}
	return lbool.True, consequences, nil
}
