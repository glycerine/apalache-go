package fp

// Source: src/muz/base/dl_rule.cpp rule_manager::mk_query free-variable
// collection, reversed query-head domain, and descending de Bruijn arguments.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// makeQueryHead returns an owning head; its declaration remains borrowed from
// that owner until the caller registers it and installs the query rule.
func (e *PropositionalEngine) makeQueryHead(query ast.ASTHandle) (declaration ast.ASTHandle, result ast.ASTRef, resultErr error) {
	var roots []ast.ASTRef
	defer func() {
		if resultErr != nil {
			declaration, result = ast.ASTHandle{}, ast.ASTRef{}
			resultErr = errors.Join(resultErr, closeASTRefs(e.manager, roots))
			return
		}
		if cleanup := releaseQueryReferences(e.manager, roots[:len(roots)-1]); cleanup != nil {
			declaration, result = ast.ASTHandle{}, ast.ASTRef{}
			resultErr = &queryReferenceCleanupFailure{manager: e.manager, runtime: e.manager.NumeralService().RuntimeOwner(), references: roots, cause: cleanup, cleanup: cleanup}
			return
		}
		result = roots[len(roots)-1]
	}()
	boolean, err := e.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	sort, err := e.manager.GetSort(query)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	if sort != boolean {
		return ast.ASTHandle{}, ast.ASTRef{}, fmt.Errorf("%w: query is not Boolean", ErrPropositionalHorn)
	}
	used, err := ast.NewASTUsedVars(e.manager)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	if err = used.Process(query); err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	count := used.MaxFoundVarIndexPlusOne()
	if !used.UsesAllVars(count) {
		return ast.ASTHandle{}, ast.ASTRef{}, fmt.Errorf("%w: query has a gap in free variables", ErrPropositionalHorn)
	}
	domain := make([]ast.ASTHandle, count)
	arguments := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		domain[i], err = used.Get(count - i - 1)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, err
		}
		root, err := e.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, err
		}
		roots = append(roots, root)
		arguments[i], err = e.manager.MkVar(count-i-1, domain[i])
		if !arguments[i].IsNull() {
			err = errors.Join(err, e.manager.SetRef(root, arguments[i]))
		}
		if err != nil {
			return ast.ASTHandle{}, ast.ASTRef{}, err
		}
	}
	prefix, err := e.manager.Symbols.Intern("query")
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	declRoot, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	roots = append(roots, declRoot)
	declaration, err = e.manager.MkFreshFuncDecl(prefix, usymbol.Symbol{}, domain, boolean)
	if !declaration.IsNull() {
		err = errors.Join(err, e.manager.SetRef(declRoot, declaration))
	}
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	headRoot, err := e.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTRef{}, err
	}
	roots = append(roots, headRoot)
	head, err := e.manager.MkApp(declaration, arguments)
	if !head.IsNull() {
		err = errors.Join(err, e.manager.SetRef(headRoot, head))
	}
	return declaration, ast.ASTRef{}, err
}
