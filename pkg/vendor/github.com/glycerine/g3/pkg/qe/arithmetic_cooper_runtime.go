package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticCooperRuntime struct {
	plugin   *ArithmeticPlugin
	variable ast.ASTHandle
	common   *big.Int
	residue  ast.ASTHandle
	fresh    []ast.ASTRef
	owned    []ast.ASTRef
}

func (p *ArithmeticPlugin) newCooperRuntime(variable ast.ASTHandle, processor *ArithmeticBoundsProcessor) (*ArithmeticCooperRuntime, bool, error) {
	all := append(processor.Divisibilities(false), processor.Divisibilities(true)...)
	if len(all) == 0 {
		return nil, false, nil
	}
	common, err := cooperLCM(all)
	if err != nil {
		return nil, false, err
	}
	if common.Cmp(big.NewInt(1)) == 0 {
		return nil, false, nil
	}
	runtime := &ArithmeticCooperRuntime{plugin: p, variable: variable, common: common}
	runtime.residue, err = runtime.boundedVariable(common)
	if err != nil {
		return nil, false, errors.Join(err, runtime.close())
	}
	return runtime, true, nil
}

// boundedVariable mirrors mk_bounded_var: the quantified owner is a bit-vector
// and arithmetic constraints use its unsigned conversion to an integer.
func (r *ArithmeticCooperRuntime) boundedVariable(limit *big.Int) (ast.ASTHandle, error) {
	if limit == nil || limit.Sign() <= 0 || uint64(limit.BitLen()) > uint64(^uint32(0)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	m := r.plugin.manager
	if _, err := m.RegisterBitVectorPlugin(); err != nil {
		return ast.ASTHandle{}, err
	}
	width := uint32(limit.BitLen())
	if width == 0 {
		width = 1
	}
	sort, err := m.BitVectorSort(width)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	bits, err := m.MkFreshConst("z", sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	owner, err := m.NewRef(bits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r.fresh = append(r.fresh, owner)
	value, err := m.MkBitVectorToInt(bits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	owner, err = m.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r.owned = append(r.owned, owner)
	return value, nil
}
func (r *ArithmeticCooperRuntime) publish() error {
	r.plugin.pendingFresh = append(r.plugin.pendingFresh, r.fresh...)
	r.fresh = nil
	return nil
}
func (r *ArithmeticCooperRuntime) close() (result error) {
	if r == nil {
		return nil
	}
	for i := len(r.owned) - 1; i >= 0; i-- {
		result = errors.Join(result, r.plugin.manager.DelRef(&r.owned[i]))
	}
	r.owned = nil
	for i := len(r.fresh) - 1; i >= 0; i-- {
		result = errors.Join(result, r.plugin.manager.DelRef(&r.fresh[i]))
	}
	r.fresh = nil
	return result
}
