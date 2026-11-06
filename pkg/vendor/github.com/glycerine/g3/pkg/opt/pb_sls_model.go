package opt

// Source: src/opt/pb_sls.cpp:143-185,709-746.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (s *PBSLS) GetModel() (answer *model.Model, result error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	answer, err := model.NewModel(s.manager)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, answer.Close())
			answer = nil
		}
	}()
	truth, err := s.manager.MkTrue()
	if err != nil {
		return nil, err
	}
	falsity, err := s.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	for i := 1; i < len(s.variableExprs); i++ {
		expression := s.variableExprs[i]
		kind, err := s.manager.GetKind(expression)
		if err != nil {
			return nil, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := s.manager.GetFamilyID(expression)
		if err != nil || family != ast.NullFamilyID {
			if err != nil {
				return nil, err
			}
			continue
		}
		n, err := s.manager.GetNumArgs(expression)
		if err != nil || n != 0 {
			if err != nil {
				return nil, err
			}
			continue
		}
		decl, err := s.manager.GetDecl(expression)
		if err != nil {
			return nil, err
		}
		value := falsity
		if s.assignment[i] {
			value = truth
		}
		if err = answer.Core().RegisterConstant(decl, value); err != nil {
			return nil, err
		}
	}
	return answer, nil
}

func (s *PBSLS) Penalty() (*big.Rat, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	return new(big.Rat).Set(&s.penalty), nil
}
