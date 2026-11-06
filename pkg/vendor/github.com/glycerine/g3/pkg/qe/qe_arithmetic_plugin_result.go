package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type ArithmeticPluginBranch struct {
	Result      ast.ASTRef
	Definition  ast.ASTRef
	Coefficient *big.Rat
	Selection   ArithmeticBranchSelection
	Fresh       []ast.ASTRef
}

func (b *ArithmeticPluginBranch) close(m *ast.ASTManager) error {
	if b == nil {
		return nil
	}
	var definitionErr error
	for i := len(b.Fresh) - 1; i >= 0; i-- {
		definitionErr = errors.Join(definitionErr, m.DelRef(&b.Fresh[i]))
	}
	b.Fresh = nil
	if b.Definition != (ast.ASTRef{}) {
		definitionErr = m.DelRef(&b.Definition)
	}
	return errors.Join(definitionErr, m.DelRef(&b.Result))
}

func (p *ArithmeticPlugin) storeBranch(key ArithmeticBranchKey, result, definition ast.ASTRef, selection ArithmeticBranchSelection, fresh []ast.ASTRef) (err error) {
	var copies []ast.ASTRef
	defer func() {
		if err != nil {
			for i := len(copies) - 1; i >= 0; i-- {
				err = errors.Join(err, p.manager.DelRef(&copies[i]))
			}
		}
	}()
	for _, ref := range fresh {
		h, cause := p.manager.RefGet(ref)
		if cause != nil {
			return cause
		}
		copy, cause := p.manager.NewRef(h)
		if cause != nil {
			return cause
		}
		copies = append(copies, copy)
	}
	if old := p.branches[key]; old != nil {
		if err := old.close(p.manager); err != nil {
			return err
		}
	}
	p.branches[key] = &ArithmeticPluginBranch{Result: result, Definition: definition, Selection: selection, Coefficient: big.NewRat(1, 1), Fresh: copies}
	return nil
}

// replayBranch republishes the cached fresh variables without re-emitting the
// branch constraints, as the source get_cache path does.
func (p *ArithmeticPlugin) replayBranch(branch *ArithmeticPluginBranch) (err error) {
	var copies []ast.ASTRef
	defer func() {
		if err != nil {
			for i := len(copies) - 1; i >= 0; i-- {
				err = errors.Join(err, p.manager.DelRef(&copies[i]))
			}
		}
	}()
	for _, ref := range branch.Fresh {
		h, cause := p.manager.RefGet(ref)
		if cause != nil {
			return cause
		}
		copy, cause := p.manager.NewRef(h)
		if cause != nil {
			return cause
		}
		copies = append(copies, copy)
	}
	p.pendingFresh = append(p.pendingFresh, copies...)
	return nil
}

func (p *ArithmeticPlugin) branch(variable ast.ASTHandle, formula ast.ASTHandle, value *big.Rat) (*ArithmeticPluginBranch, error) {
	branch, err := arithmeticBranchUnsigned(value)
	if err != nil {
		return nil, err
	}
	key := ArithmeticBranchKey{Formula: formula, Variable: variable, Branch: branch}
	b := p.branches[key]
	if b == nil {
		return nil, ast.ErrASTDomain
	}
	return b, nil
}

func arithmeticBranchUnsigned(value *big.Rat) (uint32, error) {
	if value == nil || !value.IsInt() || value.Sign() < 0 || !value.Num().IsUint64() {
		return 0, ast.ErrASTDomain
	}
	v := value.Num().Uint64()
	if v > uint64(^uint32(0)) {
		return 0, ast.ErrASTDomain
	}
	return uint32(v), nil
}
