package duality

// Source: duality_rpfp.cpp:2797-2840.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrGreedyReduceFailed = errors.New("duality greedy reduction failed")
var ErrCoreNotUnsat = errors.New("should be unsat")

// GreedyReduceCache owns and mutates the caller's assumption/core expression
// vectors. Theory guards are appended only if the initial problem is not UNSAT.
func (s *EdgeSolverScope) GreedyReduceCache(assumptions, core *[]*Expr, theory []*Expr) (result error) {
	if s == nil || s.Solver == nil || s.Assumptions == nil || assumptions == nil || core == nil {
		return ErrWrapperOwner
	}
	var full []*Expr
	closeTerms := func(terms []*Expr) error {
		var result error
		for _, term := range terms {
			result = errors.Join(result, term.Close())
		}
		return result
	}
	defer func() { result = errors.Join(result, closeTerms(full)) }()
	check := func(literals []*Expr) (lbool.LBool, error) {
		status, next, err := s.CheckCore(literals)
		if err != nil {
			return status, err
		}
		if err = closeTerms(full); err != nil {
			return status, errors.Join(err, closeTerms(next))
		}
		full = next
		return status, nil
	}
	combined := func() []*Expr { lits := append([]*Expr(nil), (*assumptions)...); return append(lits, (*core)...) }
	status, err := check(combined())
	if err != nil {
		return err
	}
	if status != lbool.False {
		for _, axiom := range theory {
			literals, err := s.Assumptions.GetAssumptionLits(axiom, nil)
			if err != nil {
				return err
			}
			*assumptions = append(*assumptions, literals...)
		}
		literals := combined()
		for attempt := 0; attempt < 4; attempt++ {
			status, err = check(literals)
			if err != nil {
				return err
			}
			if status == lbool.False {
				break
			}
		}
		if status != lbool.False {
			return ErrGreedyReduceFailed
		}
	}
	// FilterCore preserves caller order and repeated entries.
	members := make(map[ast.ASTHandle]bool)
	for _, term := range full {
		h, err := term.Raw()
		if err != nil {
			return err
		}
		members[h] = true
	}
	filtered := make([]*Expr, 0, len(*core))
	for _, term := range *core {
		h, err := term.Raw()
		if err != nil {
			return err
		}
		if members[h] {
			filtered = append(filtered, term)
		} else {
			if err = term.Close(); err != nil {
				return err
			}
		}
	}
	*core = filtered
	status, verified, err := s.CheckCore(full)
	if err != nil {
		return err
	}
	if err = closeTerms(verified); err != nil {
		return err
	}
	if status != lbool.False {
		return ErrCoreNotUnsat
	}
	for index := 0; index < len(*core); {
		last := len(*core) - 1
		removed := (*core)[index]
		(*core)[index] = (*core)[last]
		*core = (*core)[:last]
		status, err = check(combined())
		if err != nil {
			*core = append(*core, removed)
			(*core)[index], (*core)[len(*core)-1] = (*core)[len(*core)-1], (*core)[index]
			return err
		}
		if status != lbool.False {
			*core = append(*core, removed)
			(*core)[index], (*core)[len(*core)-1] = (*core)[len(*core)-1], (*core)[index]
			index++
		} else {
			if err = removed.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}
