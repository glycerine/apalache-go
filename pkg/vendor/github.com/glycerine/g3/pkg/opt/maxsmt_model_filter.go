package opt

// Source: src/opt/maxsmt.cpp/maxres.cpp filter_model_converter publication.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (s *ArithmeticSolver) filterResultModel(result *ArithmeticResult) (resultErr error) {
	if result == nil || result.Model == nil || result.ModelFiltered {
		return nil
	}
	if s.filter == nil && s.preprocessor == nil {
		return nil
	}
	var ref tactic.ModelRef
	if err := ref.Set(result.Model); err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, ref.Close()) }()
	if s.preprocessor != nil {
		if err := s.preprocessor.ApplyModel(&ref, 0); err != nil {
			return err
		}
	}
	if s.filter != nil {
		if err := s.filter.ApplyUnindexed(&ref); err != nil {
			return err
		}
	}
	filtered, err := ref.Get()
	if err != nil || filtered == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTHandle
	}
	if err = filtered.IncRef(); err != nil {
		return err
	}
	if err = result.Model.DecRef(); err != nil {
		return errors.Join(err, filtered.DecRef())
	}
	result.Model = filtered
	result.ModelFiltered = true
	return nil
}
