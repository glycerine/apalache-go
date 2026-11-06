package duality

// Source: src/muz/duality/duality_dl_interface.cpp, dl_interface::get_model.

import (
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// GetModel returns an independently owned snapshot of the active annotations.
func (e *QueryEngine) GetModel() (*model.Model, error) {
	if err := e.open(); err != nil {
		return nil, err
	}
	// Source display_certificate publishes annotations only for StatusModel.
	// A refutation owns a counterexample/proof instead; its working graph
	// annotations are not an inductive fixedpoint model.
	if e.active == nil || e.status != lbool.False {
		return nil, graph.ErrWrapperOwner
	}
	return e.active.graph.AnnotationModel()
}

func (b *Backend) GetModel() (*model.Model, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	return b.queries.GetModel()
}
