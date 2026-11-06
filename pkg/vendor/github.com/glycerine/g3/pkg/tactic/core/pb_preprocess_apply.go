package core

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *PBPreprocessTactic) simplify(g *tactic.Goal, converter *tactic.ExtensionModelConverter) (progress bool, result error) {
	if e := t.normalize(g); e != nil {
		return false, fmt.Errorf("pb normalize: %w", e)
	}
	variables, constraints, e := t.classify(g)
	if e != nil {
		return false, fmt.Errorf("pb classify: %w", e)
	}
	if len(constraints) == 0 || len(variables) == 0 {
		return false, nil
	}
	var pinned []ast.ASTHandle
	for variable := range variables {
		if e = t.manager.IncRef(variable); e != nil {
			for i := len(pinned) - 1; i >= 0; i-- {
				_ = t.manager.DecRef(pinned[i])
			}
			return false, e
		}
		pinned = append(pinned, variable)
	}
	defer func() {
		for i := len(pinned) - 1; i >= 0; i-- {
			result = errors.Join(result, t.manager.DecRef(pinned[i]))
		}
	}()
	truth, e := t.manager.MkTrue()
	if e != nil {
		return false, e
	}
	falsehood, e := t.manager.MkFalse()
	if e != nil {
		return false, e
	}
	for variable, occ := range variables {
		var positions []uint32
		var value ast.ASTHandle
		if len(occ.positive) == 0 {
			positions = occ.negative
			value = falsehood
		} else if len(occ.negative) == 0 {
			positions = occ.positive
			value = truth
		} else {
			continue
		}
		decl, x := t.manager.GetDecl(variable)
		if x != nil {
			return false, x
		}
		if x = t.manager.IncRef(decl); x != nil {
			return false, x
		}
		changed, x := t.replace(g, positions, variable, value)
		if x != nil {
			return false, errors.Join(fmt.Errorf("pb replace: %w", x), t.manager.DecRef(decl))
		}
		// The source records every one-sided variable even when an earlier
		// substitution already reduced its shared constraint to true.  The
		// model converter must therefore not depend on whether replace made a
		// second observable goal mutation.
		if x = converter.Insert(decl, value); x != nil {
			return false, errors.Join(fmt.Errorf("pb model: %w", x), t.manager.DecRef(decl))
		}
		if x = t.manager.DecRef(decl); x != nil {
			return false, x
		}
		progress = progress || changed
	}
	if progress {
		variables, constraints, e = t.classify(g)
		if e != nil {
			return false, fmt.Errorf("pb reclassify: %w", e)
		}
	}
	resolved, e := t.resolveOne(g, converter, variables, constraints)
	if e != nil {
		return false, fmt.Errorf("pb resolve: %w", e)
	}
	progress = progress || resolved
	if resolved {
		variables, constraints, e = t.classify(g)
		if e != nil {
			return false, fmt.Errorf("pb resolve reclassify: %w", e)
		}
	}
	changed, e := t.subsume(g, variables, constraints)
	if e != nil {
		return false, fmt.Errorf("pb subsume: %w", e)
	}
	progress = progress || changed
	if e = g.ElimTrue(); e != nil {
		return false, fmt.Errorf("pb eliminate true: %w", e)
	}
	return progress, nil
}
