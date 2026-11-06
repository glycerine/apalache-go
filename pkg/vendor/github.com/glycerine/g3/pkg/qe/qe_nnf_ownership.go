package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (n *QENNFNormalizer) hold(expression ast.ASTHandle) error {
	ref, err := n.manager.NewRef(expression)
	if err != nil {
		return err
	}
	n.scratch = append(n.scratch, ref)
	return nil
}

func (n *QENNFNormalizer) resetCall() (result error) {
	for len(n.scratch) > 0 {
		last := len(n.scratch) - 1
		if err := n.manager.DelRef(&n.scratch[last]); err != nil {
			return errors.Join(result, err)
		}
		n.scratch = n.scratch[:last]
	}
	n.memo = make(map[qeNNFKey]ast.ASTHandle)
	return result
}

func (n *QENNFNormalizer) remember(key qeNNFKey, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := n.hold(value); err != nil {
		return ast.ASTHandle{}, err
	}
	n.memo[key] = value
	return value, nil
}
