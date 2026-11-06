package qe

// Source: src/qe/qe_sat_tactic.cpp, solver_context::project_var_partial.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
)

// projectPartial removes one variable using the current model and publishes
// the normalized formula and updated atom index in the concrete SAT context.
func (c *satSolverContext) projectPartial(index uint32, source *model.Model, rw *rewriter.TheoryRewriter, config Config) (err error) {
	if c == nil || c.closed || source == nil || rw == nil {
		return ast.ErrASTHandle
	}
	if index >= uint32(len(c.variables)) {
		return arena.ErrBounds
	}
	variables := make([]ast.ASTHandle, len(c.variables))
	for i := range variables {
		variables[i], err = c.manager.RefGet(c.variables[i])
		if err != nil {
			return err
		}
	}
	context, err := NewSolverContext(c.manager, variables)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, context.Close()) }()
	sort, err := c.manager.GetSort(variables[index])
	if err != nil {
		return err
	}
	family, err := c.manager.GetFamilyID(sort)
	if err != nil {
		return err
	}
	arithmetic, err := c.manager.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	bv, err := c.manager.BitVectorUtilityFamilyID()
	if err != nil {
		return err
	}
	var projected bool
	switch family {
	case c.manager.BasicFamilyID():
		plugin, e := NewBoolPlugin(context)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, plugin.Close()) }()
		if e = plugin.Project(c.contains[index], source, c.formula, config); e != nil {
			return e
		}
		projected = true
	case arithmetic:
		plugin, e := NewArithmeticPlugin(context)
		if e != nil {
			return e
		}
		if e = context.AddPlugin(plugin); e != nil {
			return errors.Join(e, plugin.Close())
		}
		plugin.util.theory = rw
		adapter, e := NewSolverNNFAdapter(context)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, adapter.Close()) }()
		normalizer, e := NewQENNFNormalizer(c.manager, adapter, adapter, config)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, normalizer.Close()) }()
		atoms, e := normalizer.Normalize(c.formula)
		if e != nil {
			return e
		}
		if e = atoms.Close(); e != nil {
			return e
		}
		root, e := c.manager.RefGet(c.formula)
		if e != nil {
			return e
		}
		if e = rw.Rewrite(root, c.formula); e != nil {
			return e
		}
		projected, err = plugin.ProjectModelFormulaNNF(c.contains[index], source, c.formula, config)
		if err == nil && projected {
			err = c.takePartialArithmeticOutputs(plugin)
		}
	case bv:
		plugin, e := NewBitVectorPlugin(context)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, plugin.Close()) }()
		projected, err = plugin.ProjectModel(c.contains[index], source, c.formula, config)
	default:
		return tactic.NewTacticException("qe-sat projection has no plugin for variable sort")
	}
	if err != nil {
		return err
	}
	if !projected {
		return tactic.NewTacticException("qe-sat projection failed")
	}
	current, err := c.manager.RefGet(c.formula)
	if err != nil {
		return err
	}
	if err = rw.Rewrite(current, c.formula); err != nil {
		return err
	}
	current, err = c.manager.RefGet(c.formula)
	if err != nil {
		return err
	}
	return c.eliminateVariable(index, current, ast.ASTHandle{})
}
