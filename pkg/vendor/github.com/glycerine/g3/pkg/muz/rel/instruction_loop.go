package rel

// Source: src/muz/rel/dl_instruction.cpp instr_while_loop.

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// whileInstruction owns its body and repeats it while at least one control
// register contains a non-empty table.
type whileInstruction struct {
	accounting *base.AccountedObject
	controls   []uint32
	body       *InstructionBlock
	closed     bool
}

// NewWhileLoopInstruction transfers ownership of body to the returned instruction.
func NewWhileLoopInstruction(controls []uint32, body *InstructionBlock) Instruction {
	return &whileInstruction{
		accounting: base.NewAccountedObject(),
		controls:   append([]uint32(nil), controls...),
		body:       body,
	}
}

func (i *whileInstruction) open() error {
	if i == nil || i.closed || i.accounting == nil || i.body == nil {
		return ErrInstruction
	}
	return i.body.open()
}

func (i *whileInstruction) controlIsEmpty(context *ExecutionContext) (bool, error) {
	for _, register := range i.controls {
		table, err := context.Register(register)
		if err != nil {
			return false, err
		}
		if table == nil {
			continue
		}
		count, err := table.Len()
		if err != nil {
			return false, err
		}
		if count != 0 {
			return false, nil
		}
	}
	return true, nil
}

func (i *whileInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	for {
		empty, err := i.controlIsEmpty(context)
		if err != nil || empty {
			return empty, err
		}
		complete, err := i.body.Perform(context)
		if err != nil || !complete {
			return complete, err
		}
	}
}

func (i *whileInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	return i.body.MakeAnnotations(context)
}

func (i *whileInstruction) DisplayHead(out io.Writer) error {
	if err := i.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	parts := make([]string, len(i.controls))
	for index, register := range i.controls {
		parts[index] = fmt.Sprint(register)
	}
	_, err := fmt.Fprintf(out, "while[%s]", strings.Join(parts, ","))
	return err
}

func (i *whileInstruction) Accounting() *base.AccountedObject { return i.accounting }

func (i *whileInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	if err := i.accounting.ProcessCosts(); err != nil {
		return err
	}
	return i.body.ProcessAllCosts()
}

func (i *whileInstruction) CollectStatistics(out *statistics.Statistics) error {
	if err := i.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	cost, err := i.accounting.TotalCosts()
	if err != nil {
		return err
	}
	out.UpdateUint("instruction", cost.Instructions)
	out.UpdateUint("instruction-time", cost.Milliseconds)
	return nil
}

func (i *whileInstruction) Close() error {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	i.closed = true
	return errors.Join(i.body.Close(), i.accounting.Close())
}
