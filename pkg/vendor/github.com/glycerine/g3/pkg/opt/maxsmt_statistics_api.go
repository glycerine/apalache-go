package opt

// Source: src/opt/maxsmt.h collect_statistics publication.

import "github.com/glycerine/g3/pkg/ast"

func (s *ArithmeticSolver) MaxSMTResultStatistics(result *ArithmeticResult) ([]MaxSMTStatistics, error) {
	if s == nil || result == nil {
		return nil, ast.ErrASTHandle
	}
	return append([]MaxSMTStatistics(nil), result.MaxSMT...), nil
}
