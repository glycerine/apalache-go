package tab

// Source: src/muz/tab/tab_context.cpp, clause::init_from_rule.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// NewClauseFromRule initializes a clause from the source rule's head,
// predicate tails, and conjunction of interpreted tails.
func NewClauseFromRule(rule *base.Rule, rw *rewriter.TheoryRewriter) (_ *Clause, result error) {
	if rule == nil || rw == nil {
		return nil, ast.ErrASTHandle
	}
	m, err := rule.ASTManager()
	if err != nil {
		return nil, err
	}
	if rw.Manager() != m {
		return nil, ast.ErrASTHandle
	}
	used, err := rule.UsedVariables()
	if err != nil {
		return nil, err
	}
	numVars := used.MaxFoundVarIndexPlusOne()
	if numVars == 0 {
		numVars = 1
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return nil, err
	}
	predicates := make([]ast.ASTHandle, 0, uninterpreted)
	constraints := make([]ast.ASTHandle, 0, total-uninterpreted)
	for i := 0; i < total; i++ {
		h, _, e := rule.Tail(i)
		if e != nil {
			return nil, e
		}
		if i < uninterpreted {
			predicates = append(predicates, h)
		} else {
			constraints = append(constraints, h)
		}
	}
	constraint, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&constraint)) }()
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, constraints, constraint); err != nil {
		return nil, err
	}
	h, err := m.RefGet(constraint)
	if err != nil {
		return nil, err
	}
	clause, err := NewClause(m, rw, head, predicates, h)
	if err != nil {
		return nil, err
	}
	// get_max_rule_var initializes its maximum to zero even for ground rules.
	clause.numVars = numVars
	return clause, nil
}
