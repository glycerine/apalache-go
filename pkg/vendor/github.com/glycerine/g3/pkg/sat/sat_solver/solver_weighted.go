package sat_solver

// Source: src/sat/sat_solver/inc_sat_solver.cpp:110-132,698-709.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// DisplayWeighted is inc_sat_display's rational-weight wrapper. Source accepts
// exactly unsigned 32-bit integer weights; it does not scale fractions.
func DisplayWeighted(out io.Writer, backend solver.Solver, soft []ast.ASTHandle, weights []unumeral.OwnedRational) error {
	s, ok := backend.(*Solver)
	if !ok || s == nil {
		return ast.ErrASTDomain
	}
	if err := s.open(); err != nil {
		return err
	}
	var unsigned []uint32
	if weights != nil {
		if len(weights) < len(soft) {
			return arena.ErrBounds
		}
		service := s.manager.NumeralService()
		for i := range soft {
			valid, err := service.IsUnsigned(&weights[i])
			if err != nil {
				return err
			}
			if !valid {
				return z3exception.NewDefaultException("Cannot display weights that are not integers")
			}
			value, err := service.Unsigned(&weights[i])
			if err != nil {
				return err
			}
			unsigned = append(unsigned, value)
		}
	}
	return s.DisplayWeighted(out, soft, unsigned)
}

// DisplayWeighted permanently asserts implication guards for soft constraints,
// compiles their activation assumptions, compacts matching weights, then prints
// the actual SAT graph. It neither checks satisfiability nor defaults weights.
func (s *Solver) DisplayWeighted(out io.Writer, soft []ast.ASTHandle, weights []uint32) (err error) {
	if err = s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if weights != nil && len(weights) < len(soft) {
		return arena.ErrBounds
	}
	if weights != nil {
		s.weights = append(s.weights, weights[:len(soft)]...)
	}
	defer func() {
		s.retainFailure(err)
		err = errors.Join(err, s.drainChecks(), s.drainGoals(), s.drainModelWork())
	}()
	if err = s.encoder.initPreprocess(); err != nil {
		return err
	}
	if err = s.core.PopToBaseLevel(); err != nil {
		return err
	}
	w := &checkWork{owner: s}
	s.checks = append(s.checks, w)
	for _, f := range soft {
		sort, e := s.manager.MkBoolSort()
		if e != nil {
			return e
		}
		a, e := s.manager.MkFreshConst("s", sort)
		if e != nil {
			return e
		}
		if e = s.hold(&w.held, a); e != nil {
			return e
		}
		guard, e := s.manager.MkImplies(a, f)
		if e != nil {
			return e
		}
		if e = s.Assert(guard); e != nil {
			return e
		}
		w.assumptions = append(w.assumptions, a)
	}
	status, err := s.internalizeFormulas()
	if err != nil {
		return err
	}
	if err = weightedVerify(status, "l_true == internalize_formulas()"); err != nil {
		return err
	}
	status, _, err = s.internalizeAssumptions(w.assumptions)
	if err != nil {
		return err
	}
	if err = weightedVerify(status, "l_true == internalize_assumptions(sz, asms.c_ptr(), dep2asm)"); err != nil {
		return err
	}
	if len(s.weights) < len(s.assumptions) {
		return arena.ErrBounds
	}
	activeWeights := append([]uint32(nil), s.weights[:len(s.assumptions)]...)
	s.weights = s.weights[:0]
	work := &modelConversion{}
	s.modelWork = append(s.modelWork, work)
	work.stream, err = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	if err != nil {
		return err
	}
	err = s.core.DisplayWCNF(work.stream, s.assumptions, activeWeights)
	// Preserve any completed printer prefix on a later formatting error.
	text, readErr := work.stream.String()
	if readErr != nil {
		return errors.Join(err, readErr)
	}
	_, writeErr := io.WriteString(out, text)
	return errors.Join(err, writeErr)
}

func weightedVerify(status lbool.LBool, expression string) error {
	if status == lbool.True {
		return nil
	}
	_, _ = io.WriteString(os.Stderr, "Failed to verify: "+expression+"\n")
	if ast.ASTSourceDebug {
		return ast.ErrASTDomain
	}
	return nil
}
