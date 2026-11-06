package smtcore

// Source: src/smt/theory_wmaxsat.cpp weighted slack variables, exact cost
// bounds, conflict construction, and blocking clauses.

import (
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtWMaxVariable struct {
	boolean int32
	expr    ast.ASTHandle
	weight  big.Rat
	enabled bool
}

// SMTWeightedMaxTheory tracks the Boolean slack variables used by src/opt/wmax.cpp.
// The SAT kernel invokes Assign on the same variable that receives an SMT
// theory assignment, so conflicts are explained by the currently true slack
// variables exactly as in theory_wmaxsat::block.
type SMTWeightedMaxTheory struct {
	*SMTTheoryBase
	filter       *tactic.FilterModelConverter
	variables    []smtWMaxVariable
	byBoolean    map[int32]int
	minimumCost  big.Rat
	foundOptimal bool
	costSave     []int
	scopeLimits  []int
	modelCosts   []int
	modelCost    big.Rat
	modelValid   bool
}

func NewSMTWeightedMaxTheory(family ast.FamilyID, filter *tactic.FilterModelConverter) (*SMTWeightedMaxTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	if filter == nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTWeightedMaxTheory{SMTTheoryBase: base, filter: filter, byBoolean: make(map[int32]int)}, nil
}

func (t *SMTWeightedMaxTheory) AddWeighted(formula ast.ASTHandle, weight *big.Rat) (ast.ASTHandle, ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if weight == nil || weight.Sign() <= 0 {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ErrASTDomain
	}
	manager := state.manager
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	slack, err := manager.MkFreshConst("w", boolean)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	slackRef, err := manager.NewRef(slack)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	defer manager.DelRef(&slackRef)
	declaration, err := manager.GetDecl(slack)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = t.filter.Insert(declaration); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	variable, err := state.MkBoolVar(slack)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	assertion, err := manager.MkOr(slack, formula)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	entry := smtWMaxVariable{boolean: variable, expr: slack, enabled: true}
	entry.weight.Set(weight)
	t.byBoolean[variable] = len(t.variables)
	t.variables = append(t.variables, entry)
	return slack, assertion, nil
}

func (t *SMTWeightedMaxTheory) InitMinCost(value *big.Rat) error {
	if value == nil || value.Sign() < 0 {
		return ast.ErrASTDomain
	}
	t.minimumCost.Set(value)
	t.modelCosts = nil
	t.modelCost.SetInt64(0)
	t.modelValid = false
	return nil
}

func (t *SMTWeightedMaxTheory) currentCosts() ([]int, big.Rat, error) {
	state, err := t.State()
	if err != nil {
		return nil, big.Rat{}, err
	}
	indices := make([]int, 0, len(t.variables))
	var total big.Rat
	for i := range t.variables {
		entry := &t.variables[i]
		if !entry.enabled {
			continue
		}
		value, cause := state.Assignment(NewSMTLiteral(entry.boolean, false))
		if cause != nil {
			return nil, big.Rat{}, cause
		}
		if value == lbool.True {
			indices = append(indices, i)
			total.Add(&total, &entry.weight)
		}
	}
	return indices, total, nil
}

func (t *SMTWeightedMaxTheory) Assign(variable int32, truth bool) error {
	if !truth {
		return nil
	}
	if _, found := t.byBoolean[variable]; !found {
		return ast.ErrASTDomain
	}
	indices, cost, err := t.currentCosts()
	if err != nil || cost.Cmp(&t.minimumCost) < 0 {
		return err
	}
	return t.setCostConflict(indices)
}

func (t *SMTWeightedMaxTheory) setCostConflict(indices []int) error {
	sort.SliceStable(indices, func(i, j int) bool {
		return t.variables[indices[i]].weight.Cmp(&t.variables[indices[j]].weight) < 0
	})
	state, err := t.State()
	if err != nil {
		return err
	}
	var weight big.Rat
	literals := make([]SMTLiteral, 0, len(indices))
	for _, index := range indices {
		entry := &t.variables[index]
		literals = append(literals, NewSMTLiteral(entry.boolean, false))
		weight.Add(&weight, &entry.weight)
		if weight.Cmp(&t.minimumCost) >= 0 {
			break
		}
	}
	return state.SetTheoryConflict(t.FamilyID(), literals)
}

func (t *SMTWeightedMaxTheory) Cost() (*big.Rat, error) {
	if t.modelValid {
		return new(big.Rat).Set(&t.modelCost), nil
	}
	_, value, err := t.currentCosts()
	return &value, err
}

func (t *SMTWeightedMaxTheory) IsOptimal() (bool, error) {
	if !t.foundOptimal {
		return true, nil
	}
	cost, err := t.Cost()
	return err == nil && cost.Cmp(&t.minimumCost) < 0, err
}

func (t *SMTWeightedMaxTheory) MakeBlock() (ast.ASTHandle, error) {
	indices := append([]int(nil), t.modelCosts...)
	if !t.modelValid {
		var err error
		indices, _, err = t.currentCosts()
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	sort.SliceStable(indices, func(i, j int) bool {
		return t.variables[indices[i]].weight.Cmp(&t.variables[indices[j]].weight) < 0
	})
	var weight big.Rat
	literals := make([]ast.ASTHandle, 0, len(indices))
	refs := make([]ast.ASTRef, 0, len(indices))
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			_ = t.state.manager.DelRef(&refs[i])
		}
	}()
	for _, index := range indices {
		entry := &t.variables[index]
		negated, cause := t.state.manager.MkNot(entry.expr)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		ref, cause := t.state.manager.NewRef(negated)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		refs = append(refs, ref)
		literals = append(literals, negated)
		weight.Add(&weight, &entry.weight)
		if weight.Cmp(&t.minimumCost) >= 0 {
			break
		}
	}
	if optimal, cause := t.IsOptimal(); cause != nil {
		return ast.ASTHandle{}, cause
	} else if optimal {
		t.foundOptimal = true
		t.costSave = append(t.costSave[:0], indices...)
	}
	return t.state.manager.MkOrArgs(literals)
}

