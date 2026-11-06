package sat

// Source: src/sat/sat_scc.cpp:68-69,138,156,168,191,216,221; retained util/trace sites.

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (c *satSCC) trace(tag string, line int, condition func() bool, body func(io.Writer) error) {
	tags := c.lifecycle.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional(tag, "operator()", "src/sat/sat_scc.cpp", line, condition, body)
}

func (c *satSCC) traceSolverDisplay(tag string, line int) {
	if c.solver == nil {
		return
	}
	c.trace(tag, line, func() bool { return true }, c.solver.traceUserDisplayInto)
}

func (c *satSCC) traceSolverWatches() {
	if c.solver == nil {
		return
	}
	c.trace("scc_details", 69, func() bool { return true }, func(w io.Writer) (result error) {
		out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.lifecycle.budget})
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, out.Close()) }()
		if err = c.solver.displayWatches(out); err != nil {
			return err
		}
		body, err := out.String()
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, body)
		return err
	})
}

func (c *satSCC) traceCycle(li uint32) {
	c.trace("scc_cycle", 138, func() bool { return c.stack[len(c.stack)-1] != li }, func(w io.Writer) error {
		var body strings.Builder
		body.WriteString("cycle: ")
		for j := len(c.stack) - 1; j >= 0; j-- {
			x := c.stack[j]
			body.WriteString(Literal(x).String())
			body.WriteByte(' ')
			if x == li {
				break
			}
		}
		body.WriteByte('\n')
		_, err := io.WriteString(w, body.String())
		return err
	})
}

func (c *satSCC) traceSCCDetail(line int, body string) {
	c.trace("scc_detail", line, func() bool { return true }, func(w io.Writer) error {
		_, err := io.WriteString(w, body)
		return err
	})
}

func (c *satSCC) traceRoot(root Literal) {
	c.traceSCCDetail(191, "r: "+root.String()+"\n")
}

func (c *satSCC) traceRoots() {
	c.trace("scc", 216, func() bool { return true }, func(w io.Writer) error {
		var body strings.Builder
		for i, root := range c.roots {
			body.WriteString(strconv.Itoa(i))
			body.WriteString(" -> ")
			body.WriteString(root.String())
			body.WriteByte('\n')
		}
		body.WriteString("to_elim: ")
		for _, v := range c.toElim {
			body.WriteString(strconv.FormatUint(uint64(v), 10))
			body.WriteByte(' ')
		}
		body.WriteByte('\n')
		_, err := io.WriteString(w, body.String())
		return err
	})
}
