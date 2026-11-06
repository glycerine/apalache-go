package fp

// Source: muz/duality/duality_dl_interface.cpp and dl_register_engine.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/duality"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type dualityVerboseWriter struct{ stream *utext.NumericTextStream }

func (w dualityVerboseWriter) Write(body []byte) (int, error) {
	// std::ostream reporter failures set sticky stream state but do not abort
	// the source search. NumericTextStream has the same transport contract.
	if err := w.stream.AppendCString(string(body)); err != nil {
		return 0, err
	}
	return len(body), nil
}

func (e *PropositionalEngine) closeDuality() error {
	if e.duality != nil {
		if err := e.duality.Close(); err != nil {
			return err
		}
		e.duality = nil
	}
	if e.dualityCooperation != nil {
		if err := e.dualityCooperation.Close(); err != nil {
			return err
		}
		e.dualityCooperation = nil
	}
	return nil
}

func (e *PropositionalEngine) queryDuality(query ast.ASTHandle) (status lbool.LBool, result error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, ErrPropositionalEngine
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { result = errors.Join(result, view.Close()) }()
	quantifyArrays, err := view.Bool("xform.quantify_arrays", false)
	if err != nil {
		return lbool.Undef, err
	}
	if quantifyArrays {
		// Source context::query flushes before check_rules and backend
		// construction. A rejected rule still crosses that display boundary.
		if err := e.flushPendingRules(); err != nil {
			return lbool.Undef, err
		}
		if err := e.validateFragment(); err != nil {
			return lbool.Undef, err
		}
	}
	mbqi, err := view.Bool("duality.mbqi", true)
	if err != nil {
		return lbool.Undef, err
	}
	bound, err := view.UInt("duality.recursion_bound", ^uint32(0))
	if err != nil {
		return lbool.Undef, err
	}
	if e.duality != nil && e.dualityMBQI != mbqi {
		if err := e.closeDuality(); err != nil {
			return lbool.Undef, err
		}
	}
	if e.duality == nil {
		if e.dualityCooperation == nil {
			e.dualityCooperation, err = cooperate.NewCooperation(e.manager.Budget())
			if err != nil {
				return lbool.Undef, err
			}
		}
		services := model.ModelEvaluatorConfig{Context: e.params, Globals: e.globals, Memory: e.manager.NumeralService().RuntimeOwner(), Cooperation: e.dualityCooperation}
		e.duality, err = duality.NewBackend(e.manager, smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals, ModelEvaluator: services}, mbqi)
		if err != nil {
			return lbool.Undef, err
		}
		e.dualityMBQI = mbqi
	}
	options, err := duality.ReadQueryOptions(e.params, e.globals, view.local)
	if err != nil {
		return lbool.Undef, err
	}
	options.Report, err = diagnostic.ProcessVerboseEnabled(1)
	if err != nil {
		return lbool.Undef, err
	}
	if options.Report {
		stream, cause := diagnostic.BorrowProcessVerboseStream()
		if cause != nil {
			return lbool.Undef, cause
		}
		if err = e.duality.SetReportOutput(dualityVerboseWriter{stream: stream}); err != nil {
			return lbool.Undef, err
		}
	} else if err = e.duality.SetReportOutput(nil); err != nil {
		return lbool.Undef, err
	}
	if err := e.duality.SetOptions(options); err != nil {
		return lbool.Undef, err
	}
	var prepared *duality.PreparedQuery
	if quantifyArrays {
		prepared, err = e.prepareDualityTransformedQuery(query, bound)
		if err != nil {
			return lbool.Undef, err
		}
	} else {
		forms, _, bounds, err := e.RawRules()
		if err != nil {
			return lbool.Undef, err
		}
		defer func() { result = errors.Join(result, closeASTRefs(e.manager, forms)) }()
		handles := make([]ast.ASTHandle, len(forms))
		for i, ref := range forms {
			handles[i], err = e.manager.RefGet(ref)
			if err != nil {
				return lbool.Undef, err
			}
		}
		prepared, err = duality.PrepareQuery(e.manager, handles, bounds, query, e.predicateOrder, bound)
		if err != nil {
			return lbool.Undef, err
		}
	}
	defer func() { result = errors.Join(result, prepared.Close()) }()
	background, err := e.manager.RefGet(e.background)
	if err != nil {
		return lbool.Undef, err
	}
	if !e.manager.IsTrue(background) {
		if err := prepared.AddBackground(background); err != nil {
			return lbool.Undef, err
		}
	}
	for _, ref := range e.assertions {
		h, err := e.manager.RefGet(ref)
		if err != nil {
			return lbool.Undef, err
		}
		if e.manager.IsTrue(h) {
			continue
		}
		if err := prepared.AddBackground(h); err != nil {
			return lbool.Undef, err
		}
	}
	return e.duality.Query(prepared)
}
