package tactic

// Source: src/tactic/filter_model_converter.cpp:26,51,59-65; src/util/trace.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// BindTrace borrows the retained filter trace stream and source parameter
// owner. Nil values restore the process bindings.
func (c *tacticFilterModelConverter) BindTrace(tags *diagnostic.TraceTags, globals *uparams.GlobalParamsService) error {
	if err := c.open(); err != nil {
		return err
	}
	if globals != nil && globals.Context == nil {
		return ast.ErrASTHandle
	}
	c.trace, c.globals = tags, globals
	return nil
}

func (c *tacticFilterModelConverter) filterTraceTags() *diagnostic.TraceTags {
	if c.trace != nil {
		return c.trace
	}
	return c.manager.BorrowMemoryTraceTags()
}

func (c *tacticFilterModelConverter) displayFilter(out io.Writer) error {
	if _, err := io.WriteString(out, "(filter-model-converter"); err != nil {
		return err
	}
	for _, d := range c.declarations {
		name, err := c.manager.GetASTName(d)
		if err != nil {
			return err
		}
		spelling, err := c.manager.Symbols.String(name)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(out, " "+spelling); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")\n")
	return err
}

func (c *tacticFilterModelConverter) traceFilterModel(md *model.ModelCore, line int) error {
	tags := c.filterTraceTags()
	if !tags.ShouldTrace("filter_mc") {
		return nil
	}
	return tags.EmitConditional("filter_mc", "operator()", "src/tactic/filter_model_converter.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			globals := c.globals
			if globals == nil {
				globals = uparams.BorrowProcessGlobalParamsService()
			}
			if globals == nil || globals.Context == nil {
				return uparams.ErrGlobalParamsOwner
			}
			if line == 26 {
				if _, err := io.WriteString(w, "before filter_model_converter\n"); err != nil {
					return err
				}
			} else {
				if _, err := io.WriteString(w, "after filter_model_converter\n"); err != nil {
					return err
				}
			}
			if err := md.WriteV2(w, globals.Context, globals, false); err != nil {
				return err
			}
			if line == 26 {
				return c.displayFilter(w)
			}
			return nil
		})
}
