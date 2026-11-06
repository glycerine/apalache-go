package fp

// Source: rel_context::transform_rules registers bit blasting at 22000 and
// another interpreted-tail simplifier at 21000. Predicate expansion comes from
// dl_mk_bit_blast.cpp. The C++ transform does not migrate stored facts, and its
// relational answer uses transformed Boolean coordinates. Go preserves its
// existing finite fact, original-domain Answer and Model contracts by importing
// stored rows and materializing completed Boolean results back into their
// original tables before the query scope releases transformed predicates.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type datalogBitBlastPredicate struct {
	original, derived ast.ASTHandle
	widths            []uint32
}

// This query-local descriptor borrows plugin and table owners. The ordered
// transformer and outer Datalog query scope own every generated resource.
type datalogBitBlastQuery struct {
	engine  *PropositionalEngine
	planner *transforms.BitBlastPlugin
	columns []datalogBitBlastPredicate
}

func (q *datalogBitBlastQuery) register(transformer *base.RuleTransformer, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) error {
	if q == nil || q.engine == nil {
		return nil
	}
	e := q.engine
	enabled := false
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return err
		}
		enabled, err = view.Bool("xform.bit_blast", false)
		err = errors.Join(err, view.Close())
		if err != nil {
			return err
		}
	}
	if !enabled {
		return nil
	}
	normalization, err := e.ruleNormalizationParams()
	if err != nil {
		return err
	}
	planner, err := transforms.NewBitBlastPlugin(e.manager, e.rules, context, globals, 22000)
	if err != nil {
		return err
	}
	planner.SetNormalizationParams(normalization)
	if err = transformer.Register(planner); err != nil {
		return errors.Join(err, planner.Close())
	}
	q.planner = planner
	simplifier, err := transforms.NewInterpTailSimplifierPlugin(e.manager, e.rules, context, globals, 21000)
	if err != nil {
		return err
	}
	simplifier.SetNormalizationParams(normalization)
	if err = transformer.Register(simplifier); err != nil {
		return errors.Join(err, simplifier.Close())
	}
	return nil
}

func (q *datalogBitBlastQuery) install() error {
	if q == nil || q.planner == nil {
		return nil
	}
	e := q.engine
	mapping := q.planner.FreshPredicates()
	for _, original := range e.predicateOrder {
		derived, found := mapping[original]
		if !found {
			continue
		}
		source := e.predicates[original]
		if source == nil || source.table == nil {
			return fmt.Errorf("%w: bit-blast result requires an original finite table", ErrPropositionalHorn)
		}
		columns := datalogBitBlastPredicate{original: original, derived: derived, widths: make([]uint32, len(source.sorts))}
		var bits uint64
		for i, sort := range source.sorts {
			width, err := e.manager.BitVectorSize(sort)
			if err != nil {
				return err
			}
			if width == 0 || width > 63 {
				return fmt.Errorf("%w: bit-blast finite column width %d", ErrPropositionalHorn, width)
			}
			columns.widths[i] = width
			bits += uint64(width)
		}
		arity, err := e.manager.GetArity(derived)
		if err != nil {
			return err
		}
		if bits != uint64(arity) {
			return ast.ErrASTDomain
		}
		target, err := e.predicateState(derived)
		if err != nil {
			return err
		}
		if target.table == nil {
			return ErrPropositionalHorn
		}
		if e.predicates[derived] == nil {
			e.predicateOrder = append(e.predicateOrder, derived)
		}
		e.predicates[derived] = target
		e.auxiliary[derived] = true
		if err = e.inheritDatalogFiniteRepresentation(derived, original); err != nil {
			return err
		}
		facts, err := source.table.Facts()
		if err != nil {
			return err
		}
		for _, fact := range facts {
			encoded, err := encodeDatalogBitBlastFact(columns.widths, fact)
			if err != nil {
				return err
			}
			if _, err = target.table.Add(encoded); err != nil {
				return err
			}
		}
		q.columns = append(q.columns, columns)
	}
	if len(q.columns) != len(mapping) {
		return fmt.Errorf("%w: bit-blasted predicate has no finite owner", ErrPropositionalHorn)
	}
	return nil
}

// publish prepares every inverse table before replacing original owners, so
// conversion failure cannot publish a partially decoded relation. Bit-blast
// conversion precedes the earlier inliner/COI model ledger: those converters
// read original-domain interpretations built from these completed tables.
func (q *datalogBitBlastQuery) publish() (resultErr error) {
	if q == nil || len(q.columns) == 0 {
		return nil
	}
	e := q.engine
	tables := make([]rel.FiniteTable, len(q.columns))
	defer func() {
		for _, table := range tables {
			if table != nil {
				resultErr = errors.Join(resultErr, table.Close())
			}
		}
	}()
	for index, columns := range q.columns {
		original, derived := e.predicates[columns.original], e.predicates[columns.derived]
		if original == nil || original.table == nil || derived == nil || derived.table == nil {
			return ErrPropositionalEngine
		}
		table, err := e.cloneFiniteForContext(original.table)
		if err != nil {
			return err
		}
		tables[index] = table
		if err = table.Reset(); err != nil {
			return err
		}
		facts, err := derived.table.Facts()
		if err != nil {
			return err
		}
		for _, fact := range facts {
			decoded, err := decodeDatalogBitBlastFact(columns.widths, fact)
			if err != nil {
				return err
			}
			if _, err = table.Add(decoded); err != nil {
				return err
			}
		}
	}
	for index, columns := range q.columns {
		table := tables[index]
		tables[index] = nil // installDatalogTable consumes on success and failure.
		if err := e.installDatalogTable(columns.original, table); err != nil {
			return err
		}
	}
	return nil
}
