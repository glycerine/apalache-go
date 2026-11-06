package opt

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

func (p *ObjectivePurifier) FilterModel(input *model.ModelCore) (_ *model.ModelCore, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if input == nil {
		return nil, ast.ErrASTHandle
	}
	if p.stats.FreshDeclarations == 0 {
		if err := input.IncRef(); err != nil {
			return nil, err
		}
		return input, nil
	}
	var reference tactic.ModelRef
	if err := reference.Set(input); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, reference.Close()) }()
	if err := p.filter.ApplyUnindexed(&reference); err != nil {
		return nil, err
	}
	output, err := reference.Get()
	if err != nil || output == nil {
		if err == nil {
			err = ast.ErrASTHandle
		}
		return nil, err
	}
	if err = output.IncRef(); err != nil {
		return nil, err
	}
	return output, nil
}
