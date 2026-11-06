package bv

// Source: src/tactic/bv/max_bv_sharing_tactic.cpp:238-271.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

var ErrSharingMaxSteps = errors.New("max. steps exceeded")
var ErrSharingMaxMemory = errors.New("max. memory exceeded")

// SharingGoalPass applies the concrete BV sharing reducer to goal formulas.
// The standard tactic owns the retained traversal and lifecycle.
type SharingGoalPass struct {
	manager     *ast.ASTManager
	memory      *memory.Runtime
	cooperation *cooperate.Cooperation
	family      ast.FamilyID
	maxArgs     uint32
	maxSteps    uint32
	maxMemory   uint64
}

func NewSharingGoalPass(m *ast.ASTManager, maxArgs uint32) (*SharingGoalPass, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	numerals := m.NumeralService()
	if numerals == nil || numerals.RuntimeOwner() == nil {
		return nil, ast.ErrASTHandle
	}
	runtime := numerals.RuntimeOwner()
	if _, e := runtime.AllocationSize(); e != nil {
		return nil, e
	}
	fid, e := m.RegisterBitVectorPlugin()
	if e != nil {
		return nil, e
	}
	return &SharingGoalPass{manager: m, memory: runtime, family: fid, maxArgs: maxArgs, maxSteps: math.MaxUint32, maxMemory: math.MaxUint64}, nil
}

// BindCooperation borrows the source process owner for sharing checkpoints.
func (p *SharingGoalPass) BindCooperation(owner *cooperate.Cooperation) error {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if owner == nil {
		return arena.ErrConfig
	}
	if _, err := owner.Enabled(); err != nil {
		return err
	}
	p.cooperation = owner
	return nil
}

// UpdateParams reads the three unsigned settings used by rw_cfg::updt_params.
// The memory limit reads the source process allocation counter.
func (p *SharingGoalPass) UpdateParams(c *uparams.ParamsContext, value *uparams.ParamsRef) error {
	if p == nil || p.manager == nil || p.memory == nil || c == nil || c.SymbolsField != p.manager.Symbols {
		return ast.ErrASTHandle
	}
	paramsNumerals := c.Numerals()
	if paramsNumerals == nil || paramsNumerals.RuntimeOwner() == nil {
		return ast.ErrASTHandle
	}
	process, paramsProcess := p.memory.ProcessOwner(), paramsNumerals.RuntimeOwner().ProcessOwner()
	if (process != nil || paramsProcess != nil) && process != paramsProcess {
		return ast.ErrASTHandle
	}
	memName, stepsName, argsName := "max_memory", "max_steps", "max_args"
	mem, e := c.GetUIntName(value, &memName, math.MaxUint32)
	if e != nil {
		return e
	}
	steps, e := c.GetUIntName(value, &stepsName, math.MaxUint32)
	if e != nil {
		return e
	}
	args, e := c.GetUIntName(value, &argsName, 128)
	if e != nil {
		return e
	}
	limit := memory.MegabytesToBytes(mem)
	p.maxMemory, p.maxSteps, p.maxArgs = limit, steps, args
	return nil
}

func NewSharingGoalPassWithParams(m *ast.ASTManager, c *uparams.ParamsContext, value *uparams.ParamsRef) (*SharingGoalPass, error) {
	p, e := NewSharingGoalPass(m, 128)
	if e != nil {
		return nil, e
	}
	if e = p.UpdateParams(c, value); e != nil {
		return nil, e
	}
	return p, nil
}

// Apply preserves the earlier concrete API while sharing the source traversal
// with MaxBVSharingTactic. The returned goal reference belongs to the caller.
func (p *SharingGoalPass) Apply(goal *tactic.Goal) (_ *tactic.Goal, result error) {
	imp, err := newSharingImplementation(p)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, imp.Close()) }()
	if err = imp.apply(goal); err != nil {
		if err.Error() == ErrSharingMaxSteps.Error() {
			return nil, ErrSharingMaxSteps
		}
		if err.Error() == ErrSharingMaxMemory.Error() {
			return nil, ErrSharingMaxMemory
		}
		return nil, err
	}
	if err = goal.IncRef(); err != nil {
		return nil, err
	}
	return goal, nil
}
