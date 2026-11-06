package opt

// Source: src/opt/wmax.cpp, wmax::is_true and update_assignment.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// wmaxModelAssignment evaluates each soft formula without model completion,
// as model::eval does in the weighted-theory engine.
func wmaxModelAssignment(s *ArithmeticSolver, md *model.ModelCore, soft []ast.ASTHandle) ([]bool, error) {
	if s == nil || s.manager == nil || md == nil {
		return nil, ast.ErrASTHandle
	}
	result := make([]bool, len(soft))
	for i, term := range soft {
		yes, err := wmaxModelIsTrue(s, md, term)
		if err != nil {
			return nil, err
		}
		result[i] = yes
	}
	return result, nil
}

func wmaxModelIsTrue(s *ArithmeticSolver, md *model.ModelCore, term ast.ASTHandle) (truth bool, resultErr error) {
	config := s.config.ModelEvaluator
	config.Params = nil // model::eval constructs the default evaluator.
	evaluator, err := model.NewModelEvaluator(md, config)
	if err != nil {
		if evaluator != nil {
			err = errors.Join(err, evaluator.Close())
		}
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	if err = evaluator.SetModelCompletion(false); err != nil {
		return false, err
	}
	value, err := s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&value)) }()
	if err = evaluator.Evaluate(term, value); errors.Is(err, model.ErrModelEvaluatorTheory) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	h, err := s.manager.RefGet(value)
	if err != nil {
		return false, err
	}
	return s.manager.IsTrue(h), nil
}
