package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// SolverNNFAdapter is the concrete retained i_solver_context relevance and
// atom-normalization adapter.
type SolverNNFAdapter struct {
	self     *SolverNNFAdapter
	context  *SolverContext
	contains []*ast.ContainsApp
	closed   bool
}

func NewSolverNNFAdapter(context *SolverContext) (result *SolverNNFAdapter, err error) {
	if context == nil {
		return nil, ast.ErrASTHandle
	}
	a := &SolverNNFAdapter{context: context}
	a.self = a
	defer func() {
		if err != nil {
			err = errors.Join(err, a.Close())
			result = nil
		}
	}()
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	count, err := context.NumVariables()
	if err != nil {
		return nil, err
	}
	for index := uint32(0); index < count; index++ {
		variable, cause := context.Variable(index)
		if cause != nil {
			return nil, cause
		}
		contains, cause := ast.NewContainsApp(manager, variable)
		if cause != nil {
			return nil, cause
		}
		a.contains = append(a.contains, contains)
	}
	return a, nil
}

func (a *SolverNNFAdapter) open() error {
	if a == nil || a.self != a || a.context == nil {
		return ast.ErrASTHandle
	}
	if a.closed {
		return arena.ErrClosed
	}
	_, err := a.context.Manager()
	return err
}

func (a *SolverNNFAdapter) IsRelevant(expression ast.ASTHandle) (bool, error) {
	if err := a.open(); err != nil {
		return false, err
	}
	if err := a.syncVariables(); err != nil {
		return false, err
	}
	for _, contains := range a.contains {
		found, err := contains.Check(expression)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

// syncVariables follows additions and swap-removals made by the solver context.
func (a *SolverNNFAdapter) syncVariables() error {
	variables, err := a.context.Variables()
	if err != nil {
		return err
	}
	common := len(a.contains)
	if len(variables) < common {
		common = len(variables)
	}
	firstChanged := common
	for i := 0; i < common; i++ {
		target, cause := a.contains[i].Target()
		if cause != nil {
			return cause
		}
		if target != variables[i] {
			firstChanged = i
			break
		}
	}
	for len(a.contains) > firstChanged {
		i := len(a.contains) - 1
		if err = a.contains[i].Close(); err != nil {
			return err
		}
		a.contains = a.contains[:i]
	}
	for len(a.contains) < len(variables) {
		contains, cause := ast.NewContainsApp(a.context.manager, variables[len(a.contains)])
		if cause != nil {
			return cause
		}
		a.contains = append(a.contains, contains)
	}
	return nil
}

func (a *SolverNNFAdapter) NormalizeAtom(expression ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	if err := a.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return a.context.MkAtom(expression, positive)
}

func (a *SolverNNFAdapter) Close() (result error) {
	if a == nil || a.self != a {
		return ast.ErrASTHandle
	}
	if a.closed {
		return nil
	}
	for len(a.contains) > 0 {
		last := len(a.contains) - 1
		if err := a.contains[last].Close(); err != nil {
			return errors.Join(result, err)
		}
		a.contains = a.contains[:last]
	}
	a.closed = true
	return result
}
