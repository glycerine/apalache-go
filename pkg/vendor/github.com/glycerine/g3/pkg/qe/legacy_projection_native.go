package qe

// Source: src/muz/spacer/spacer_util.cpp:602-610 qe_project's native
// arithmetic branch. Boolean and array projection precede this call.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (p *LegacyProjection) projectNativeArithmetic(md *model.ModelCore, variables []ast.ASTHandle, current ast.ASTRef) (resultErr error) {
	var vars, forms []ast.ASTRef
	defer func() {
		resultErr = errors.Join(resultErr, truncateLiteVector(p.manager, &forms, 0), truncateLiteVector(p.manager, &vars, 0))
	}()
	for _, variable := range variables {
		ref, err := p.manager.NewRef(variable)
		if err != nil {
			return err
		}
		vars = append(vars, ref)
	}
	handle, err := p.manager.RefGet(current)
	if err != nil {
		return err
	}
	flattened, err := p.manager.FlattenAnd([]ast.ASTHandle{handle})
	if err != nil {
		return err
	}
	for _, formula := range flattened {
		ref, err := p.manager.NewRef(formula)
		if err != nil {
			return err
		}
		forms = append(forms, ref)
	}
	projector, err := NewModelBasedProjection(p.manager, p.config)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, projector.Close()) }()
	if err = projector.Project(true, &vars, md, &forms); err != nil {
		return err
	}
	if len(vars) != 0 {
		return ast.ErrASTDomain
	}
	flattened = flattened[:0]
	for _, ref := range forms {
		handle, err := p.manager.RefGet(ref)
		if err != nil {
			return err
		}
		flattened = append(flattened, handle)
	}
	conjunction, err := p.manager.MkAndArray(flattened)
	if err != nil {
		return err
	}
	return p.manager.SetRef(current, conjunction)
}
