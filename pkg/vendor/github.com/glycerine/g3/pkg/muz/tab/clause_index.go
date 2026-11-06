package tab

// Source: src/muz/tab/tab_context.cpp, tb::rules.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
)

// RuleIndex owns the clause copies of a Datalog rule set and indexes them by
// head declaration. Returned clauses are borrowed until the index is closed.
type RuleIndex struct {
	manager *ast.ASTManager
	clauses []*Clause
	byHead  map[ast.ASTHandle][]int
	closed  bool
}

func NewRuleIndex(rules *base.RuleSet, rw *rewriter.TheoryRewriter) (built *RuleIndex, result error) {
	if rules == nil || rw == nil || rw.Manager() == nil {
		return nil, ast.ErrASTHandle
	}
	index := &RuleIndex{manager: rw.Manager(), byHead: make(map[ast.ASTHandle][]int)}
	attempt := &ruleIndexConstructionAttempt{manager: rw.Manager(), index: index}
	defer func() { built, result = finishRuleIndexConstruction(attempt, result) }()
	size, err := rules.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < size; i++ {
		rule, e := rules.Rule(i)
		if e != nil {
			return nil, e
		}
		attempt.rule = rule
		clause, e := NewClauseFromRule(rule, rw)
		if clause != nil {
			attempt.clause = clause
		}
		if closeErr := rule.Close(); closeErr != nil {
			e = errors.Join(e, closeErr)
		} else {
			attempt.rule = nil
		}
		if e != nil {
			return nil, e
		}
		clause.index = uint32(i)
		head, e := clause.Head()
		if e != nil {
			return nil, errors.Join(e, clause.Close())
		}
		decl, e := index.manager.GetDecl(head)
		if e != nil {
			return nil, errors.Join(e, clause.Close())
		}
		index.byHead[decl] = append(index.byHead[decl], len(index.clauses))
		index.clauses = append(index.clauses, clause)
		attempt.clause = nil
	}
	return index, nil
}

func (r *RuleIndex) Len() (int, error) {
	if r == nil || r.closed {
		return 0, ast.ErrASTHandle
	}
	return len(r.clauses), nil
}
func (r *RuleIndex) Clause(index int) (*Clause, error) {
	if r == nil || r.closed {
		return nil, ast.ErrASTHandle
	}
	if index < 0 || index >= len(r.clauses) {
		return nil, arena.ErrBounds
	}
	return r.clauses[index], nil
}
func (r *RuleIndex) Count(decl ast.ASTHandle) (int, error) {
	if r == nil || r.closed {
		return 0, ast.ErrASTHandle
	}
	if _, err := r.manager.GetKind(decl); err != nil {
		return 0, err
	}
	return len(r.byHead[decl]), nil
}
func (r *RuleIndex) Rule(decl ast.ASTHandle, index int) (*Clause, error) {
	if r == nil || r.closed {
		return nil, ast.ErrASTHandle
	}
	if _, err := r.manager.GetKind(decl); err != nil {
		return nil, err
	}
	group := r.byHead[decl]
	if index < 0 || index >= len(group) {
		return nil, arena.ErrBounds
	}
	return r.clauses[group[index]], nil
}
func (r *RuleIndex) Close() error {
	if r == nil || r.closed {
		return nil
	}
	for _, clause := range r.clauses {
		if err := clause.Close(); err != nil {
			return err
		}
	}
	r.closed = true
	r.clauses = nil
	r.byHead = nil
	return nil
}

// Index is the position assigned when the source rule collection is built.
func (c *Clause) Index() (uint32, error) {
	if c == nil || c.closed {
		return 0, ast.ErrASTHandle
	}
	return c.index, nil
}
