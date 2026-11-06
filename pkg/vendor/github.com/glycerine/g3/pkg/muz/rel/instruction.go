package rel

// Source: src/muz/rel/dl_instruction.h and src/muz/rel/dl_instruction.cpp
// execution_context, instruction, instruction_block, dealloc, clone, and move.

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
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
	registers                    []FiniteTable
	explanationRegisters         []*FiniteExplanationProduct
	relationExplanationRegisters []*RelationExplanationProduct
	annotations                  map[uint32]string
	relations                    *RelationManager
	terminate                    func() bool
	memory                       *memory.Runtime
	timeLimit                    time.Duration
	started                      time.Time
	stats                        ExecutionStats
	closed                       bool
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
	var runtime *memory.Runtime
	if numerals := relations.manager.NumeralService(); numerals != nil && numerals.RuntimeOwner() != nil {
		runtime = numerals.RuntimeOwner()
	}
	return &ExecutionContext{annotations: make(map[uint32]string), relations: relations, terminate: terminate, memory: runtime}, nil
}

func (c *ExecutionContext) open() error {
	if c == nil || c.closed {
		return ErrInstruction
	}
	return nil
}

func (c *ExecutionContext) Register(index uint32) (*HashTable, error) {
	value, err := c.FiniteRegister(index)
	if err != nil || value == nil {
		return nil, err
	}
	table, ok := value.(*HashTable)
	if !ok {
		return nil, ErrInstruction
	}
	return table, nil
}

// FiniteRegister borrows the concrete finite table stored at index.
func (c *ExecutionContext) FiniteRegister(index uint32) (FiniteTable, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if uint64(index) < uint64(len(c.explanationRegisters)) && c.explanationRegisters[index] != nil {
		return nil, ErrInstruction
	}
	if hasRelationExplanationRegister(c, index) {
		return nil, ErrInstruction
	}
	if uint64(index) >= uint64(len(c.registers)) {
		return nil, nil
	}
	return c.registers[index], nil
}

// ReleaseRegister transfers ownership out of a register and leaves it empty.
func (c *ExecutionContext) ReleaseRegister(index uint32) (*HashTable, error) {
	value, err := c.FiniteRegister(index)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	table, ok := value.(*HashTable)
	if !ok {
		return nil, ErrInstruction
	}
	c.registers[index] = nil
	return table, nil
}

// ReleaseFiniteRegister transfers the concrete finite owner out of index.
func (c *ExecutionContext) ReleaseFiniteRegister(index uint32) (FiniteTable, error) {
	value, err := c.FiniteRegister(index)
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
	if value == nil {
		return c.SetFiniteRegister(index, nil)
	}
	return c.SetFiniteRegister(index, value)
}

// SetFiniteRegister transfers ownership of a concrete finite table into index.
func (c *ExecutionContext) SetFiniteRegister(index uint32, value FiniteTable) error {
	if err := c.open(); err != nil {
		return err
	}
	switch concrete := value.(type) {
	case *HashTable:
		if concrete == nil {
			value = nil
		}
	case *SparseTable:
		if concrete == nil {
			value = nil
		}
	}
	if index == ^uint32(0) {
		return ErrInstruction
	}
	if err := c.clearRelationExplanationRegister(index); err != nil {
		return err
	}
	if uint64(index) >= uint64(len(c.registers)) {
		c.registers = append(c.registers, make([]FiniteTable, int(uint64(index)-uint64(len(c.registers))+1))...)
	}
	if uint64(index) < uint64(len(c.explanationRegisters)) && c.explanationRegisters[index] != nil {
		if err := c.explanationRegisters[index].Close(); err != nil {
			return err
		}
		c.explanationRegisters[index] = nil
	}
	if previous := c.registers[index]; previous != nil && previous != value {
		if err := previous.Close(); err != nil {
			return err
		}
	}
	c.registers[index] = value
	return nil
}

