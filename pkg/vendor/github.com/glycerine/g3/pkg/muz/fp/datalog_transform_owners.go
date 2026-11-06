package fp

import (
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// The source relational query owns its transformer. Engines constructed
// without parameter services additionally own default services for the
// transform's theory rewriter; those services must outlive the transformer.
type datalogTransformOwners struct {
	transformer *base.RuleTransformer
	context     *uparams.ParamsContext
	globals     *uparams.GlobalParamsService
	catalog     *uparams.GlobalParamsCatalog
	guard       *astrewriter.TheoryRewriter
}

func (o *datalogTransformOwners) services(e *PropositionalEngine) (*uparams.ParamsContext, *uparams.GlobalParamsService, error) {
	if e.params != nil || e.globals != nil {
		return e.params, e.globals, nil
	}
	var err error
	o.context, err = uparams.NewParamsContext(e.manager.Symbols, e.manager.NumeralService(), uparams.ParamsConfig{Budget: e.manager.Budget()})
	if err != nil {
		return nil, nil, err
	}
	o.catalog, err = uparams.NewGlobalParamsCatalog(e.manager.Symbols, uparams.GlobalParamsCatalogConfig{Budget: e.manager.Budget()})
	if err != nil {
		return nil, nil, err
	}
	o.globals, err = uparams.NewGlobalParamsService(e.manager.Symbols, o.context, uparams.GlobalParamsConfig{Budget: e.manager.Budget(), Catalog: o.catalog})
	if err != nil {
		return nil, nil, err
	}
	if err := o.globals.InitializeSource(); err != nil {
		return nil, nil, err
	}
	return o.context, o.globals, nil
}

func (o *datalogTransformOwners) Close() error {
	if o.transformer != nil {
		if err := o.transformer.Close(); err != nil {
			return err
		}
		o.transformer = nil
	}
	if o.guard != nil {
		if err := o.guard.Close(); err != nil {
			return err
		}
		o.guard = nil
	}
	if o.globals != nil {
		if err := o.globals.Close(); err != nil {
			return err
		}
		o.globals = nil
	}
	if o.catalog != nil {
		if err := o.catalog.Close(); err != nil {
			return err
		}
		o.catalog = nil
	}
	if o.context != nil {
		if err := o.context.Close(); err != nil {
			return err
		}
		o.context = nil
	}
	return nil
}

func (e *PropositionalEngine) clearDatalogTransformCleanup() error {
	if e.datalogTransformCleanup == nil {
		return nil
	}
	if err := e.datalogTransformCleanup.Close(); err != nil {
		return err
	}
	e.datalogTransformCleanup = nil
	return nil
}
