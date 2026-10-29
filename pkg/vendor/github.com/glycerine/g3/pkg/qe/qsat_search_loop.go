package qe

// Source: src/qe/qsat.cpp qsat::check_sat/push/pop/validation/display.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *qsatSearch) checkpoint() error {
	canceled, err := s.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(s.manager.ResourceLimit().CancelMessage())
	}
	return nil
}
func (s *qsatSearch) verbose(text string) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(3)
	if err != nil || !enabled {
		return err
	}
	out, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return out.AppendCString(text)
}
func (s *qsatSearch) push() error { s.level++; return s.preparation.abstraction.Push() }
func (s *qsatSearch) pop(n uint32) error {
	if err := s.current.Set(nil); err != nil {
		return err
	}
	if n == 0 || n > s.level {
		return ast.ErrASTDomain
	}
	if err := s.preparation.abstraction.Pop(n); err != nil {
		return err
	}
	s.level -= n
	return nil
}
func (s *qsatSearch) display(w io.Writer) error {
	_, _ = fmt.Fprintf(w, "level: %d\n", s.level)
	for _, block := range s.preparation.variables {
		if err := writeExpressionVector(s.manager, s.config, w, block); err != nil {
			return err
		}
		_, _ = io.WriteString(w, "\n")
	}
	return s.preparation.abstraction.Display(w)
}
func (s *qsatSearch) validateDefinitions(msg string) (bool, error) {
	md, err := s.current.Get()
	if err != nil {
		return false, err
	}
	if md == nil {
		return true, nil
	}
	valid, err := s.preparation.abstraction.ValidateDefinitions(md)
	if err != nil || valid {
		return valid, err
	}
	err = s.preparation.abstraction.emit("validate_defs", 776, func(w io.Writer) error {
		_, _ = io.WriteString(w, msg+"\n")
		if e := s.display(w); e != nil {
			return e
		}
		if s.level > 0 {
			solver, e := s.kernel(s.level - 1).solver()
			if e != nil {
				return e
			}
			if e = solver.Display(w, nil); e != nil {
				return e
			}
		}
		asms := new([]ast.ASTRef)
		c := s.call()
		c.vectors = append(c.vectors, asms)
		if e := s.preparation.abstraction.GetAssumptions(md, asms); e != nil {
			return e
		}
		if e := writeExpressionVector(s.manager, s.config, w, *asms); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		if e := s.preparation.abstraction.PredicatesToLiterals(asms); e != nil {
			return e
		}
		if e := writeExpressionVector(s.manager, s.config, w, *asms); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
	return false, err
}
func (s *qsatSearch) requireDefinitions(msg string) error {
	if !ast.ASTSourceDebug {
		return nil
	}
	valid, err := s.validateDefinitions(msg)
	if err != nil {
		return err
	}
	if !valid {
		return ast.ErrASTDomain
	}
	return nil
}
func (c *qsatSearchCall) validateAssumptions(md *model.ModelCore, forms []ast.ASTRef) (bool, error) {
	s := c.owner
	e, err := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: s.config.Context, Globals: s.config.Globals, Memory: s.manager.NumeralService().RuntimeOwner(), Cooperation: s.config.Cooperation, Trace: s.config.Trace})
	if e != nil {
		c.evaluators = append(c.evaluators, e)
	}
	if err != nil {
		return false, err
	}
	out, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	for _, r := range forms {
		h, err := s.manager.RefGet(r)
		if err != nil {
			return false, err
		}
		if err = e.Evaluate(h, out); err != nil {
			return false, err
		}
		v, err := s.manager.RefGet(out)
		if err != nil {
			return false, err
		}
		if !s.manager.IsTrue(v) {
			err = s.preparation.abstraction.emit("validate_assumptions", 1069, func(w io.Writer) error {
				_, _ = io.WriteString(w, "component of core is not true: ")
				if e := writeExpression(s.manager, s.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			})
			return false, err
		}
	}
	return true, nil
}
func (s *qsatSearch) check() (status lbool.LBool, err error) {
	outer, err := s.begin()
	if err != nil {
		return lbool.Undef, err
	}
	defer s.finish(outer, &err)
	return s.checkLoop()
}

func (s *qsatSearch) checkLoop() (status lbool.LBool, err error) {
	for {
		s.rounds++
		if err = s.verbose(fmt.Sprintf("(check-qsat level: %d round: %d)\n", s.level, s.rounds)); err != nil {
			return lbool.Undef, err
		}
		if err = s.checkpoint(); err != nil {
			return lbool.Undef, err
		}
		c := s.call()
		asms := c.vector()
		if err = s.appendRefs(asms, s.assumptions); err != nil {
			return lbool.Undef, err
		}
		md, e := s.current.Get()
		if e != nil {
			return lbool.Undef, e
		}
		if err = s.preparation.abstraction.GetAssumptions(md, asms); err != nil {
			return lbool.Undef, err
		}
		if md != nil {
			if _, err = c.validateAssumptions(md, *asms); err != nil {
				return lbool.Undef, err
			}
		}
		if err = s.preparation.abstraction.emit("check_sat", 619, func(w io.Writer) error {
			if e := writeExpressionVector(s.manager, s.config, w, *asms); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		}); err != nil {
			return lbool.Undef, err
		}
		solver, e := s.kernel(s.level).solver()
		if e != nil {
			return lbool.Undef, e
		}
		hs := make([]ast.ASTHandle, len(*asms))
		for i, r := range *asms {
			hs[i], err = s.manager.RefGet(r)
			if err != nil {
				return lbool.Undef, err
			}
		}
		result, e := solver.CheckSat(hs)
		if e != nil {
			return lbool.Undef, e
		}
		switch result {
		case lbool.True:
			md, err = solver.AcquireModel()
			if err != nil {
				return lbool.Undef, err
			}
			if md == nil {
				return lbool.Undef, ast.ErrASTDomain
			}
			c.acquired = append(c.acquired, md)
			if err = s.current.Set(md); err != nil {
				return lbool.Undef, err
			}
			if err = s.requireDefinitions("check_sat"); err != nil {
				return lbool.Undef, err
			}
			if ast.ASTSourceDebug {
				valid, e := c.validateAssumptions(md, *asms)
				if e != nil {
					return lbool.Undef, e
				}
				if !valid {
					return lbool.Undef, ast.ErrASTDomain
				}
			}
			// Source validate_model is an unconditional true stub; its disabled
			// solver-validation body is not activated here.
			if err = s.preparation.abstraction.emit("check_sat", 628, func(w io.Writer) error {
				if e := solver.Display(w, nil); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				if e := s.display(w); e != nil {
					return e
				}
				if e := md.WriteV2(w, s.config.Context, s.config.Globals, false); e != nil {
					return e
				}
				return s.preparation.abstraction.DisplayAssumptions(w, *asms)
			}); err != nil {
				return lbool.Undef, err
			}
			if s.level == 0 {
				if err = s.saved.SetRef(&s.current); err != nil {
					return lbool.Undef, err
				}
			}
			if err = s.push(); err != nil {
				return lbool.Undef, err
			}
			if s.level == 1 && s.mode == qsatMaximize {
				if err = s.maximizeModel(c); err != nil {
					return lbool.Undef, err
				}
			}
		case lbool.False:
			if s.level == 0 {
				if err = s.closeRound(c); err != nil {
					return lbool.Undef, err
				}
				return lbool.False, nil
			}
			if s.level == 1 && s.mode == qsatSAT {
				if err = s.closeRound(c); err != nil {
					return lbool.Undef, err
				}
				return lbool.True, nil
			}
			md, e = s.current.Get()
			if e != nil {
				return lbool.Undef, e
			}
			if md == nil {
				if err = s.pop(1); err != nil {
					return lbool.Undef, err
				}
			} else if s.level == 1 {
				if ast.ASTSourceDebug {
					valid, e := c.validateAssumptions(md, *asms)
					if e != nil {
						return lbool.Undef, e
					}
					if !valid {
						return lbool.Undef, ast.ErrASTDomain
					}
				}
				if err = s.projectQE(c, asms); err != nil {
					return lbool.Undef, err
				}
			} else {
				ok, e := s.project(c, asms)
				if e != nil {
					return lbool.Undef, e
				}
				if !ok {
					if err = s.closeRound(c); err != nil {
						return lbool.Undef, err
					}
					return lbool.Undef, nil
				}
			}
		case lbool.Undef:
			if err = s.closeRound(c); err != nil {
				return lbool.Undef, err
			}
			return lbool.Undef, nil
		default:
			return lbool.Undef, ast.ErrASTDomain
		}
		// End the actual round's lexical owners before the next solver call.
		// Failed-round owners stay on pending for the outer cleanup receipt.
		if err = s.closeRound(c); err != nil {
			return lbool.Undef, err
		}
	}
}

func (s *qsatSearch) closeRound(c *qsatSearchCall) error {
	if err := c.close(); err != nil {
		return err
	}
	if len(s.pending) == 0 || s.pending[len(s.pending)-1] != c {
		return ast.ErrASTHandle
	}
	s.pending = s.pending[:len(s.pending)-1]
	return nil
}
