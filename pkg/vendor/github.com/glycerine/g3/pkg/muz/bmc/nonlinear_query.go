package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, nonlinear::mk_level_predicate,
// mk_level_rule and compile_query.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// Compiler borrows the AST manager and constructs BMC level encodings.
type Compiler struct{ manager *ast.ASTManager }

func NewCompiler(m *ast.ASTManager) (*Compiler, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	return &Compiler{manager: m}, nil
}

func (c *Compiler) levelDeclaration(predicate ast.ASTHandle, level uint32, ruleIndex *uint32) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	name, err := m.GetASTName(predicate)
	if err != nil {
		return result, err
	}
	text, err := m.Symbols.Display(name)
	if err != nil {
		return result, err
	}
	if ruleIndex == nil {
		text = fmt.Sprintf("%s#%d", text, level)
	} else {
		text = fmt.Sprintf("rule:%s#%d_%d", text, level, *ruleIndex)
	}
	sym, err := m.Symbols.Intern(text)
	if err != nil {
		return result, err
	}
	arity, err := m.GetArity(predicate)
	if err != nil {
		return result, err
	}
	domain := make([]ast.ASTHandle, arity)
	for i := range domain {
		domain[i], err = m.GetDomain(predicate, uint32(i))
		if err != nil {
			return result, err
		}
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return result, err
	}
	decl, err := m.MkFuncDecl(sym, domain, boolean, ast.ASTFuncDeclInfo{})
	if err != nil {
		return result, err
	}
	return m.NewRef(decl)
}

// CompileQuery returns an owning reference to the source level predicate
// applied to one named constant per predicate argument.
func (c *Compiler) CompileQuery(predicate ast.ASTHandle, level uint32) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	// Publication cleanup runs after the level-declaration and argument-root
	// destructors, so a late temporary cleanup error cannot return a live query
	// together with an error.
	defer func() {
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = disposeASTRefs(m, []ast.ASTRef{result}, resultErr)
			result = ast.ASTRef{}
		}
	}()
	original, err := m.GetASTName(predicate)
	if err != nil {
		return result, err
	}
	name, err := m.Symbols.Display(original)
	if err != nil {
		return result, err
	}
	levelRef, err := c.levelDeclaration(predicate, level, nil)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = disposeASTRefs(m, []ast.ASTRef{levelRef}, resultErr) }()
	levelDecl, err := m.RefGet(levelRef)
	if err != nil {
		return result, err
	}
	arity, err := m.GetArity(levelDecl)
	if err != nil {
		return result, err
	}
	args := make([]ast.ASTHandle, arity)
	var refs []ast.ASTRef
	defer func() {
		resultErr = disposeASTRefs(m, refs, resultErr)
	}()
	for i := range args {
		sort, err := m.GetDomain(levelDecl, uint32(i))
		if err != nil {
			return result, err
		}
		sym, err := m.Symbols.Intern(fmt.Sprintf("%s#%d_%d", name, level, i))
		if err != nil {
			return result, err
		}
		args[i], err = m.MkConst(sym, sort)
		if err != nil {
			return result, err
		}
		ref, err := m.NewRef(args[i])
		if err != nil {
			return result, err
		}
		refs = append(refs, ref)
	}
	application, err := m.MkApp(levelDecl, args)
	if err != nil {
		return result, err
	}
	return m.NewRef(application)
}

func (c *Compiler) Close() error {
	if c != nil {
		c.manager = nil
	}
	return nil
}
