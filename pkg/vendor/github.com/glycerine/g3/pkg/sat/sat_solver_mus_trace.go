package sat

// Source: src/sat/sat_mus.cpp:40,61,71,93,120; retained util/trace sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *satSolverMUSMembers) traceMUS(line int, body func(io.Writer) error) {
	tags := m.solver.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	function := "mus1"
	if line == 40 {
		function = "set_core"
	}
	_ = tags.EmitConditional("sat", function, "src/sat/sat_mus.cpp", line,
		func() bool { return true }, body)
}

func satMUSTraceColumn(w io.Writer, column *satUintColumn) error {
	n, err := column.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		if i != 0 {
			if _, err = io.WriteString(w, " "); err != nil {
				return err
			}
		}
		v, e := column.Read(i)
		if e != nil {
			return e
		}
		if _, err = io.WriteString(w, Literal(v).String()); err != nil {
			return err
		}
	}
	return nil
}

func (m *satSolverMUSMembers) traceMUSCore(line int, prefix string, column *satUintColumn) {
	m.traceMUS(line, func(w io.Writer) error {
		if _, err := io.WriteString(w, prefix); err != nil {
			return err
		}
		if err := satMUSTraceColumn(w, column); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}

func (m *satSolverMUSMembers) traceMUSPair(line int, firstPrefix string, first *satUintColumn, secondPrefix string, second *satUintColumn) {
	m.traceMUS(line, func(w io.Writer) error {
		if _, err := io.WriteString(w, firstPrefix); err != nil {
			return err
		}
		if err := satMUSTraceColumn(w, first); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"+secondPrefix); err != nil {
			return err
		}
		if err := satMUSTraceColumn(w, second); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
