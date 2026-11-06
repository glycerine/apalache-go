package fp

// Source: src/muz/pdr/pdr_dl_interface.cpp:153,174 transformed-rule and
// empty-program model diagnostics.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type pdrVerboseRuleWriter struct{ stream *utext.NumericTextStream }

func (w pdrVerboseRuleWriter) Write(body []byte) (int, error) {
	if err := w.stream.AppendCString(string(body)); err != nil {
		return 0, err
	}
	return len(body), nil
}

func (e *PropositionalEngine) writePDRQueryRules(rules *base.RuleSet) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(2)
	if err != nil || !enabled {
		return err
	}
	outputs, err := rules.OutputPredicates()
	if err != nil || len(outputs) == 0 {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return e.writeRuleSetDisplay(pdrVerboseRuleWriter{stream: stream}, rules)
}

// The source model printer is reached only when the transformed program is
// empty but still has an output. Replay the preprocessing model converters
// before printing, just as context::get_model does through inductive_property.
func (e *PropositionalEngine) writePDREmptyProgramModel(declaration ast.ASTHandle) (resultErr error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	count, err := e.pdrRules.Len()
	if err != nil || count != 0 {
		return err
	}
	outputs, err := e.pdrRules.OutputPredicates()
	if err != nil || len(outputs) == 0 {
		return err
	}
	source, err := e.pdr.Model(e.pdrPredicate(declaration))
	if err != nil {
		return err
	}
	md, err := e.expandHornModel(source, e.pdrConversions)
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
