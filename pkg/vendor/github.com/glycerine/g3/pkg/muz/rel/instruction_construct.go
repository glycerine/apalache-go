package rel

// Source: src/muz/rel/dl_instruction.cpp unary-singleton, total-relation,
// and signature-assertion instructions.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type constructInstruction struct {
	accounting *base.AccountedObject
	kind       string
	signature  Signature
	value      Element
	target     uint32
	closed     bool
}

func NewUnarySingletonInstruction(domain uint64, value Element, target uint32) (Instruction, error) {
	signature, err := NewSignature([]uint64{domain})
	if err != nil || uint64(value) >= domain {
		return nil, errors.Join(ErrInstruction, err)
	}
	return &constructInstruction{
		accounting: base.NewAccountedObject(), kind: "mk_unary_singleton",
		signature: signature, value: value, target: target,
	}, nil
}

func NewTotalInstruction(signature Signature, target uint32) Instruction {
	return &constructInstruction{
		accounting: base.NewAccountedObject(), kind: "mk_total",
		signature: Signature{domains: signature.Domains()}, target: target,
	}
}

func NewAssertSignatureInstruction(signature Signature, target uint32) Instruction {
	return &constructInstruction{
		accounting: base.NewAccountedObject(), kind: "assert_signature",
		signature: Signature{domains: signature.Domains()}, target: target,
	}
}

func (i *constructInstruction) open() error {
	if i == nil || i.closed || i.accounting == nil {
		return ErrInstruction
	}
	return nil
}

func fillTotalTable(table *HashTable, signature Signature) error {
	fact := make(Fact, signature.Len())
	for {
		if _, err := table.Add(fact); err != nil {
			return err
		}
		column := len(fact) - 1
		for column >= 0 {
			fact[column]++
			if uint64(fact[column]) < signature.domains[column] {
				break
			}
			fact[column] = 0
			column--
		}
		if column < 0 {
			return nil
		}
	}
}

func (i *constructInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	switch i.kind {
	case "mk_unary_singleton":
		context.stats.UnarySingleton++
		table := NewHashTable(i.signature)
		if _, err := table.Add(Fact{i.value}); err != nil {
			return false, err
		}
		return true, context.SetRegister(i.target, table)
	case "mk_total":
		context.stats.Total++
		table := NewHashTable(i.signature)
		if err := fillTotalTable(table, i.signature); err != nil {
			return false, err
		}
		return true, context.SetRegister(i.target, table)
	case "assert_signature":
		table, err := context.Register(i.target)
		if err != nil || table == nil {
			return err == nil, err
		}
		actual, err := table.Signature()
		if err != nil {
			return false, err
		}
		if !actual.equal(i.signature) {
			return false, fmt.Errorf("%w: register signature assertion failed", ErrInstruction)
		}
		return true, nil
	default:
		return false, ErrInstruction
	}
}

func (i *constructInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	if _, found, err := context.RegisterAnnotation(i.target); err != nil || found {
		return err
	}
	return context.SetRegisterAnnotation(i.target, i.kind)
}

func (i *constructInstruction) DisplayHead(out io.Writer) error {
	if err := i.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	switch i.kind {
	case "mk_unary_singleton":
		_, err := fmt.Fprintf(out, "mk_unary_singleton into %d domain:%d val:%d", i.target, i.signature.domains[0], i.value)
		return err
	case "mk_total":
		_, err := fmt.Fprintf(out, "mk_total into %d signature:%v", i.target, i.signature.domains)
		return err
	case "assert_signature":
		_, err := fmt.Fprintf(out, "assert_signature of %d signature:%v", i.target, i.signature.domains)
		return err
	default:
		return ErrInstruction
	}
}

func (i *constructInstruction) Accounting() *base.AccountedObject { return i.accounting }

func (i *constructInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	return i.accounting.ProcessCosts()
}

func (i *constructInstruction) CollectStatistics(out *statistics.Statistics) error {
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

func (i *constructInstruction) Close() error {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	i.closed = true
	return i.accounting.Close()
}
