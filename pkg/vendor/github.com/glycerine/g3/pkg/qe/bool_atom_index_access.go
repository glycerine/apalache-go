package qe

import "github.com/glycerine/g3/pkg/ast"

func (i *BoolAtomIndex) Positive(atom ast.ASTHandle) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	_, found := i.positive[atom]
	return found, nil
}

func (i *BoolAtomIndex) Negative(atom ast.ASTHandle) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	_, found := i.negative[atom]
	return found, nil
}

func (i *BoolAtomIndex) positiveAtoms() map[ast.ASTHandle]ast.ASTRef { return i.positive }
func (i *BoolAtomIndex) negativeAtoms() map[ast.ASTHandle]ast.ASTRef { return i.negative }
