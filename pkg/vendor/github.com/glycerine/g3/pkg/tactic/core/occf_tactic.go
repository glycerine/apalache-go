package core

// Source: src/tactic/core/occf_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type OCCFTactic struct {
	self       *OCCFTactic
	manager    *ast.ASTManager
	services   model.ModelEvaluatorConfig
	references uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*OCCFTactic)(nil)

func NewOCCFTactic(m *ast.ASTManager, services model.ModelEvaluatorConfig) (*OCCFTactic, error) {
	if m == nil || services.Cooperation == nil || services.Memory == nil {
		return nil, ast.ErrASTHandle
	}
	amount := uint64(reflect.TypeOf(OCCFTactic{}).Size())
	if err := m.Budget().Reserve(amount); err != nil {
		return nil, err
	}
	t := &OCCFTactic{manager: m, services: services, ownerBytes: amount}
	t.self = t
	return t, nil
}

func MakeOCCFTactic(m *ast.ASTManager, _ *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewOCCFTactic(m, services)
	if err != nil {
		return nil, err
	}
	clean, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return clean, nil
}

func (t *OCCFTactic) open() error {
	if t == nil || t.self != t || t.manager == nil || t.services.Cooperation == nil {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}

func (t *OCCFTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}

func (t *OCCFTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.references == 0 {
		return arena.ErrBounds
	}
	t.references--
	if t.references == 0 {
		return t.Close()
	}
	return nil
}

func (t *OCCFTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}

func (t *OCCFTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *OCCFTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *OCCFTactic) CollectStatistics() map[string]float64         { return nil }
func (t *OCCFTactic) ResetStatistics() error                        { return t.open() }
func (t *OCCFTactic) Cleanup() error                                { return t.open() }
func (t *OCCFTactic) Reset() error                                  { return t.Cleanup() }
func (t *OCCFTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *OCCFTactic) SetProgressCallback(func(uint32)) error        { return t.open() }

func (t *OCCFTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewOCCFTactic(m, t.services)
}

func (t *OCCFTactic) checkpoint() error {
	canceled, err := t.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(t.manager.ResourceLimit().CancelMessage())
	}
	return t.services.Cooperation.Cooperate("occf")
}

func (t *OCCFTactic) isUninterpretedConstant(term ast.ASTHandle) (bool, error) {
	kind, err := t.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	count, err := t.manager.GetNumArgs(term)
	if err != nil || count != 0 {
		return false, err
	}
	decl, err := t.manager.GetDecl(term)
	if err != nil {
		return false, err
	}
	family, err := t.manager.GetFamilyID(decl)
	return family == ast.NullFamilyID, err
}

func (t *OCCFTactic) isLiteral(term ast.ASTHandle) (bool, error) {
	constant, err := t.isUninterpretedConstant(term)
	if err != nil || constant {
		return constant, err
	}
	not, child, err := t.manager.IsNot(term)
	if err != nil || !not {
		return false, err
	}
	return t.isUninterpretedConstant(child)
}

func (t *OCCFTactic) isClause(term ast.ASTHandle) (bool, error) {
	kind, err := t.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := t.manager.GetFamilyID(term)
	if err != nil || family != t.manager.BasicFamilyID() {
		return false, err
	}
	op, err := t.manager.GetDeclKind(term)
	return op == ast.ASTOpOr, err
}

func (t *OCCFTactic) isTarget(clause ast.ASTHandle) (bool, error) {
	count, err := t.manager.GetNumArgs(clause)
	if err != nil {
		return false, err
	}
	found := false
	for i := uint32(0); i < count; i++ {
		argument, getErr := t.manager.GetArg(clause, i)
		if getErr != nil {
			return false, getErr
		}
		literal, getErr := t.isLiteral(argument)
		if getErr != nil {
			return false, getErr
		}
		if !literal {
			if found {
				return true, nil
			}
			found = true
		}
	}
	return false, nil
}

type occfBooleanInfo struct {
	variable          ast.ASTHandle
	generatedPositive bool
	generatedNegative bool
}

func (t *OCCFTactic) stripNot(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	sign := false
	for {
		not, child, err := t.manager.IsNot(term)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if !not {
			return term, sign, nil
		}
		term, sign = child, !sign
	}
}

func (t *OCCFTactic) assertDefinition(goal *tactic.Goal, left, right ast.ASTHandle) error {
	clause, err := t.manager.MkOr(left, right)
	if err != nil {
		return err
	}
	return goal.Assert(clause)
}

