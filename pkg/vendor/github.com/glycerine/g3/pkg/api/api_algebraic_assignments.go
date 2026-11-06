package api

// Source: src/api/api_algebraic.cpp:326-351 vector_var2anum.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

type apiAlgebraicAssignments struct {
	values []apiAlgebraicInput
}

func newAPIAlgebraicAssignments(manager *ast.ASTManager, handles []ast.ASTHandle) (result apiAlgebraicAssignments, resultErr error) {
	if uint64(len(handles)) > math.MaxUint32 {
		return result, arena.ErrBounds
	}
	result.values = make([]apiAlgebraicInput, len(handles))
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.close())
		}
	}()
	for i := range handles {
		value, err := readAPIAlgebraicInput(manager, handles[i])
		if err != nil {
			return result, err
		}
		result.values[i] = value
	}
	return result, nil
}

func (v *apiAlgebraicAssignments) close() (result error) {
	if v == nil {
		return nil
	}
	for i := len(v.values); i != 0; i-- {
		result = errors.Join(result, v.values[i-1].close())
	}
	v.values = nil
	return result
}

func (v *apiAlgebraicAssignments) Contains(variable uint32) (bool, error) {
	return uint64(variable) < uint64(len(v.values)), nil
}

func (v *apiAlgebraicAssignments) Algebraic(variable uint32) (*polynomial.Algebraic, error) {
	if uint64(variable) >= uint64(len(v.values)) {
		return nil, arena.ErrBounds
	}
	return &v.values[variable].value, nil
}
