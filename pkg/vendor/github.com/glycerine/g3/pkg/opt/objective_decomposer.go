package opt

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type ObjectiveDecomposer struct {
	manager   *ast.ASTManager
	arith     ast.FamilyID
	zeroOne   map[ast.ASTHandle]struct{}
	derived   []ast.ASTRef
	generated int
	closed    bool
}

func NewObjectiveDecomposer(manager *ast.ASTManager, zeroOne []ast.ASTHandle) (*ObjectiveDecomposer, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	if _, err = manager.RegisterBitVectorPlugin(); err != nil {
		return nil, err
	}
	d := &ObjectiveDecomposer{manager: manager, arith: family, zeroOne: make(map[ast.ASTHandle]struct{}, len(zeroOne))}
	for _, term := range zeroOne {
		kind, e := manager.GetKind(term)
		if e != nil || kind != ast.ASTApp {
			if e == nil {
				e = ast.ErrASTDomain
			}
			return nil, e
		}
		d.zeroOne[term] = struct{}{}
	}
	return d, nil
}

func (d *ObjectiveDecomposer) open() error {
	if d == nil || d.manager == nil {
		return ast.ErrASTHandle
	}
	if d.closed {
		return arena.ErrClosed
	}
	return nil
}

func (d *ObjectiveDecomposer) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := d.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	d.derived = append(d.derived, r)
	return h, nil
}

func (d *ObjectiveDecomposer) retain(h ast.ASTHandle) (ast.ASTHandle, error) {
	h, err := d.hold(h)
	if err == nil {
		d.generated++
	}
	return h, err
}

func (d *ObjectiveDecomposer) Close() (result error) {
	if d == nil || d.manager == nil {
		return ast.ErrASTHandle
	}
	if d.closed {
		return nil
	}
	for i := len(d.derived) - 1; i >= 0; i-- {
		result = errors.Join(result, d.manager.DelRef(&d.derived[i]))
	}
	if result == nil {
		d.derived = nil
		d.generated = 0
		d.zeroOne = nil
		d.closed = true
	}
	return result
}
