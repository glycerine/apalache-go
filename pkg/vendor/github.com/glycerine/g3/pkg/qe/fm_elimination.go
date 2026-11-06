package qe

// Source: src/qe/qe_lite.cpp fm::fm::try_eliminate / copy_remaining.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

// These are source constants, not public configuration parameters.
const (
	fmLimit   uint32 = 5000000
	fmCutoff1 uint32 = 8
	fmCutoff2 uint32 = 256
	fmExtra   uint32 = 0
)

func (s *fmStore) tryEliminate(x uint32) (bool, error) {
	if err := s.check(); err != nil {
		return false, err
	}
	if uint64(x) >= uint64(len(s.variables)) {
		return false, ast.ErrASTDomain
	}
	v := &s.variables[x]
	v.lowers = fmLiveConstraints(v.lowers)
	v.uppers = fmLiveConstraints(v.uppers)
	lower, upper := v.lowers, v.uppers
	if len(lower) == 0 || len(upper) == 0 {
		s.markDead(x)
		return true, s.recognizer.emit("try_eliminate", 2159, func(out io.Writer) error {
			_, _ = io.WriteString(out, "variable was eliminated (trivial case)\n")
			return nil
		})
	}
	nl, nu := uint32(len(lower)), uint32(len(upper))
	if nl > fmCutoff1 && nu > fmCutoff1 {
		return false, nil
	}
	if nl*nu > fmCutoff2 {
		return false, nil
	}
	yes, err := s.canEliminate(x)
	if err != nil || !yes {
		return false, err
	}
	s.counter += nl * nu
	if err = s.recognizer.emit("try_eliminate", 2177, func(out io.Writer) error {
		h, err := s.recognizer.manager.RefGet(v.expression)
		if err != nil {
			return err
		}
		_, _ = io.WriteString(out, "eliminating ")
		if err = writeExpression(s.recognizer.manager, s.recognizer.config, out, h); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\nlowers:\n")
		if err = s.displayConstraints(out, lower); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "uppers:\n")
		return s.displayConstraints(out, upper)
	}); err != nil {
		return false, err
	}
	limit := nl + nu + fmExtra
	var generated []*fmConstraint
	for _, l := range lower {
		for _, u := range upper {
			// Source checks before the next pair, with > rather than >=; the
			// final generated batch can therefore exceed the limit by one.
			if s.inconsistent || uint32(len(generated)) > limit {
				if err = s.recognizer.emit("try_eliminate", 2187, func(out io.Writer) error {
					_, _ = io.WriteString(out, "too many new constraints: "+strconv.Itoa(len(generated))+"\n")
					return nil
				}); err != nil {
					return false, err
				}
				for _, c := range generated {
					if err = s.deleteConstraint(c); err != nil {
						return false, err
					}
				}
				return false, nil
			}
			c, err := s.resolve(l, u, x)
			if err != nil {
				return false, err
			}
			if c != nil {
				generated = append(generated, c)
			}
		}
	}
	s.markDead(x)
	s.counter += uint32(len(generated))
	for _, c := range generated {
		if err = s.backwardSubsumption(c); err != nil {
			return false, err
		}
		if _, err = s.register(c); err != nil {
			return false, err
		}
	}
	err = s.recognizer.emit("try_eliminate", 2212, func(out io.Writer) error {
		_, _ = io.WriteString(out, "variables was eliminated old: "+strconv.FormatUint(uint64(nl+nu), 10)+" new_constraints: "+strconv.Itoa(len(generated))+"\n")
		return nil
	})
	return true, err
}

func (s *fmStore) displayConstraints(out io.Writer, cs []*fmConstraint) error {
	for _, c := range cs {
		_, _ = io.WriteString(out, "  ")
		if err := s.displayConstraint(out, c); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
	}
	return nil
}
func (s *fmStore) display(out io.Writer) error {
	for _, v := range s.variables {
		if v.forbidden {
			continue
		}
		h, err := s.recognizer.manager.RefGet(v.expression)
		if err != nil {
			return err
		}
		if err = writeExpression(s.recognizer.manager, s.recognizer.config, out, h); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		if err = s.displayConstraints(out, v.lowers); err != nil {
			return err
		}
		if err = s.displayConstraints(out, v.uppers); err != nil {
			return err
		}
	}
	return nil
}

func (s *fmStore) copyRemaining() (err error) {
	scope, err := s.begin()
	if err != nil {
		return err
	}
	defer s.finish(scope, &err)
	// Source copies all upper use lists before all lower use lists.
	for _, lower := range []bool{false, true} {
		for i := range s.variables {
			v := &s.variables[i]
			cs := v.uppers
			if lower {
				cs = v.lowers
			}
			for _, c := range cs {
				if c.dead {
					continue
				}
				c.dead = true
				h, err := s.expressionCore(scope, c)
				if err != nil {
					return err
				}
				if err = s.recognizer.emit("copy_remaining", 2228, func(out io.Writer) error {
					_, _ = io.WriteString(out, "asserting...\n")
					if err := writeExpression(s.recognizer.manager, s.recognizer.config, out, h); err != nil {
						return err
					}
					_, _ = io.WriteString(out, "\nnew_dep: 0\n")
					return nil
				}); err != nil {
					return err
				}
				ref, err := s.recognizer.manager.NewRef(h)
				if err != nil {
					return err
				}
				s.outputs = append(s.outputs, ref)
			}
			if lower {
				v.lowers = nil
			} else {
				v.uppers = nil
			}
		}
	}
	return nil
}
