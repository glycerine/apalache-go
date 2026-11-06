package pdr

// Source: inductive_property::to_model in src/muz/pdr/pdr_manager.cpp:109.

import (
	"io"

	"github.com/glycerine/g3/pkg/model"
)

// traceInductiveModel observes the raw model before any model converter runs.
// Source ostream failures set badbit but do not change the solver result.
func (c *Context) traceInductiveModel(md *model.Model) error {
	tags := c.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "to_model", "src/muz/pdr/pdr_manager.cpp", 109,
		func() bool { return true }, func(out io.Writer) error {
			config := c.owner.contexts.config
			return md.Core().WriteSMT2(out, config.Params, config.Globals, 0)
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