func (t *SMTWeightedMaxTheory) DisableVariable(expression ast.ASTHandle) error {
	for i := range t.variables {
		if t.variables[i].expr == expression {
			t.variables[i].enabled = false
			return nil
		}
	}
	return ast.ErrASTDomain
}

func (t *SMTWeightedMaxTheory) ResetLocal() error {
	if _, err := t.State(); err != nil {
		return err
	}
	t.variables = nil
	t.byBoolean = make(map[int32]int)
	t.minimumCost.SetInt64(0)
	t.foundOptimal = false
	t.costSave = nil
	t.modelCosts = nil
	t.modelCost.SetInt64(0)
	t.modelValid = false
	for i := range t.scopeLimits {
		t.scopeLimits[i] = 0
	}
	return nil
}

func (t *SMTWeightedMaxTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.scopeLimits = append(t.scopeLimits, len(t.variables))
	return nil
}

func (t *SMTWeightedMaxTheory) PopScopes(scopes uint32) error {
	if scopes == 0 || scopes > uint32(len(t.scopeLimits)) {
		return ast.ErrASTDomain
	}
	level := len(t.scopeLimits) - int(scopes)
	limit := t.scopeLimits[level]
	if err := t.SMTTheoryBase.PopScopes(scopes); err != nil {
		return err
	}
	for i := limit; i < len(t.variables); i++ {
		delete(t.byBoolean, t.variables[i].boolean)
	}
	t.variables = t.variables[:limit]
	t.scopeLimits = t.scopeLimits[:level]
	return nil
}

func (*SMTWeightedMaxTheory) InternalizeAtom(ast.ASTHandle, bool) (bool, error) { return false, nil }
func (*SMTWeightedMaxTheory) InternalizeEquality(ast.ASTHandle, int32) error    { return nil }
func (*SMTWeightedMaxTheory) InternalizeTerm(ast.ASTHandle) (bool, error)       { return false, nil }
func (*SMTWeightedMaxTheory) ApplySortConstraint(*SMTEnode, ast.ASTHandle) error {
	return nil
}
func (*SMTWeightedMaxTheory) UseDisequalities() bool                          { return false }
func (*SMTWeightedMaxTheory) NewEquality(SMTTheoryVar, SMTTheoryVar) error    { return nil }
func (*SMTWeightedMaxTheory) NewDisequality(SMTTheoryVar, SMTTheoryVar) error { return nil }

func (t *SMTWeightedMaxTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	indices, cost, err := t.currentCosts()
	if err != nil {
		return SMTFinalCheckGiveUp, err
	}
	t.modelCosts = append(t.modelCosts[:0], indices...)
	t.modelCost.Set(&cost)
	t.modelValid = true
	if cost.Cmp(&t.minimumCost) >= 0 {
		if err = t.setCostConflict(indices); err != nil {
			return SMTFinalCheckGiveUp, err
		}
	}
	return SMTFinalCheckDone, nil
}