func (t *OCCFTactic) getAuxiliary(cache map[ast.ASTHandle]*occfBooleanInfo, constraint ast.ASTHandle, goal *tactic.Goal) (ast.ASTHandle, bool, error) {
	base, sign, err := t.stripNot(constraint)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	info := cache[base]
	if info == nil {
		return ast.ASTHandle{}, false, nil
	}
	if sign {
		if !info.generatedNegative {
			info.generatedNegative = true
			notBase, getErr := t.manager.MkNot(base)
			if getErr != nil {
				return ast.ASTHandle{}, false, getErr
			}
			if getErr = t.assertDefinition(goal, info.variable, notBase); getErr != nil {
				return ast.ASTHandle{}, false, getErr
			}
		}
		negated, getErr := t.manager.MkNot(info.variable)
		return negated, true, getErr
	}
	if !info.generatedPositive {
		info.generatedPositive = true
		negated, getErr := t.manager.MkNot(info.variable)
		if getErr != nil {
			return ast.ASTHandle{}, false, getErr
		}
		if getErr = t.assertDefinition(goal, negated, base); getErr != nil {
			return ast.ASTHandle{}, false, getErr
		}
	}
	return info.variable, true, nil
}

func (t *OCCFTactic) makeAuxiliary(cache map[ast.ASTHandle]*occfBooleanInfo, constraint ast.ASTHandle, models bool, goal *tactic.Goal, converter *tactic.FilterModelConverter) (ast.ASTHandle, error) {
	base, sign, err := t.stripNot(constraint)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	boolean, err := t.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	variable, err := t.manager.MkFreshConstNullable(nil, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if models {
		decl, getErr := t.manager.GetDecl(variable)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if getErr = converter.Insert(decl); getErr != nil {
			return ast.ASTHandle{}, getErr
		}
	}
	info := &occfBooleanInfo{variable: variable, generatedPositive: !sign, generatedNegative: sign}
	cache[base] = info
	if sign {
		notBase, getErr := t.manager.MkNot(base)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if getErr = t.assertDefinition(goal, variable, notBase); getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		return t.manager.MkNot(variable)
	}
	negated, err := t.manager.MkNot(variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.assertDefinition(goal, negated, base); err != nil {
		return ast.ASTHandle{}, err
	}
	return variable, nil
}

func (t *OCCFTactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if goal == nil || out == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err := tactic.FailIfProofGeneration("occf", goal); err != nil {
		return err
	}
	if sorted, err := goal.IsWellSorted(); err != nil {
		return err
	} else if !sorted {
		return ast.ErrASTDomain
	}
	if err := out.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("occf", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { resultErr = errors.Join(resultErr, report.Close()) }()
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	cache := make(map[ast.ASTHandle]*occfBooleanInfo)
	var converter *tactic.FilterModelConverter
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		if err = t.checkpoint(); err != nil {
			return err
		}
		formula, getErr := goal.Form(i)
		if getErr != nil {
			return getErr
		}
		clause, getErr := t.isClause(formula)
		if getErr != nil {
			return getErr
		}
		if !clause {
			continue
		}
		target, getErr := t.isTarget(formula)
		if getErr != nil {
			return getErr
		}
		if !target {
			continue
		}
		if models && converter == nil {
			converter, getErr = tactic.NewFilterModel(t.manager)
			if getErr != nil {
				return getErr
			}
			if getErr = out.SetModelConverter(converter); getErr != nil {
				return errors.Join(getErr, converter.Close())
			}
		}
		dependency, getErr := goal.Dependency(i)
		if getErr != nil {
			return getErr
		}
		count, getErr := t.manager.GetNumArgs(formula)
		if getErr != nil {
			return getErr
		}
		var keep ast.ASTHandle
		literals := make([]ast.ASTHandle, 0, count)
		for j := uint32(0); j < count; j++ {
			literal, getErr := t.manager.GetArg(formula, j)
			if getErr != nil {
				return getErr
			}
			isLiteral, getErr := t.isLiteral(literal)
			if getErr != nil {
				return getErr
			}
			if isLiteral {
				literals = append(literals, literal)
				continue
			}
			auxiliary, present, getErr := t.getAuxiliary(cache, literal, goal)
			if getErr != nil {
				return getErr
			}
			if present {
				literals = append(literals, auxiliary)
			} else if keep.IsNull() {
				keep = literal
			} else {
				auxiliary, getErr = t.makeAuxiliary(cache, literal, models, goal, converter)
				if getErr != nil {
					return getErr
				}
				literals = append(literals, auxiliary)
			}
		}
		if !keep.IsNull() {
			literals = append(literals, keep)
		}
		rewritten, getErr := t.manager.MkOrArgs(literals)
		if getErr != nil {
			return getErr
		}
		if getErr = goal.Update(i, rewritten, ast.ASTHandle{}, dependency); getErr != nil {
			return getErr
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if sorted, checkErr := goal.IsWellSorted(); checkErr != nil {
		return checkErr
	} else if !sorted {
		return ast.ErrASTDomain
	}
	return out.AddGoal(goal)
}
