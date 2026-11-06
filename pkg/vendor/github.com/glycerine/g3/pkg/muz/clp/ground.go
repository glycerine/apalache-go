package clp

// Source: src/muz/clp/clp_context.cpp, reset_ground and ground.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

// groundEnvironment owns the rule-local fresh constants shared by the head
// and every tail. Each attempted rule receives a new environment.
type groundEnvironment struct {
	manager      *ast.ASTManager
	values       []ast.ASTHandle
	refs         []ast.ASTRef
	temporaries  []ast.ASTRef
	substitution *rewriter.VarSubstitution
}

type groundCleanupFailure struct {
	cause       error
	cleanupErr  error
	environment *groundEnvironment
}

func (f *groundCleanupFailure) Error() string { return f.cause.Error() }
func (f *groundCleanupFailure) Unwrap() error { return f.cause }
func (f *groundCleanupFailure) Cleanup() error {
	f.cleanupErr = f.environment.close()
	return f.cleanupErr
}
func (f *groundCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.environment.manager.NumeralService().RuntimeOwner()
}
func (f *groundCleanupFailure) OriginalCause() error { return f.cause }
func (f *groundCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (g *groundEnvironment) dispose() error {
	if err := g.close(); err != nil {
		return &groundCleanupFailure{cause: err, cleanupErr: err, environment: g}
	}
	return nil
}

// disposeResult also owns result when its construction failed, or when the
// environment itself could not be retired. This prevents a successful
// substitution result from becoming unreachable behind a cleanup error.
func (g *groundEnvironment) disposeResult(result *ast.ASTRef, discard bool) error {
	err := g.close()
	if result != nil && *result != (ast.ASTRef{}) && (discard || err != nil) {
		if err == nil {
			err = g.manager.DelRef(result)
		}
		if *result != (ast.ASTRef{}) {
			g.refs = append(g.refs, *result)
			g.values = append(g.values, ast.ASTHandle{})
			*result = ast.ASTRef{}
		}
	}
	if err != nil {
		return &groundCleanupFailure{cause: err, cleanupErr: err, environment: g}
	}
	return nil
}

func (g *groundEnvironment) close() error {
	if g.substitution != nil {
		if err := g.substitution.Close(); err != nil {
			return err
		}
		g.substitution = nil
	}
	for i := len(g.temporaries) - 1; i >= 0; i-- {
		if g.temporaries[i] != (ast.ASTRef{}) {
			if err := g.manager.DelRef(&g.temporaries[i]); err != nil {
				return err
			}
		}
	}
	g.temporaries = nil
	for i := len(g.refs) - 1; i >= 0; i-- {
		if g.refs[i] != (ast.ASTRef{}) {
			if err := g.manager.DelRef(&g.refs[i]); err != nil {
				return err
			}
			g.values[i] = ast.ASTHandle{}
		}
	}
	g.values, g.refs = nil, nil
	return nil
}

func (g *groundEnvironment) finishSubstitution(sub *rewriter.VarSubstitution, out *ast.ASTRef, operationErr error) (resultErr error) {
	resultErr = operationErr
	cleanupErr := sub.Close()
	if cleanupErr != nil {
		g.substitution = sub
	}
	resultErr = errors.Join(resultErr, cleanupErr)
	if resultErr == nil || out == nil || *out == (ast.ASTRef{}) {
		return resultErr
	}
	if cleanupErr == nil {
		resultErr = errors.Join(resultErr, g.manager.DelRef(out))
	}
	if *out != (ast.ASTRef{}) {
		g.temporaries = append(g.temporaries, *out)
		*out = ast.ASTRef{}
	}
	return resultErr
}

func (g *groundEnvironment) ground(h ast.ASTHandle) (out ast.ASTRef, resultErr error) {
	free, err := rewriter.NewExprFreeVars(g.manager)
	if err != nil {
		return out, err
	}
	if err = free.Collect(h); err != nil {
		return out, err
	}
	if int(free.Size()) > len(g.values) {
		count := int(free.Size()) - len(g.values)
		g.values = append(g.values, make([]ast.ASTHandle, count)...)
		g.refs = append(g.refs, make([]ast.ASTRef, count)...)
	}
	for i := uint32(0); i < free.Size(); i++ {
		sort, err := free.Get(i)
		if err != nil {
			return out, err
		}
		if sort.IsNull() {
			continue
		}
		if !g.values[i].IsNull() {
			oldSort, err := g.manager.GetSort(g.values[i])
			if err != nil {
				return out, err
			}
			if oldSort != sort {
				return out, ast.ErrASTDomain
			}
			continue
		}
		value, err := g.manager.MkFreshConst("c", sort)
		if err != nil {
			return out, err
		}
		ref, err := g.manager.NewRef(value)
		if err != nil {
			return out, err
		}
		g.values[i], g.refs[i] = value, ref
	}
	sub, err := rewriter.NewVarSubstitution(g.manager, false)
	defer func() {
		if sub != nil {
			resultErr = g.finishSubstitution(sub, &out, resultErr)
		}
	}()
	if err != nil {
		return out, err
	}
	out, err = g.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return out, err
	}
	if err = sub.Substitute(h, g.values, out); err != nil {
		return out, err
	}
	return out, nil
}

func (e *Engine) ground(h ast.ASTHandle) (out ast.ASTRef, resultErr error) {
	environment := &groundEnvironment{manager: e.manager}
	defer func() {
		resultErr = errors.Join(resultErr, environment.disposeResult(&out, resultErr != nil))
	}()
	return environment.ground(h)
}
