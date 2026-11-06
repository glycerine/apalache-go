package fp

// Source: pdr_context.cpp context::display_certificate applies its model
// converter to the finite inductive property before printing it.
import (
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/pdr"
)

// The engine owns this context and its conversion ledger. Read the live
// ledger because preprocessing installs it after context construction.
type pdrCertificateModelConverter struct{ engine *PropositionalEngine }

func (c pdrCertificateModelConverter) ConvertCertificateModel(source *model.Model) (*model.Model, error) {
	return c.engine.expandHornModel(source, c.engine.pdrConversions)
}

var _ pdr.CertificateModelConverter = pdrCertificateModelConverter{}
