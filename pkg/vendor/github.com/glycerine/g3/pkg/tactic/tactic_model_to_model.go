package tactic

// Source: src/tactic/model_converter.cpp:176-181.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func ModelConverterToModel(m *ast.ASTManager, c ModelConverter, out *ModelRef) (err error) {
	if c == nil {
		return nil
	}
	md, err := model.NewConverterModel(m)
	if err != nil {
		return err
	}
	if err = out.Set(md.Core()); err != nil {
		if !out.holds(md.Core()) {
			return errors.Join(err, md.Close())
		}
		return err
	}
	return c.ApplyModel(out, 0)
}
