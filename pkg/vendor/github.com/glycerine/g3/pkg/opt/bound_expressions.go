package opt

// Source: src/opt/opt_context.cpp:1343-1350, context::to_exprs.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// AppendBoundExpressions appends the infinity, rational and infinitesimal
// coefficients of a retained opt bound as independently owned AST references.
// The caller owns every entry appended to out, including a completed prefix
// when a later conversion fails.
func AppendBoundExpressions(m *ast.ASTManager, bound *numeral.OwnedInfEpsInfRational, out *[]ast.ASTRef) (result error) {
	if m == nil || out == nil {
		return arena.ErrConfig
	}
	s := m.NumeralService()
	inf, err := s.InfEpsInfGetInfinity(bound)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.Del(&inf)) }()
	r, err := s.InfEpsInfGetRational(bound)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.Del(&r)) }()
	eps, err := s.InfEpsInfGetInfinitesimal(bound)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.Del(&eps)) }()
	for _, value := range []*numeral.OwnedRational{&inf, &r, &eps} {
		integer, err := s.IsInt(value)
		if err != nil {
			return err
		}
		h, err := m.ArithmeticNumeral(value, integer)
		if err != nil {
			return err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return err
		}
		*out = append(*out, ref)
	}
	return nil
}
