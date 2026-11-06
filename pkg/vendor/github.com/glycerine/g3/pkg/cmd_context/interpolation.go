package cmd_context

// Source: src/cmd_context/interpolant_cmds.cpp get-interpolant and
// compute-interpolant commands.

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (c *Context) interpolationManager() (*interp.Manager, error) {
	if _, err := c.Manager(); err != nil {
		return nil, err
	}
	return interp.NewManagerWithParams(c.manager, c.config.Params, c.config.Globals)
}

func (c *Context) interpolationRunConfig() (*interp.RunConfig, solver.Factory, error) {
	factory, err := smtcore.NewSMTSolverFactory(c.config)
	if err != nil {
		return nil, nil, err
	}
	config := &interp.RunConfig{Profiler: interp.ProcessProfiler(), SolverFactory: factory, EvaluatorFactory: func(md *model.ModelCore) (solver.ConsequenceEvaluator, error) {
		return model.NewModelEvaluator(md, c.config.ModelEvaluator)
	}}
	return config, factory, nil
}

func (c *Context) writeInterpolantResults(out io.Writer, rewritingManager *interp.Manager, refs []ast.ASTRef) (result error) {
	if _, err := io.WriteString(out, "(interpolants"); err != nil {
		return err
	}
	for i := range refs {
		h, err := c.manager.RefGet(refs[i])
		if err != nil {
			return err
		}
		display, err := rewritingManager.Simplify(h)
		if err != nil {
			return err
		}
		h, err = c.manager.RefGet(display)
		if err != nil {
			return errors.Join(err, c.manager.DelRef(&display))
		}
		if _, err = io.WriteString(out, "\n "); err != nil {
			return errors.Join(err, c.manager.DelRef(&display))
		}
		if err = c.WriteExpression(out, h); err != nil {
			return errors.Join(err, c.manager.DelRef(&display))
		}
		if err = c.manager.DelRef(&display); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")\n")
	return err
}

func (c *Context) finishInterpolationOutput(out io.Writer, manager *interp.Manager, factory solver.Factory, constraints []ast.ASTHandle, tree ast.ASTHandle, interpolants []ast.ASTHandle) (result error) {
	options, err := interp.NewParams(c.config.Params, c.config.Globals, &c.params)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, options.Close()) }()
	check, err := options.Check()
	if err != nil {
		return err
	}
	if check {
		checker, e := factory.New(c.manager, &c.params, false, true, false, c.logic)
		if e != nil {
			return e
		}
		var diagnostics bytes.Buffer
		valid, checkErr := interp.CheckTreeInterpolants(manager, checker, &diagnostics, constraints, tree, interpolants)
		closeErr := checker.Close()
		if checkErr != nil || closeErr != nil {
			return errors.Join(checkErr, closeErr)
		}
		if valid {
			_, err = io.WriteString(out, "correct\n")
		} else {
			_, err = fmt.Fprintf(out, "incorrect: %s\n", diagnostics.String())
		}
		if err != nil {
			return err
		}
	}
	profile, err := options.Profile()
	if err != nil {
		return err
	}
	if profile {
		err = interp.ProcessProfiler().Print(out)
	}
	return err
}

// WriteInterpolants computes interpolants from the latest unsatisfiable proof.
func (c *Context) WriteInterpolants(out io.Writer, tree ast.ASTHandle) (result error) {
	state, err := c.parameters.State()
	if err != nil {
		return err
	}
	if !state.Proof {
		return fmt.Errorf("interpolation is not enabled, use command (set-option :produce-interpolants true)")
	}
	if !c.checked || c.status != lbool.False {
		return fmt.Errorf("proof is not available")
	}
	proof, err := c.checkSatResult().Proof()
	if err != nil || proof.IsNull() {
		return fmt.Errorf("proof is not available")
	}
	manager, err := c.interpolationManager()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, manager.Close()) }()
	config, factory, err := c.interpolationRunConfig()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, factory.Close()) }()
	constraints := make([]ast.ASTHandle, len(c.assertions))
	for i := range c.assertions {
		constraints[i] = c.assertions[i].expression
	}
	refs, err := interp.TreeInterpolants(manager, proof, constraints, tree, config)
	if err != nil {
		return err
	}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, c.manager.DelRef(&refs[i]))
		}
	}()
	interpolants, err := c.interpolationHandles(refs)
	if err != nil {
		return err
	}
	if err = c.writeInterpolantResults(out, manager, refs); err != nil {
		return err
	}
	return c.finishInterpolationOutput(out, manager, factory, constraints, tree, interpolants)
}

// ComputeInterpolants solves an interpolation tree with a fresh proof solver.
func (c *Context) ComputeInterpolants(out io.Writer, tree ast.ASTHandle) (result error) {
	proofMode, err := ast.NewScopedProof(c.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, proofMode.Close()) }()
	manager, err := c.interpolationManager()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, manager.Close()) }()
	config, factory, err := c.interpolationRunConfig()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, factory.Close()) }()
	target, err := factory.New(c.manager, &c.params, true, true, false, c.logic)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, target.Close()) }()
	status, constraints, refs, mdl, err := interp.SolveTree(manager, target, tree, true, config)
	if mdl != nil {
		defer func() { result = errors.Join(result, mdl.DecRef()) }()
	}
	if err != nil {
		return err
	}
	word := "unknown"
	if status == lbool.True {
		word = "sat"
	} else if status == lbool.False {
		word = "unsat"
	}
	if _, err = fmt.Fprintln(out, word); err != nil {
		return err
	}
	if status != lbool.False {
		for i := range refs {
			result = errors.Join(result, c.manager.DelRef(&refs[i]))
		}
		return result
	}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, c.manager.DelRef(&refs[i]))
		}
	}()
	interpolants, err := c.interpolationHandles(refs)
	if err != nil {
		return err
	}
	if err = c.writeInterpolantResults(out, manager, refs); err != nil {
		return err
	}
	return c.finishInterpolationOutput(out, manager, factory, constraints, tree, interpolants)
}

func (c *Context) interpolationHandles(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		handle, err := c.manager.RefGet(refs[i])
		if err != nil {
			return nil, err
		}
		result[i] = handle
	}
	return result, nil
}
