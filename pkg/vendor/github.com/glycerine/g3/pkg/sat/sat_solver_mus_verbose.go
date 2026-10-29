package sat

// Source: src/sat/sat_mus.cpp:52-70,95-145; src/util/util.h:176-186.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func satMUSVerboseStream(level uint32) (*utext.NumericTextStream, error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(level)
	if err != nil || !enabled {
		return nil, err
	}
	return diagnostic.BorrowProcessVerboseStream()
}

func satMUSWriteLiteral(out *utext.NumericTextStream, raw uint32) error {
	lit := Literal(raw)
	if lit.sign() {
		if err := out.AppendCString("-"); err != nil {
			return err
		}
	}
	return out.AppendUint32(uint32(lit.variable()))
}

func satMUSWriteCore(out *utext.NumericTextStream, core *satUintColumn) error {
	n, err := core.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		if i != 0 {
			if err = out.AppendCString(" "); err != nil {
				return err
			}
		}
		value, readErr := core.Read(i)
		if readErr != nil {
			return readErr
		}
		if err = satMUSWriteLiteral(out, value); err != nil {
			return err
		}
	}
	return nil
}

func (m *satSolverMUSMembers) emitMUSEntry() error {
	out, err := satMUSVerboseStream(3)
	if err != nil || out == nil {
		return err
	}
	n, err := m.solver.conflict.core.Size()
	if err != nil {
		return err
	}
	if err = out.AppendCString("(sat.mus size: "); err != nil {
		return err
	}
	if err = out.AppendUint32(n); err != nil {
		return err
	}
	if err = out.AppendCString(" core: ["); err != nil {
		return err
	}
	if err = satMUSWriteCore(out, &m.solver.conflict.core); err != nil {
		return err
	}
	return out.AppendCString("])\n")
}

func (m *satSolverMUSMembers) emitMUSProgress(coreSize uint32, partial bool) error {
	out, err := satMUSVerboseStream(1)
	if err != nil || out == nil {
		return err
	}
	musSize, err := m.mus.Size()
	if err != nil {
		return err
	}
	if err = out.AppendCString("(sat.mus num-to-process: "); err != nil {
		return err
	}
	if err = out.AppendUint32(coreSize); err != nil {
		return err
	}
	if err = out.AppendCString(" mus: "); err != nil {
		return err
	}
	if err = out.AppendUint32(musSize); err != nil {
		return err
	}
	if partial {
		if err = out.AppendCString(" max-restarts: "); err != nil {
			return err
		}
		if err = out.AppendUint32(m.maxNumRestarts); err != nil {
			return err
		}
	}
	return out.AppendCString(")\n")
}

func (m *satSolverMUSMembers) emitMUSResult(result lbool.LBool) error {
	out, err := satMUSVerboseStream(1)
	if err != nil || out == nil {
		return err
	}
	return out.AppendCString("(sat.mus " + result.String() + ")\n")
}

func (m *satSolverMUSMembers) emitMUSUnitReduction(lit Literal) error {
	out, err := satMUSVerboseStream(3)
	if err != nil || out == nil {
		return err
	}
	if err = out.AppendCString("(sat.mus unit reduction, literal is in both cores "); err != nil {
		return err
	}
	if err = satMUSWriteLiteral(out, uint32(lit)); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}

func (m *satSolverMUSMembers) emitMUSNewCore() error {
	out, err := satMUSVerboseStream(3)
	if err != nil || out == nil {
		return err
	}
	if err = out.AppendCString("(sat.mus.new "); err != nil {
		return err
	}
	if err = satMUSWriteCore(out, &m.solver.conflict.core); err != nil {
		return err
	}
	return out.AppendCString(")\n")
}
