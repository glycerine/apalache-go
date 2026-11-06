package rel

// Source: src/muz/rel/dl_instruction.cpp unary-singleton, total-relation,
// and signature-assertion instructions.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type constructInstruction struct {
	accounting    *base.AccountedObject
	kind          string
	signature     Signature
	value         Element
	astValue      ast.ASTHandle
	target        uint32
	closed        bool
	relations     *RelationManager
	explanations  int
	finiteSort    ast.ASTRef
	astValueRoot  ast.ASTRef
	finiteManager *ast.ASTManager
	pendingFinite FiniteTable
	closing       bool
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

// NewExplanationTotalInstruction is mk_total for a rule-sort-only signature:
// its full explanation relation has one row of undefined slots. A zero-column
// signature is the full nullary relation used to seed ground-rule assembly.
func NewExplanationTotalInstruction(relations *RelationManager, columns int, target uint32) (Instruction, error) {
	if relations == nil || relations.open() != nil || columns < 0 {
		return nil, ErrInstruction
	}
	return &constructInstruction{accounting: base.NewAccountedObject(), kind: "mk_explanation_total", relations: relations, explanations: columns, target: target}, nil
}

// NewExplanationFiniteTotalInstruction builds the finite unary full relation
// used when a compiler appends an unbound head variable to an explained body.
func NewExplanationFiniteTotalInstruction(relations *RelationManager, sort ast.ASTHandle, target uint32) (Instruction, error) {
	return newExplanationFiniteColumnInstruction(relations, sort, 0, target, false)
}

// NewExplanationFiniteSingletonInstruction builds the finite unary singleton
// used to append a constant head column to an explained body.
func NewExplanationFiniteSingletonInstruction(relations *RelationManager, sort ast.ASTHandle, value Element, target uint32) (Instruction, error) {
	return newExplanationFiniteColumnInstruction(relations, sort, value, target, true)
}

// NewExplanationNumericColumnInstruction creates a total numeric column or
// singleton. A nil value means the full relation; otherwise the selected
// numeric plugin applies its native equality operation.
func NewExplanationNumericColumnInstruction(relations *RelationManager, sort ast.ASTHandle, value ast.ASTHandle, target uint32) (Instruction, error) {
	if relations == nil || relations.open() != nil || relations.explanationNumericKind == "" {
		return nil, ErrInstruction
	}
	accepted, err := relationExplanationNumericCanHandle(relations, relations.explanationNumericKind, []ast.ASTHandle{sort})
	if err != nil {
		return nil, err
	}
	if !accepted {
		if value.IsNull() {
			return NewExplanationFiniteTotalInstruction(relations, sort, target)
		}
		encoded, encodeErr := relations.EncodeValue(sort, value)
		if encodeErr != nil {
			return nil, encodeErr
		}
		return NewExplanationFiniteSingletonInstruction(relations, sort, encoded, target)
	}
	root, err := relations.manager.NewRef(sort)
	if err != nil {
		return nil, err
	}
	var valueRoot ast.ASTRef
	if !value.IsNull() {
		valueRoot, err = relations.manager.NewRef(value)
		if err != nil {
			return nil, errors.Join(err, relations.manager.DelRef(&root))
		}
	}
	return &constructInstruction{accounting: base.NewAccountedObject(), kind: "mk_explanation_numeric_column", relations: relations, finiteManager: relations.manager, finiteSort: root, astValueRoot: valueRoot, astValue: value, target: target}, nil
}

func newExplanationFiniteColumnInstruction(relations *RelationManager, sort ast.ASTHandle, value Element, target uint32, singleton bool) (Instruction, error) {
	if relations == nil || relations.open() != nil {
		return nil, ErrInstruction
	}
	domain, err := relations.sortDomain(sort)
	if err != nil {
		return nil, err
	}
	if singleton && uint64(value) >= domain {
		return nil, ErrInstruction
	}
	signature, err := NewSignature([]uint64{domain})
	if err != nil {
		return nil, err
	}
	root, err := relations.manager.NewRef(sort)
	if err != nil {
		return nil, err
	}
	kind := "mk_explanation_finite_total"
	if singleton {
		kind = "mk_explanation_finite_singleton"
	}
	return &constructInstruction{accounting: base.NewAccountedObject(), kind: kind, relations: relations, finiteManager: relations.manager, finiteSort: root, signature: signature, value: value, target: target}, nil
}

func NewAssertSignatureInstruction(signature Signature, target uint32) Instruction {
	return &constructInstruction{
		accounting: base.NewAccountedObject(), kind: "assert_signature",
		signature: Signature{domains: signature.Domains()}, target: target,
	}
}

func (i *constructInstruction) open() error {
	if i == nil || i.closed || i.closing || i.accounting == nil {
		return ErrInstruction
	}
	return nil
}

func (i *constructInstruction) clearPendingFinite() error {
	if i.pendingFinite == nil {
		return nil
	}
	if err := i.pendingFinite.Close(); err != nil {
		return err
	}
	i.pendingFinite = nil
	return nil
}

func (i *constructInstruction) disposeUninstalledFinite(table FiniteTable, cause error) error {
	if table == nil {
		return cause
	}
	if err := table.Close(); err != nil {
		i.pendingFinite = table
		return errors.Join(cause, err)
	}
	return cause
}

