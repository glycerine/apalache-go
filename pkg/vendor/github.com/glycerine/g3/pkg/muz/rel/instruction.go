package rel

// Source: src/muz/rel/dl_instruction.h and src/muz/rel/dl_instruction.cpp
// execution_context, instruction, instruction_block, dealloc, clone, and move.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

var ErrInstruction = errors.New("muz/rel: invalid instruction operation")

type ExecutionStats struct {
	Join                     uint32
	Project                  uint32
	Filter                   uint32
	Total                    uint32
	UnarySingleton           uint32
	FilterByNegation         uint32
	SelectEqualProject       uint32
	JoinProject              uint32
	ProjectRename            uint32
	Union                    uint32
	FilterInterpretedProject uint32
	FilterIdentical          uint32
	FilterEqual              uint32
	Minimum                  uint32
}

// ExecutionContext owns every non-nil table placed in a register.
type ExecutionContext struct {
	registers   []*HashTable
	annotations map[uint32]string
	relations   *RelationManager
	terminate   func() bool
	stats       ExecutionStats
	closed      bool
}

func NewExecutionContext(terminate func() bool) *ExecutionContext {
	return &ExecutionContext{annotations: make(map[uint32]string), terminate: terminate}
}

// NewExecutionContextWithRelations borrows relations for load/store
// instructions executed by the context.
func NewExecutionContextWithRelations(relations *RelationManager, terminate func() bool) (*ExecutionContext, error) {
	if err := relations.open(); err != nil {
		return nil, err
	}
	return &ExecutionContext{annotations: make(map[uint32]string), relations: relations, terminate: terminate}, nil
}

func (c *ExecutionContext) open() error {
	if c == nil || c.closed {
		return ErrInstruction
	}
	return nil
}

func (c *ExecutionContext) Register(index uint32) (*HashTable, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if uint64(index) >= uint64(len(c.registers)) {
		return nil, nil
	}
	return c.registers[index], nil
}

// ReleaseRegister transfers ownership out of a register and leaves it empty.
func (c *ExecutionContext) ReleaseRegister(index uint32) (*HashTable, error) {
	value, err := c.Register(index)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, ErrInstruction
	}
	c.registers[index] = nil
	return value, nil
}

// SetRegister transfers ownership of value into index.
func (c *ExecutionContext) SetRegister(index uint32, value *HashTable) error {
	if err := c.open(); err != nil {
		return err
	}
	if index == ^uint32(0) {
		return ErrInstruction
	}
	if uint64(index) >= uint64(len(c.registers)) {
		c.registers = append(c.registers, make([]*HashTable, int(uint64(index)-uint64(len(c.registers))+1))...)
	}
	c.registers[index] = value
	return nil
}

func (c *ExecutionContext) MakeEmpty(index uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	if uint64(index) < uint64(len(c.registers)) {
		c.registers[index] = nil
	}
	return nil
}

func (c *ExecutionContext) RegisterCount() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return uint32(len(c.registers)), nil
}

func (c *ExecutionContext) RegisterAnnotation(index uint32) (string, bool, error) {
	if err := c.open(); err != nil {
		return "", false, err
	}
	value, found := c.annotations[index]
	return value, found, nil
}

func (c *ExecutionContext) SetRegisterAnnotation(index uint32, value string) error {
	if err := c.open(); err != nil {
		return err
	}
	c.annotations[index] = value
	return nil
}

func (c *ExecutionContext) ShouldTerminate() bool {
	return c == nil || c.closed || c.terminate != nil && c.terminate()
}

func (c *ExecutionContext) Statistics() (ExecutionStats, error) {
	if err := c.open(); err != nil {
		return ExecutionStats{}, err
	}
	return c.stats, nil
}

func (c *ExecutionContext) CollectStatistics(out *statistics.Statistics) error {
	if err := c.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	out.UpdateUint("dl.joins", c.stats.Join)
	out.UpdateUint("dl.project", c.stats.Project)
	out.UpdateUint("dl.filter", c.stats.Filter)
	out.UpdateUint("dl.total", c.stats.Total)
	out.UpdateUint("dl.unary_singleton", c.stats.UnarySingleton)
	out.UpdateUint("dl.filter_by_negation", c.stats.FilterByNegation)
	out.UpdateUint("dl.select_equal_project", c.stats.SelectEqualProject)
	out.UpdateUint("dl.join_project", c.stats.JoinProject)
	out.UpdateUint("dl.project_rename", c.stats.ProjectRename)
	out.UpdateUint("dl.union", c.stats.Union)
	out.UpdateUint("dl.filter_interpreted_project", c.stats.FilterInterpretedProject)
	out.UpdateUint("dl.filter_id", c.stats.FilterIdentical)
	out.UpdateUint("dl.filter_eq", c.stats.FilterEqual)
	return nil
}

func (c *ExecutionContext) Reset() error {
	if err := c.open(); err != nil {
		return err
	}
	clear(c.registers)
	c.registers = c.registers[:0]
	clear(c.annotations)
	c.stats = ExecutionStats{}
	return nil
}

func (c *ExecutionContext) Close() error {
	if c == nil {
		return ErrInstruction
	}
	if c.closed {
		return nil
	}
	clear(c.registers)
	c.registers = nil
	clear(c.annotations)
	c.annotations = nil
	c.closed = true
	return nil
}

// Instruction is the concrete relational bytecode contract consumed by a
// source-order InstructionBlock.
type Instruction interface {
	Perform(*ExecutionContext) (bool, error)
	MakeAnnotations(*ExecutionContext) error
	DisplayHead(io.Writer) error
	Accounting() *base.AccountedObject
	ProcessAllCosts() error
	CollectStatistics(*statistics.Statistics) error
	Close() error
}