func (c *ExecutionContext) MakeEmpty(index uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	if uint64(index) < uint64(len(c.registers)) {
		return c.SetRegister(index, nil)
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
	if c == nil || c.closed || c.terminate != nil && c.terminate() {
		return true
	}
	// Source compares the limit with integer-truncated elapsed milliseconds.
	if c.timeLimit != 0 && time.Since(c.started)/time.Millisecond > c.timeLimit/time.Millisecond {
		return true
	}
	if c.memory != nil {
		above, err := c.memory.AboveWatermark()
		return err != nil || above
	}
	return false
}

// SetTimeLimit starts a fresh source-style per-execution timeout. A zero limit
// is invalid, matching execution_context::set_timelimit's assertion.
func (c *ExecutionContext) SetTimeLimit(milliseconds uint32) error {
	if err := c.open(); err != nil || milliseconds == 0 {
		return errors.Join(ErrInstruction, err)
	}
	c.timeLimit = time.Duration(milliseconds) * time.Millisecond
	c.started = time.Now()
	return nil
}

// ResetTimeLimit disables a timeout without changing the external cancellation
// callback.
func (c *ExecutionContext) ResetTimeLimit() error {
	if err := c.open(); err != nil {
		return err
	}
	c.timeLimit = 0
	c.started = time.Time{}
	return nil
}

// ReportBigRelations writes the source profile table, ordered by estimated
// bytes descending. Register annotations are indexed by the sorted row number,
// matching the retained C++ implementation.
func (c *ExecutionContext) ReportBigRelations(threshold uint32, out io.Writer) error {
	if err := c.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	type estimate struct {
		register uint32
		bytes    uint32
	}
	sizes := make([]estimate, len(c.registers))
	var total uint64
	for i, table := range c.registers {
		var bytes uint32
		if table != nil {
			_, value, err := finiteTableSizeEstimates(table)
			if err != nil {
				return err
			}
			bytes = value
		} else if i < len(c.explanationRegisters) && c.explanationRegisters[i] != nil {
			value, err := c.explanationRegisters[i].SizeEstimateBytes()
			if err != nil {
				return err
			}
			bytes = value
		}
		total += uint64(bytes)
		sizes[i] = estimate{register: uint32(i), bytes: bytes}
	}
	sort.Slice(sizes, func(i, j int) bool { return sizes[i].bytes > sizes[j].bytes })
	if _, err := fmt.Fprintf(out, "bytes %d\nbytes\trows\tannotation\n", total); err != nil {
		return err
	}
	for i, size := range sizes {
		if size.bytes < threshold {
			continue
		}
		var rows uint32
		if table := c.registers[size.register]; table != nil {
			value, _, err := finiteTableSizeEstimates(table)
			if err != nil {
				return err
			}
			rows = value
		} else if int(size.register) < len(c.explanationRegisters) && c.explanationRegisters[size.register] != nil {
			value, err := c.explanationRegisters[size.register].SizeEstimateRows()
			if err != nil {
				return err
			}
			rows = value
		}
		annotation := c.annotations[uint32(i)]
		if _, err := fmt.Fprintf(out, "%d\t%d\t%s\n", size.bytes, rows, annotation); err != nil {
			return err
		}
	}
	return nil
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
	if err := c.closeRegisters(); err != nil {
		return err
	}
	c.registers = c.registers[:0]
	c.explanationRegisters = c.explanationRegisters[:0]
	c.relationExplanationRegisters = c.relationExplanationRegisters[:0]
	clear(c.annotations)
	// execution_context::reset clears registers, annotations and the timeout,
	// but leaves its statistics cumulative for the lifetime of the context.
	_ = c.ResetTimeLimit()
	return nil
}

func (c *ExecutionContext) Close() error {
	if c == nil {
		return ErrInstruction
	}
	if c.closed {
		return nil
	}
	err := c.closeRegisters()
	if err != nil {
		return err
	}
	c.registers = nil
	c.explanationRegisters = nil
	c.relationExplanationRegisters = nil
	clear(c.annotations)
	c.annotations = nil
	c.memory = nil
	c.closed = true
	return err
}

func (c *ExecutionContext) closeRegisters() error {
	var result error
	for index := range c.relationExplanationRegisters {
		result = errors.Join(result, c.clearRelationExplanationRegister(uint32(index)))
	}
	for i, product := range c.explanationRegisters {
		if product != nil {
			if err := product.Close(); err != nil {
				result = errors.Join(result, err)
			} else {
				c.explanationRegisters[i] = nil
			}
		}
	}
	for i, table := range c.registers {
		if table != nil {
			result = errors.Join(result, table.Close())
			c.registers[i] = nil
		}
	}
	return result
}

func finiteTableSizeEstimates(table FiniteTable) (uint32, uint32, error) {
	switch value := table.(type) {
	case *HashTable:
		rows, err := value.SizeEstimateRows()
		if err != nil {
			return 0, 0, err
		}
		bytes, err := value.SizeEstimateBytes()
		return rows, bytes, err
	case *FiniteCheckedPair:
		rows, err := value.SizeEstimateRows()
		if err != nil {
			return 0, 0, err
		}
		bytes, err := value.SizeEstimateBytes()
		return rows, bytes, err
	case *FormulaCheckedFinitePair:
		rows, err := value.SizeEstimateRows()
		if err != nil {
			return 0, 0, err
		}
		bytes, err := value.SizeEstimateBytes()
		return rows, bytes, err
	case *SparseTable:
		rows, err := value.SizeEstimateRows()
		if err != nil {
			return 0, 0, err
		}
		bytes, err := value.SizeEstimateBytes()
		return rows, bytes, err
	default:
		return 0, 0, ErrInstruction
	}
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
	if err := context.open(); err != nil {
		return false, err
	}
	if hasExplanationInstructionRegister(context, i.source) {
		return i.performExplanationRegister(context)
	}
	source, err := context.FiniteRegister(i.source)
	if err != nil {
		return false, err
	}
	switch i.kind {
	case "dealloc":
		return true, context.MakeEmpty(i.source)
	case "clone":
		var value FiniteTable
		if source != nil {
			value, err = source.CloneFinite()
			if err != nil {
				return false, err
			}
		}
		return true, context.SetFiniteRegister(i.target, value)
	case "move":
		if source == nil {
			return true, context.SetFiniteRegister(i.target, nil)
		}
		value, x := context.ReleaseFiniteRegister(i.source)
		if x != nil {
			return false, x
		}
		return true, context.SetFiniteRegister(i.target, value)
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
