package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp:166-169,260-263 empty
// transformed-program model diagnostics.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// writeSpacerEmptyProgramModel presents the already installed empty-program
// model after replaying the Horn preprocessing converters. The caller supplies
// the source entry point's threshold: ordinary queries use two, while
// query_from_lvl uses one.
func (e *PropositionalEngine) writeSpacerEmptyProgramModel(verbosity uint32) (resultErr error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(verbosity)
	if err != nil || !enabled {
		return err
	}
	source, err := e.spacer.Model(e.spacerQuery)
	if err != nil {
		return err
	}
	md, err := e.expandSpacerModel(source)
	if err != nil {
		return err
	}
	owner := &pdrModelPresentation{manager: e.manager, model: md}
	defer func() { resultErr = owner.finish(resultErr) }()
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return md.Core().WriteSMT2(pdrVerboseRuleWriter{stream: stream}, e.params, e.globals, 0)
}
