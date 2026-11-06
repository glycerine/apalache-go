package opt

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

type ObjectiveFormulaSet struct {
	manager  *ast.ASTManager
	codec    *ObjectiveFormulaCodec
	formulas []ast.ASTRef
	closed   bool
}

func NewObjectiveFormulaSet(manager *ast.ASTManager, hard []ast.ASTHandle, objectives []FormulaObjective) (set *ObjectiveFormulaSet, resultErr error) {
	codec, err := NewObjectiveFormulaCodec(manager)
	if err != nil {
		return nil, err
	}
	set = &ObjectiveFormulaSet{manager: manager, codec: codec}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, set.Close())
			set = nil
		}
	}()
	for _, formula := range hard {
		ref, e := manager.NewRef(formula)
		if e != nil {
			return nil, e
		}
		set.formulas = append(set.formulas, ref)
	}
	for _, objective := range objectives {
		formula, e := codec.EncodeObjective(objective)
		if e != nil {
			return nil, e
		}
		ref, e := manager.NewRef(formula)
		if e != nil {
			return nil, e
		}
		set.formulas = append(set.formulas, ref)
	}
	return set, nil
}

func (s *ObjectiveFormulaSet) open() error {
	if s == nil || s.manager == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return arena.ErrClosed
	}
	if s.codec == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (s *ObjectiveFormulaSet) Handles() ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(s.formulas))
	for i := range s.formulas {
		handle, err := s.manager.RefGet(s.formulas[i])
		if err != nil {
			return nil, err
		}
		result[i] = handle
	}
	return result, nil
}

func (s *ObjectiveFormulaSet) Decode() (DecodedFormulaSet, error) {
	handles, err := s.Handles()
	if err != nil {
		return DecodedFormulaSet{}, err
	}
	return s.codec.Decode(handles)
}

// DecodeHandles consumes a tactic-transformed formula vector while the set
// retains marker declarations and source metadata.
func (s *ObjectiveFormulaSet) DecodeHandles(formulas []ast.ASTHandle) (DecodedFormulaSet, error) {
	if err := s.open(); err != nil {
		return DecodedFormulaSet{}, err
	}
	return s.codec.Decode(formulas)
}

func (s *ObjectiveFormulaSet) Close() (result error) {
	if s == nil || s.manager == nil {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	for i := len(s.formulas) - 1; i >= 0; i-- {
		result = errors.Join(result, s.manager.DelRef(&s.formulas[i]))
	}
	if result == nil {
		s.formulas = nil
		result = s.codec.Close()
	}
	if result == nil {
		s.codec = nil
		s.closed = true
	}
	return result
}
