package fp

// Source: rel_context::display_output_facts and
// relation_manager::display_output_tables for the live finite frontend.

import (
	"io"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (e *PropositionalEngine) WriteDatalogOutputTables(names *base.DatalogContext, out io.Writer) error {
	if err := e.open(); err != nil {
		return err
	}
	if e.backend != "datalog" || !e.queried {
		return ErrPropositionalEngine
	}
	// The query scope has restored the source output-pragmas here. In
	// particular, QueryPredicates(all) must not print every requested result.
	return e.relations.DisplayOutputTables(names, e.set, out)
}