type registerInstruction struct {
	accounting *base.AccountedObject
	kind       string
	source     uint32
	target     uint32
	closed     bool
}

func NewDeallocateInstruction(index uint32) Instruction {
	return &registerInstruction{accounting: base.NewAccountedObject(), kind: "dealloc", source: index}
}

func NewCloneInstruction(source, target uint32) Instruction {
	return &registerInstruction{accounting: base.NewAccountedObject(), kind: "clone", source: source, target: target}
}

func NewMoveInstruction(source, target uint32) Instruction {
	return &registerInstruction{accounting: base.NewAccountedObject(), kind: "move", source: source, target: target}
}

func (i *registerInstruction) open() error {
	if i == nil || i.closed || i.accounting == nil {
		return ErrInstruction
	}
	return nil
}

func (i *registerInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	source, err := context.Register(i.source)
	if err != nil {
		return false, err
	}
	switch i.kind {
	case "dealloc":
		return true, context.MakeEmpty(i.source)
	case "clone":
		var value *HashTable
		if source != nil {
			value, err = source.Clone()
			if err != nil {
				return false, err
			}
		}
		return true, context.SetRegister(i.target, value)
	case "move":
		if source == nil {
			return true, context.SetRegister(i.target, nil)
		}
		value, x := context.ReleaseRegister(i.source)
		if x != nil {
			return false, x
		}
		return true, context.SetRegister(i.target, value)
	default:
		return false, ErrInstruction
	}
}

func (i *registerInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	if i.kind == "dealloc" {
		return context.SetRegisterAnnotation(i.source, "alloc")
	}
	value, found, err := context.RegisterAnnotation(i.source)
	if err != nil {
		return err
	}
	if found {
		return context.SetRegisterAnnotation(i.target, value)
	}
	value, found, err = context.RegisterAnnotation(i.target)
	if err != nil || !found {
		return err
	}
	return context.SetRegisterAnnotation(i.source, value)
}

func (i *registerInstruction) DisplayHead(out io.Writer) error {
	if err := i.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if i.kind == "dealloc" {
		_, err := fmt.Fprintf(out, "dealloc %d", i.source)
		return err
	}
	_, err := fmt.Fprintf(out, "%s %d into %d", i.kind, i.source, i.target)
	return err
}

func (i *registerInstruction) Accounting() *base.AccountedObject { return i.accounting }

func (i *registerInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	return i.accounting.ProcessCosts()
}

func (i *registerInstruction) CollectStatistics(out *statistics.Statistics) error {
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

func (i *registerInstruction) Close() error {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	i.closed = true
	return i.accounting.Close()
}

type InstructionBlock struct {
	instructions []Instruction
	closed       bool
}

func NewInstructionBlock() *InstructionBlock { return &InstructionBlock{} }

func (b *InstructionBlock) open() error {
	if b == nil || b.closed {
		return ErrInstruction
	}
	return nil
}

// Append transfers ownership of instruction to the block.
func (b *InstructionBlock) Append(instruction Instruction) error {
	if err := b.open(); err != nil || instruction == nil || instruction.Accounting() == nil {
		return errors.Join(ErrInstruction, err)
	}
	b.instructions = append(b.instructions, instruction)
	return nil
}

func (b *InstructionBlock) Len() (int, error) {
	if err := b.open(); err != nil {
		return 0, err
	}
	return len(b.instructions), nil
}

func (b *InstructionBlock) Perform(context *ExecutionContext) (result bool, resultErr error) {
	if err := b.open(); err != nil {
		return false, err
	}
	if err := context.open(); err != nil {
		return false, err
	}
	recorder := base.NewCostRecorder()
	defer func() { resultErr = errors.Join(resultErr, recorder.Close()) }()
	for _, instruction := range b.instructions {
		if err := recorder.Start(instruction.Accounting()); err != nil {
			return false, err
		}
		if context.ShouldTerminate() {
			return false, nil
		}
		complete, err := instruction.Perform(context)
		if err != nil || !complete {
			return complete, err
		}
	}
	return true, nil
}

func (b *InstructionBlock) MakeAnnotations(context *ExecutionContext) error {
	if err := b.open(); err != nil {
		return err
	}
	for _, instruction := range b.instructions {
		if err := instruction.MakeAnnotations(context); err != nil {
			return err
		}
	}
	return nil
}

func (b *InstructionBlock) ProcessAllCosts() error {
	if err := b.open(); err != nil {
		return err
	}
	for _, instruction := range b.instructions {
		if err := instruction.ProcessAllCosts(); err != nil {
			return err
		}
	}
	return nil
}

func (b *InstructionBlock) CollectStatistics(out *statistics.Statistics) error {
	if err := b.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	for _, instruction := range b.instructions {
		if err := instruction.CollectStatistics(out); err != nil {
			return err
		}
	}
	return nil
}

func (b *InstructionBlock) Display(out io.Writer) error {
	if err := b.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	for _, instruction := range b.instructions {
		if err := instruction.DisplayHead(out); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (b *InstructionBlock) Reset() (result error) {
	if err := b.open(); err != nil {
		return err
	}
	for i := 0; i < len(b.instructions); i++ {
		result = errors.Join(result, b.instructions[i].Close())
	}
	b.instructions = nil
	return result
}

func (b *InstructionBlock) Close() error {
	if b == nil {
		return ErrInstruction
	}
	if b.closed {
		return nil
	}
	result := b.Reset()
	b.closed = true
	return result
}