func fillTotalTable(table FiniteTable, signature Signature) error {
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

func constructFiniteTable(context *ExecutionContext, signature Signature) (FiniteTable, error) {
	if context != nil && context.relations != nil {
		return context.relations.NewDefaultFiniteTable(signature)
	}
	return NewHashTable(signature), nil
}

func (i *constructInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if err := i.clearPendingFinite(); err != nil {
		return false, err
	}
	switch i.kind {
	case "mk_unary_singleton":
		context.stats.UnarySingleton++
		table, err := constructFiniteTable(context, i.signature)
		if err != nil {
			return false, err
		}
		if _, err := table.Add(Fact{i.value}); err != nil {
			return false, errors.Join(err, table.Close())
		}
		if err := context.SetFiniteRegister(i.target, table); err != nil {
			return false, i.disposeUninstalledFinite(table, err)
		}
		return true, nil
	case "mk_total":
		context.stats.Total++
		table, err := constructFiniteTable(context, i.signature)
		if err != nil {
			return false, err
		}
		if err := fillTotalTable(table, i.signature); err != nil {
			return false, errors.Join(err, table.Close())
		}
		if err := context.SetFiniteRegister(i.target, table); err != nil {
			return false, i.disposeUninstalledFinite(table, err)
		}
		return true, nil
	case "mk_explanation_finite_total", "mk_explanation_finite_singleton":
		if err := context.open(); err != nil {
			return false, err
		}
		if context.relations != nil && context.relations != i.relations {
			return false, ErrInstruction
		}
		sort, err := i.finiteManager.RefGet(i.finiteSort)
		if err != nil {
			return false, err
		}
		table := NewHashTable(i.signature)
		if i.kind == "mk_explanation_finite_total" {
			context.stats.Total++
			err = fillTotalTable(table, i.signature)
		} else {
			context.stats.UnarySingleton++
			_, err = table.Add(Fact{i.value})
		}
		if err != nil {
			return false, errors.Join(err, table.Close())
		}
		product, err := finiteExplanationProductFromHashTable(i.relations, []ast.ASTHandle{sort}, table)
		err = errors.Join(err, table.Close())
		if err != nil {
			if product != nil {
				err = errors.Join(err, product.Close())
			}
			return false, err
		}
		if err = context.SetExplanationRegister(i.target, product); err != nil {
			return false, errors.Join(err, product.Close())
		}
		return true, nil
	case "mk_explanation_numeric_column":
		if err := context.open(); err != nil {
			return false, err
		}
		if context.relations != nil && context.relations != i.relations {
			return false, ErrInstruction
		}
		sort, err := i.finiteManager.RefGet(i.finiteSort)
		if err != nil {
			return false, err
		}
		product, err := newNumericRelationExplanationProductWithProofColumns(i.relations, []ast.ASTHandle{sort}, i.relations.explanationNumericKind, i.relations.explanationNumericCheck, 0)
		if err != nil {
			return false, err
		}
		var seed RelationExplanationNumericSeed
		seed.Kind = product.numeric.kind
		switch seed.Kind {
		case "bound_relation":
			seed.Bound = NewBoundRelation(1, false)
		case "interval_relation":
			seed.Interval = NewIntervalFull(1)
		case "karr_relation":
			seed.Karr = NewKarrFull(1, nil, product.numeric.karr.formulaChecker)
		default:
			return false, errors.Join(ErrInstruction, product.Close())
		}
		if err = product.UnionNumericSeed(seed); err == nil && !i.astValue.IsNull() {
			err = product.FilterEqualAST(i.astValue, 0)
		}
		if err != nil {
			return false, errors.Join(err, product.Close())
		}
		if err = product.explanation.AddFact(nil); err != nil {
			return false, errors.Join(err, product.Close())
		}
		context.stats.Total++
		return setRelationExplanationResult(context, i.target, product)
	case "mk_explanation_total":
		if err := context.open(); err != nil {
			return false, err
		}
		if context.relations != nil && context.relations != i.relations {
			return false, ErrInstruction
		}
		context.stats.Total++
		product, err := newFiniteExplanationProduct(i.relations, nil, i.explanations)
		if err != nil {
			return false, err
		}
		if err = product.addEncodedRow(Fact{0}, make([]ast.ASTHandle, i.explanations)); err != nil {
			return false, errors.Join(err, product.Close())
		}
		if err = context.SetExplanationRegister(i.target, product); err != nil {
			return false, errors.Join(err, product.Close())
		}
		return true, nil
	case "assert_signature":
		table, err := context.FiniteRegister(i.target)
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
	case "mk_explanation_finite_total":
		_, err := fmt.Fprintf(out, "mk_total into %d finite-signature:%v", i.target, i.signature.domains)
		return err
	case "mk_explanation_finite_singleton":
		_, err := fmt.Fprintf(out, "mk_unary_singleton into %d domain:%d val:%d", i.target, i.signature.domains[0], i.value)
		return err
	case "mk_explanation_numeric_column":
		_, err := fmt.Fprintf(out, "mk_total into %d numeric-column:%s", i.target, i.relations.explanationNumericKind)
		return err
	case "mk_explanation_total":
		_, err := fmt.Fprintf(out, "mk_total into %d explanation-columns:%d", i.target, i.explanations)
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
	i.closing = true
	if err := i.clearPendingFinite(); err != nil {
		return err
	}
	if i.finiteSort != (ast.ASTRef{}) {
		if err := i.finiteManager.DelRef(&i.finiteSort); err != nil {
			return err
		}
	}
	if i.astValueRoot != (ast.ASTRef{}) {
		if err := i.finiteManager.DelRef(&i.astValueRoot); err != nil {
			return err
		}
	}
	if err := i.accounting.Close(); err != nil {
		return err
	}
	i.closed = true
	return nil
}
