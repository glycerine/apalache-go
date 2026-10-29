package qe

// Source: src/qe/qe_mbp.cpp mbp::impl::operator()/validate_model.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func (c *modelProjectionCall) evaluate(md *model.ModelCore, h ast.ASTHandle) (ast.ASTHandle, error) {
	if c.scratch.evaluator == nil {
		if err := c.scratch.startEvaluator(md); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return c.scratch.eval(h)
}
func (c *modelProjectionCall) validate(md *model.ModelCore, forms []ast.ASTRef) error {
	m := c.scratch.owner.manager
	for _, r := range forms {
		h, err := m.RefGet(r)
		if err != nil {
			return err
		}
		val, err := c.evaluate(md, h)
		if err != nil {
			return err
		}
		if !m.IsTrue(val) {
			return ast.ErrASTDomain
		}
	}
	return nil
}
func (p *ModelBasedProjection) projectVariable(md *model.ModelCore, v ast.ASTHandle, vars, forms *[]ast.ASTRef) (bool, error) {
	s, err := p.manager.GetSort(v)
	if err != nil {
		return false, err
	}
	fid, err := p.manager.GetFamilyID(s)
	if err != nil {
		return false, err
	}
	switch fid {
	case p.arithmetic.family:
		return p.arithmetic.ProjectVariable(md, v, vars, forms)
	case p.datatype.family:
		return p.datatype.ProjectVariable(md, v, vars, forms)
	case p.array.family:
		return p.array.ProjectVariable(md, v, vars, forms)
	default:
		return false, nil
	}
}

// Project updates the caller's variables and model-satisfied formula cube.
// forceElimination substitutes one remaining variable by its model value when
// a complete source projection round cannot otherwise make progress.
func (p *ModelBasedProjection) Project(forceElimination bool, vars *[]ast.ASTRef, md *model.ModelCore, forms *[]ast.ASTRef) (err error) {
	c, err := p.begin(md)
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if vars == nil || forms == nil {
		return ast.ErrASTHandle
	}
	if ast.ASTSourceDebug {
		if err = c.validate(md, *forms); err != nil {
			return err
		}
	}
	if err = p.preprocess(md, vars, forms); err != nil {
		return err
	}
	if err = p.preprocessor.filterVariables(vars, forms); err != nil {
		return err
	}
	if err = p.preprocessor.projectBooleans(md, vars, forms); err != nil {
		return err
	}
	progress := true
	for progress && len(*vars) > 0 && len(*forms) > 0 {
		newVars := new([]ast.ASTRef)
		c.vectors = append(c.vectors, newVars)
		progress = false
		// Only the arithmetic plugin overrides the source batch hook.
		for _, fid := range p.families {
			if fid == p.arithmetic.family {
				if err = p.arithmetic.Project(md, vars, forms); err != nil {
					return err
				}
			}
		}
		for len(*vars) > 0 && len(*forms) > 0 {
			i := len(*vars) - 1
			v, e := p.manager.RefGet((*vars)[i])
			if e != nil {
				return e
			}
			if _, err = c.scratch.ref(v); err != nil {
				return err
			}
			if err = p.preprocessor.shrink(vars, i); err != nil {
				return err
			}
			ok, e := p.projectVariable(md, v, vars, forms)
			if e != nil {
				return e
			}
			if ok {
				progress = true
			} else if err = p.preprocessor.appendRef(newVars, v); err != nil {
				return err
			}
		}
		if !progress && len(*newVars) > 0 && len(*forms) > 0 && forceElimination {
			i := len(*newVars) - 1
			v, e := p.manager.RefGet((*newVars)[i])
			if e != nil {
				return e
			}
			if _, err = c.scratch.ref(v); err != nil {
				return err
			}
			if err = p.preprocessor.shrink(newVars, i); err != nil {
				return err
			}
			sub, e := c.scratch.replacement()
			if e != nil {
				return e
			}
			val, e := c.evaluate(md, v)
			if e != nil {
				return e
			}
			if err = sub.Insert(v, val); err != nil {
				return err
			}
			tmp, e := c.scratch.ref(ast.ASTHandle{})
			if e != nil {
				return e
			}
			for i := 0; i < len(*forms); i++ {
				h, e := p.manager.RefGet((*forms)[i])
				if e != nil {
					return e
				}
				if err = sub.Apply(h, tmp); err != nil {
					return err
				}
				h, e = p.manager.RefGet(tmp)
				if e != nil {
					return e
				}
				if err = p.preprocessor.rewriter.Rewrite(h, tmp); err != nil {
					return err
				}
				h, e = p.manager.RefGet(tmp)
				if e != nil {
					return e
				}
				if p.manager.IsTrue(h) {
					if err = p.preprocessor.erase(forms, &i); err != nil {
						return err
					}
				} else if err = p.manager.SetRef((*forms)[i], h); err != nil {
					return err
				}
			}
			progress = true
		}
		for _, r := range *newVars {
			h, e := p.manager.RefGet(r)
			if e != nil {
				return e
			}
			if err = p.preprocessor.appendRef(vars, h); err != nil {
				return err
			}
		}
		if progress {
			if err = p.preprocess(md, vars, forms); err != nil {
				return err
			}
		}
	}
	if len(*forms) == 0 {
		if err = p.preprocessor.shrink(vars, 0); err != nil {
			return err
		}
	}
	// Source unused_fmls is always empty: filtering only marks/compacts vars.
	if ast.ASTSourceDebug {
		if err = c.validate(md, *forms); err != nil {
			return err
		}
	}
	return p.preprocessor.emit("operator()", 517, func(w io.Writer) error {
		if e := p.preprocessor.displayForms(w, *vars); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " ")
		if e := p.preprocessor.displayForms(w, *forms); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}
