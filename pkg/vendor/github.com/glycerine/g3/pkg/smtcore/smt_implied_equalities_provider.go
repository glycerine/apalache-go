package smtcore

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type smtImpliedEqualityOwnedEvaluator struct {
	evaluator *model.ModelEvaluator
	model     *model.ModelCore
}

func (o *smtImpliedEqualityOwnedEvaluator) EvaluateValue(term ast.ASTHandle) (ast.ASTRef, error) {
	return o.evaluator.EvaluateValue(term)
}

func (o *smtImpliedEqualityOwnedEvaluator) Close() error {
	if o == nil {
		return nil
	}
	var err error
	if o.evaluator != nil {
		err = o.evaluator.Close()
		o.evaluator = nil
	}
	if o.model != nil {
		err = errors.Join(err, o.model.DecRef())
		o.model = nil
	}
	return err
}

func (s *SMTSolver) newImpliedEqualityEvaluator() (smtImpliedEqualityEvaluator, error) {
	if s.config.ModelEvaluator.Context == nil || s.config.ModelEvaluator.Globals == nil || s.config.ModelEvaluator.Memory == nil || s.config.ModelEvaluator.Cooperation == nil {
		return nil, nil
	}
	md, err := s.AcquireModel()
	if err != nil || md == nil {
		return nil, errors.Join(err, ast.ErrASTPending)
	}
	config := s.config.ModelEvaluator
	config.Params = &s.params
	evaluator, err := model.NewModelEvaluator(md, config)
	owner := &smtImpliedEqualityOwnedEvaluator{evaluator: evaluator, model: md}
	if err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return owner, nil
}
