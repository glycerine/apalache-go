package qe

// Source: src/qe/qe_arith.cpp imp::maximize/validate_model.

import (
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
)

func (p *ArithmeticProjection) Maximize(md *model.ModelCore, forms []ast.ASTRef, objective ast.ASTHandle, ge, gt ast.ASTRef) (value simplex.Optimum, err error) {
	c, err := p.begin(md)
	if err != nil {
		return value, err
	}
	defer p.finish(c, &err)
	integer, arith, err := p.arithmetic(objective)
	if err != nil {
		return value, err
	}
	if !arith || integer {
		return value, ast.ErrASTDomain
	}
	if _, err = p.manager.GetDecl(objective); err != nil {
		return value, err
	}
	local := new([]ast.ASTRef)
	c.vectors = append(c.vectors, local)
	for _, r := range forms {
		h, e := p.manager.RefGet(r)
		if e != nil {
			return value, e
		}
		if e = p.appendRef(local, h); e != nil {
			return value, e
		}
	}
	if err = c.startEvaluator(md); err != nil {
		return value, err
	}
	constant := new(big.Rat)
	terms := projectionTerms()
	if err = c.linearizeTerm(big.NewRat(1, 1), objective, constant, local, terms); err != nil {
		return value, err
	}
	coeffs, err := c.coefficients(terms)
	if err != nil {
		return value, err
	}
	if err = c.mbo.SetObjective(coeffs, constant); err != nil {
		return value, err
	}
	if ast.ASTSourceDebug {
		if valid, e := c.validateModel(forms); e != nil {
			return value, e
		} else if !valid {
			return value, ast.ErrASTDomain
		}
	}
	for i := 0; i < len(*local); i++ {
		h, e := p.manager.RefGet((*local)[i])
		if e != nil {
			return value, e
		}
		if _, e = c.linearizeLiteral(h, local); e != nil {
			return value, e
		}
	}
	value, err = c.mbo.Maximize()
	if err != nil {
		return value, err
	}
	for h, id := range c.ids.All() {
		d, e := p.manager.GetDecl(h)
		if e != nil {
			return value, e
		}
		fid, e := p.manager.GetFamilyID(d)
		if e != nil {
			return value, e
		}
		n, e := p.manager.GetNumArgs(h)
		if e != nil {
			return value, e
		}
		if fid == ast.NullFamilyID && n == 0 {
			v, e := c.mbo.Value(id)
			if e != nil {
				return value, e
			}
			numeral, e := c.hold(p.manager.ArithmeticNumeralRat(v, false))
			if e != nil {
				return value, e
			}
			if e = md.RegisterConstant(d, numeral); e != nil {
				return value, e
			}
		} else {
			if e = p.emit("maximize", 470, func(w io.Writer) error {
				_, _ = io.WriteString(w, "omitting model update for non-uninterpreted constant ")
				if e := writeExpression(p.manager, p.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); e != nil {
				return value, e
			}
		}
	}
	bound, err := c.hold(p.manager.ArithmeticNumeralRat(&value.Value, false))
	if err != nil {
		return value, err
	}
	current, err := c.eval(objective)
	if err != nil {
		return value, err
	}
	var weak, strong ast.ASTHandle
	if value.Infinite {
		weak, err = c.hold(p.manager.MkArithmeticGE(objective, current))
		if err != nil {
			return value, err
		}
		strong, err = c.hold(p.manager.MkFalse())
	} else if value.Epsilon.Sign() < 0 {
		weak, err = c.hold(p.manager.MkArithmeticGE(objective, current))
		if err != nil {
			return value, err
		}
		strong, err = c.hold(p.manager.MkArithmeticGE(objective, bound))
	} else {
		weak, err = c.hold(p.manager.MkArithmeticGE(objective, bound))
		if err != nil {
			return value, err
		}
		strong, err = c.hold(p.manager.MkFamilyApp(p.family, 5, nil, []ast.ASTHandle{objective, bound}))
	}
	if err != nil {
		return value, err
	}
	if err = p.manager.SetRef(ge, weak); err != nil {
		return value, err
	}
	if err = p.manager.SetRef(gt, strong); err != nil {
		return value, err
	}
	if ast.ASTSourceDebug {
		if valid, e := c.validateModel(forms); e != nil {
			return value, e
		} else if !valid {
			return value, ast.ErrASTDomain
		}
	}
	return value, nil
}
func (c *arithmeticProjectionCall) validateModel(forms []ast.ASTRef) (bool, error) {
	p := c.owner
	valid := true
	for _, r := range forms {
		h, err := p.manager.RefGet(r)
		if err != nil {
			return false, err
		}
		v, err := c.eval(h)
		if err != nil {
			return false, err
		}
		if !p.manager.IsTrue(v) {
			valid = false
			if err = p.emit("validate_model", 501, func(w io.Writer) error {
				if e := writeExpression(p.manager, p.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " := ")
				if e := writeExpression(p.manager, p.config, w, v); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); err != nil {
				return false, err
			}
		}
	}
	return valid, nil
}
