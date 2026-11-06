package smtcore

// Source: src/cmd_context/pdecl.cpp:432-461,495-503,555-563,666-677.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// ParamDatatypeSortKind is the reached ptype tag used while resolving
// mutually recursive accessor types. Parametric sort instantiation is owned
// by a later pdecl phase; this pass only needs to preserve its tag.
type ParamDatatypeSortKind uint8

const (
	ParamSort ParamDatatypeSortKind = iota
	ParamRecursiveSort
	ParamMissingSort
)

type ParamDatatypeAccessor struct {
	Name           usymbol.Symbol
	SortKind       ParamDatatypeSortKind
	RecursiveIndex int32
	MissingSort    usymbol.Symbol
}

type ParamDatatypeConstructor struct {
	Name      usymbol.Symbol
	Accessors []ParamDatatypeAccessor
}

type ParamDatatypeDecl struct {
	Name         usymbol.Symbol
	Constructors []ParamDatatypeConstructor
}

// HasMissingRef follows the accessor walk used before datatype instantiation.
func (d *ParamDatatypeDecl) HasMissingRef() (usymbol.Symbol, bool) {
	for ci := range d.Constructors {
		for ai := range d.Constructors[ci].Accessors {
			a := &d.Constructors[ci].Accessors[ai]
			if a.SortKind == ParamMissingSort {
				return a.MissingSort, true
			}
		}
	}
	return usymbol.Symbol{}, false
}

// HasDuplicateAccessors checks one datatype across all its constructors.
// Other datatypes in a recursive group have independent accessor namespaces.
func (d *ParamDatatypeDecl) HasDuplicateAccessors() (usymbol.Symbol, bool) {
	names := make(map[usymbol.Symbol]struct{})
	for ci := range d.Constructors {
		for ai := range d.Constructors[ci].Accessors {
			name := d.Constructors[ci].Accessors[ai].Name
			if _, found := names[name]; found {
				return name, true
			}
			names[name] = struct{}{}
		}
	}
	return usymbol.Symbol{}, false
}

// ParamDatatypeGroup is the concrete pdatatypes_decl name-resolution owner.
// Declaration and accessor slices retain source order; equal datatype names
// overwrite the previous dictionary index, as map::insert does.
type ParamDatatypeGroup struct {
	Datatypes []ParamDatatypeDecl
	trace     *diagnostic.TraceTags
}

// FixMissingRefs changes each reached missing accessor to a recursive index.
// It stops at the first unknown name without undoing earlier replacements.
func (g *ParamDatatypeGroup) FixMissingRefs() (missing usymbol.Symbol, ok bool) {
	tags := g.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("fix_missing_refs", "fix_missing_refs", "src/cmd_context/pdecl.cpp", 667,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, "pdatatypes_decl::fix_missing_refs\n")
			return err
		})
	indices := make(map[usymbol.Symbol]int32, len(g.Datatypes))
	for i := range g.Datatypes {
		indices[g.Datatypes[i].Name] = int32(i)
	}
	for di := range g.Datatypes {
		for ci := range g.Datatypes[di].Constructors {
			for ai := range g.Datatypes[di].Constructors[ci].Accessors {
				a := &g.Datatypes[di].Constructors[ci].Accessors[ai]
				if a.SortKind != ParamMissingSort {
					continue
				}
				idx, found := indices[a.MissingSort]
				if !found {
					return a.MissingSort, false
				}
				a.SortKind = ParamRecursiveSort
				a.RecursiveIndex = idx
				a.MissingSort = usymbol.Symbol{}
			}
		}
	}
	return usymbol.Symbol{}, true
}
