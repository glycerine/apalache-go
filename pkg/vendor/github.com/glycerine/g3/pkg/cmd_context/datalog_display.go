package cmd_context

// Source: context::register_finite_sort/get_constant_number and
// rel_context::display_output_facts. The command session owns source names
// after the file parser has released its temporary AST references.

import (
	"fmt"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Domain numbering belongs to the context in the source. A new parser must
// continue an already registered domain before it publishes any new rules.
func (p *datalogParser) seedDatalogDomain(sort ast.ASTHandle, numeric bool) error {
	owner := p.context.datalogDisplay
	if owner == nil {
		return nil
	}
	known, err := owner.HasSortDomain(sort)
	if err != nil || !known {
		return err
	}
	snapshot, err := owner.SortDomainSnapshot(sort)
	if err != nil {
		return err
	}
	want := base.DatalogSortSymbol
	if numeric {
		want = base.DatalogSortUint64
	}
	if snapshot.Kind != want {
		return fmt.Errorf("datalog: finite domain already registered with a different constant kind")
	}
	if numeric {
		for ordinal, value := range snapshot.Uint64s {
			p.numericConstants[sort][value] = uint64(ordinal)
		}
	} else {
		for ordinal, value := range snapshot.Symbols {
			name, err := p.manager.Symbols.String(value)
			if err != nil {
				return err
			}
			p.constants[sort][name] = uint64(ordinal)
		}
	}
	return nil
}

func (c *Context) rememberDatalogDisplay(p *datalogParser) error {
	if c.datalogDisplay == nil {
		owner, err := base.NewDatalogContext(p.manager)
		c.datalogDisplay = owner
		if err != nil {
			return err
		}
	}
	owner := c.datalogDisplay
	names := make([]string, 0, len(p.sorts))
	for name := range p.sorts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := p.sorts[name]
		strings, symbolic := p.constants[s]
		numbers, numeric := p.numericConstants[s]
		if !symbolic && !numeric {
			continue // Arithmetic sorts have no finite-domain name table.
		}
		kind := base.DatalogSortSymbol
		if numeric {
			kind = base.DatalogSortUint64
		}
		known, err := owner.HasSortDomain(s)
		if err != nil {
			return err
		}
		if !known {
			if err = owner.RegisterFiniteSort(s, kind); err != nil {
				return err
			}
		} else if existing, err := owner.SortDomainKind(s); err != nil || existing != kind {
			return fmt.Errorf("datalog: conflicting display domain kind for %s", name)
		}
		// Assigned ordinals, not Go map iteration, define the reverse table.
		if numeric {
			ordered := make([]uint64, len(numbers))
			for value, ordinal := range numbers {
				ordered[ordinal] = value
			}
			for ordinal, value := range ordered {
				index, err := owner.Uint64Number(s, value)
				if err != nil {
					return err
				}
				if index != uint64(ordinal) {
					return fmt.Errorf("datalog: conflicting display numbering for %s", name)
				}
			}
		} else {
			ordered := make([]string, len(strings))
			for value, ordinal := range strings {
				ordered[ordinal] = value
			}
			for ordinal, value := range ordered {
				symbol, err := p.manager.Symbols.Intern(value)
				if err != nil {
					return err
				}
				index, err := owner.SymbolNumber(s, symbol)
				if err != nil {
					return err
				}
				if index != uint64(ordinal) {
					return fmt.Errorf("datalog: conflicting display numbering for %s", name)
				}
			}
		}
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	declarations, err := engine.Predicates()
	if err != nil {
		return err
	}
	for _, declaration := range declarations {
		known, err := owner.IsPredicate(declaration)
		if err != nil {
			return err
		}
		if known {
			continue
		}
		if err = owner.RegisterPredicate(declaration, false); err != nil {
			return err
		}
		arity, err := p.manager.GetArity(declaration)
		if err != nil {
			return err
		}
		arguments := make([]usymbol.Symbol, arity)
		for i := range arguments {
			arguments[i], err = engine.ArgumentName(declaration, uint32(i))
			if err != nil {
				return err
			}
		}
		if err = owner.SetArgumentNames(declaration, arguments); err != nil {
			return err
		}
	}
	return nil
}

// FixedpointDatalogPrintTuples reads the current source output option before
// the shell starts its query-local saturation attempt.
func (c *Context) FixedpointDatalogPrintTuples() (bool, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return false, err
	}
	return engine.DatalogPrintTuples()
}

// WriteFixedpointDatalogTuples prints the original pragma outputs without
// issuing another query or changing the copied numeric result snapshots.
func (c *Context) WriteFixedpointDatalogTuples(out io.Writer) error {
	if err := c.valid(); err != nil {
		return err
	}
	if c.datalogDisplay == nil {
		return fmt.Errorf("datalog: no parsed display domains")
	}
	engine, err := c.fixedpointContext()
	if err != nil {
		return err
	}
	return engine.WriteDatalogOutputTables(c.datalogDisplay, out)
}
