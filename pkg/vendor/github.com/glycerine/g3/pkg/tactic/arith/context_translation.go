package arith

// Source: translate methods for the arithmetic finite-domain preprocessing tactics.
import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TranslateContextLeaf(value tactic.Tactic, m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, services model.ModelEvaluatorConfig) (result tactic.Tactic, matched bool, resultErr error) {
	if t, ok := value.(*DegreeShiftTactic); ok {
		if err := t.open(); err != nil {
			return nil, true, err
		}
		next, err := NewDegreeShiftTactic(m, services)
		return next, true, err
	}
	if t, ok := value.(*Eq2BVTactic); ok {
		if err := t.open(); err != nil {
			return nil, true, err
		}
		next, err := NewEq2BVTactic(m, c, g)
		if err != nil {
			return nil, true, err
		}
		return next, true, nil
	}
	var source *uparams.ParamsContext
	var params *uparams.ParamsRef
	switch t := value.(type) {
	case *LIA2CardTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *LIA2PBTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *Card2BVTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *PB2BVTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *FixDLVarTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *Recover01Tactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *NormalizeBoundsTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *DiffNeqTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *Elim01Tactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *PropagateIneqsTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.core.params
	case *FMTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	case *QFNRAPurifyTactic:
		if err := t.open(); err != nil {
			return nil, true, err
		}
		source, params = t.context, &t.params
	default:
		return nil, false, nil
	}
	p, err := source.TranslateTo(params, c)
	if err != nil {
		return nil, true, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, c.Del(&p))
		if resultErr != nil && result != nil {
			if e := result.IncRef(); e == nil {
				resultErr = errors.Join(resultErr, result.DecRef())
			} else {
				resultErr = errors.Join(resultErr, e)
			}
			result = nil
		}
	}()
	switch t := value.(type) {
	case *LIA2CardTactic:
		next, e := NewLIA2CardTactic(m, c, g, &p)
		err = e
		if err == nil {
			next.trace = t.trace
			next.bounds.trace = t.trace
			result = next
		}
	case *LIA2PBTactic:
		next, e := NewLIA2PBTactic(m, c, g, &p, services)
		err = e
		if err == nil {
			next.trace = t.trace
			next.bounds.trace = t.trace
			result = next
		}
	case *Card2BVTactic:
		next, e := NewCard2BVTactic(m, c, g, &p)
		err = e
		if err == nil {
			result = next
		}
	case *PB2BVTactic:
		next, e := NewPB2BVTactic(m, c, g, &p, services)
		err = e
		if err == nil {
			next.trace = t.trace
			next.bounds.trace = t.trace
			result = next
		}
	case *FixDLVarTactic:
		next, e := NewFixDLVarTactic(m, c, g, &p, services)
		err = e
		if err == nil {
			next.trace = t.trace
			result = next
		}
	case *Recover01Tactic:
		next, e := NewRecover01Tactic(m, c, g, &p, services)
		err = e
		if err == nil {
			result = next
		}
	case *NormalizeBoundsTactic:
		next, e := NewNormalizeBoundsTactic(m, c, g, &p, services)
		err = e
		if err == nil {
			next.trace = t.trace
			next.bounds.trace = t.trace
			result = next
		}
	case *DiffNeqTactic:
		next, e := NewDiffNeqTactic(m, c, g, &p)
		err = e
		if err == nil {
			next.trace = t.trace
			result = next
		}
	case *Elim01Tactic:
		next, e := NewElim01Tactic(m, c, g, &p, services)
		err = e
		if err == nil {
			result = next
		}
	case *PropagateIneqsTactic:
		next, e := newPropagateIneqsAdapter(m, c, &p, 47)
		err = e
		if err == nil {
			result = next
		}
	case *FMTactic:
		next, e := NewFMTactic(m, c, g, &p, services)
		err = e
		if err == nil {
			result = next
		}
	case *QFNRAPurifyTactic:
		next, e := NewQFNRAPurifyTactic(m, c, g, &p, services)
		err = e
		if err == nil {
			next.trace = t.trace
			result = next
		}
	}
	return result, true, err
}
