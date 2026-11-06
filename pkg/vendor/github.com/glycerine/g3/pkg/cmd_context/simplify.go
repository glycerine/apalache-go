package cmd_context

// Source: src/cmd_context/simplify_cmd.cpp default term-rewrite execution.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

type SimplifyDetails struct {
	Seconds                                                float64
	Steps, CacheSize, NodesBefore, SharedNodes, NodesAfter uint32
	MemoryBytes, MaximumMemoryBytes                        uint64
	Failed                                                 bool
	Failure                                                string
}

type simplifyExpressionSolver struct {
	context *Context
	backend *smtcore.SMTSolver
}

func (s *simplifyExpressionSolver) CheckSatisfiable(expression ast.ASTHandle) (status lbool.LBool, resultErr error) {
	if s == nil || s.context == nil {
		return lbool.Undef, ast.ErrASTHandle
	}
	if s.backend == nil {
		backend, err := smtcore.NewSMTSolver(s.context.manager, &s.context.params, s.context.logic, s.context.config)
		if err != nil {
			return lbool.Undef, err
		}
		s.backend = backend
	}
	if err := s.backend.Push(); err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.backend.Pop(1)) }()
	if err := s.backend.Assert(expression); err != nil {
		return lbool.Undef, err
	}
	return s.backend.CheckSat(nil)
}

func (s *simplifyExpressionSolver) Close() error {
	if s == nil || s.backend == nil {
		return nil
	}
	err := s.backend.Close()
	if err == nil {
		s.backend = nil
	}
	return err
}

// Simplify rewrites one term with command-local theory-rewriter parameters.
// The returned reference is owned by the caller.
func (c *Context) Simplify(expression ast.ASTHandle, params *uparams.ParamsRef) (result ast.ASTRef, resultErr error) {
	result, _, resultErr = c.SimplifyWithProof(expression, params, false)
	return result, resultErr
}

// SimplifyWithProof optionally returns the rewrite proof in a second caller-
// owned reference. Proof production requires a proof-enabled AST manager.
func (c *Context) SimplifyWithProof(expression ast.ASTHandle, params *uparams.ParamsRef, produceProof bool) (result, proof ast.ASTRef, resultErr error) {
	result, proof, _, resultErr = c.SimplifyWithDetails(expression, params, produceProof, false)
	return result, proof, resultErr
}

// SimplifyWithDetails optionally captures the source command's statistics
// fields while the simplified term remains rooted for node accounting.
func (c *Context) SimplifyWithDetails(expression ast.ASTHandle, params *uparams.ParamsRef, produceProof, collectStatistics bool) (result, proof ast.ASTRef, details SimplifyDetails, resultErr error) {
	return c.simplifyWithDetails(expression, params, produceProof, collectStatistics, false)
}

// SimplifyCommandWithDetails adds simplify_cmd's recoverable diagnostic
// behavior to the underlying rewrite operation.
func (c *Context) SimplifyCommandWithDetails(expression ast.ASTHandle, params *uparams.ParamsRef, produceProof, collectStatistics bool) (result, proof ast.ASTRef, details SimplifyDetails, resultErr error) {
	return c.simplifyWithDetails(expression, params, produceProof, collectStatistics, true)
}

func (c *Context) simplifyWithDetails(expression ast.ASTHandle, params *uparams.ParamsRef, produceProof, collectStatistics, catchFailure bool) (result, proof ast.ASTRef, details SimplifyDetails, resultErr error) {
	if err := c.valid(); err != nil {
		return result, proof, details, err
	}
	m, err := c.Manager()
	if err != nil {
		return result, proof, details, err
	}
	if collectStatistics {
		details.NodesBefore, err = m.GetNumExprs(expression)
		if err != nil {
			return result, proof, details, err
		}
	}
	value, err := rewriter.NewTheoryRewriter(m, c.config.Params, c.config.Globals, params)
	if err != nil {
		if value != nil {
			err = errors.Join(err, value.Close())
		}
		return result, proof, details, err
	}
	checker := &simplifyExpressionSolver{context: c}
	if err = value.SetSolver(checker); err != nil {
		return result, proof, details, errors.Join(err, value.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, checker.Close()) }()
	defer func() { resultErr = errors.Join(resultErr, value.Close()) }()
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, proof, details, err
	}
	published := false
	defer func() {
		if !published {
			if proof != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(&proof))
			}
			resultErr = errors.Join(resultErr, m.DelRef(&result))
		}
	}()
	if produceProof {
		proof, err = m.NewRef(ast.ASTHandle{})
		if err != nil {
			return result, proof, details, err
		}
	}
	local := params
	empty := c.config.Params.EmptyValue()
	if local == nil {
		local = &empty
	}
	timeoutName, rlimitName := "timeout", "rlimit"
	timeout, err := c.config.Params.GetUIntName(local, &timeoutName, math.MaxUint32)
	if err != nil {
		return result, proof, details, err
	}
	rlimit, err := c.config.Params.GetUIntName(local, &rlimitName, math.MaxUint32)
	if err != nil {
		return result, proof, details, err
	}
	if err = m.ResourceLimit().Push(rlimit); err != nil {
		return result, proof, details, err
	}
	defer m.ResourceLimit().Pop()
	handler := &contextTimeoutHandler{limit: m.ResourceLimit(), active: true}
	timer, err := control.NewScopedTimer(timeout, handler)
	if err != nil {
		return result, proof, details, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, timer.Close())
		handler.close()
	}()
	watch := stopwatch.New(nil)
	if err = watch.Start(); err != nil {
		return result, proof, details, err
	}
	if produceProof {
		err = value.RewriteWithProof(expression, result, proof)
	} else {
		err = value.Rewrite(expression, result)
	}
	rewriteErr := err
	if err = watch.Stop(); err != nil {
		return result, proof, details, errors.Join(rewriteErr, err)
	}
	if rewriteErr != nil {
		if !catchFailure || !rewriter.IsRewriterException(rewriteErr) {
			return result, proof, details, rewriteErr
		}
		details.Failed = true
		details.Failure = rewriteErr.Error()
		if err = m.SetRef(result, expression); err != nil {
			return result, proof, details, err
		}
	}
	if collectStatistics {
		if details.Seconds, err = watch.Seconds(); err != nil {
			return result, proof, details, err
		}
		if details.Steps, err = value.NumSteps(); err != nil {
			return result, proof, details, err
		}
		if details.CacheSize, err = value.CacheSize(); err != nil {
			return result, proof, details, err
		}
		if !details.Failed {
			h, e := m.RefGet(result)
			if e != nil {
				return result, proof, details, e
			}
			if details.NodesAfter, e = m.GetNumExprs(h); e != nil {
				return result, proof, details, e
			}
			shared, e := ast.NewASTSharedOccurrences(m)
			if e != nil {
				return result, proof, details, e
			}
			if e = shared.Collect(h); e == nil {
				details.SharedNodes = shared.NumShared()
			}
			e = errors.Join(e, shared.Close())
			if e != nil {
				return result, proof, details, e
			}
		}
		runtime := c.config.Params.NumeralsField.RuntimeOwner()
		if details.MemoryBytes, err = runtime.AllocationSize(); err != nil {
			return result, proof, details, err
		}
		if details.MaximumMemoryBytes, err = runtime.MaximumUsed(); err != nil {
			return result, proof, details, err
		}
	}
	published = true
	return result, proof, details, nil
}
