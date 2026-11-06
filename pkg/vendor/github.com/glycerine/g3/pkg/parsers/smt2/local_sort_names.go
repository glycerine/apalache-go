package smt2

// Source: src/parsers/smt2/smt2parser.cpp:55-58,590-604,857-879,2098-2146,2992-3006.

import usymbol "github.com/glycerine/g3/pkg/util/symbol"

// smt2SortNames owns the parser's three reached dictionary specializations.
// No source path iterates their hash tables; datatypeNames separately preserves
// the declaration order used by parse_declare_datatypes.
type smt2SortNames struct {
	sortParamIndex map[usymbol.Symbol]int32
	datatypeIndex  map[usymbol.Symbol]int32
	datatypeArity  map[usymbol.Symbol]uint32
	datatypeNames  []usymbol.Symbol
}

func (n *smt2SortNames) beginSortParams() {
	clear(n.sortParamIndex)
}

func (n *smt2SortNames) addSortParam(name usymbol.Symbol, position int32) {
	if n.sortParamIndex == nil {
		n.sortParamIndex = make(map[usymbol.Symbol]int32)
	}
	n.sortParamIndex[name] = position
}

func (n *smt2SortNames) sortParam(name usymbol.Symbol) (total uint32, position int32, found bool) {
	position, found = n.sortParamIndex[name]
	return uint32(len(n.sortParamIndex)), position, found
}

// beginDatatypeDecls is the source parse_sort_decl_or_params entry. The
// following declaration body may reset datatypeIndex separately.
func (n *smt2SortNames) beginDatatypeDecls() {
	clear(n.sortParamIndex)
	clear(n.datatypeIndex)
	clear(n.datatypeArity)
	n.datatypeNames = n.datatypeNames[:0]
}

func (n *smt2SortNames) beginDatatypeBodies() { clear(n.datatypeIndex) }

func (n *smt2SortNames) addDatatype(name usymbol.Symbol, position int32, arity uint32) {
	if n.datatypeIndex == nil {
		n.datatypeIndex = make(map[usymbol.Symbol]int32)
	}
	if n.datatypeArity == nil {
		n.datatypeArity = make(map[usymbol.Symbol]uint32)
	}
	n.datatypeIndex[name] = position
	n.datatypeArity[name] = arity
	n.datatypeNames = append(n.datatypeNames, name)
}

func (n *smt2SortNames) lookupDatatypeIndex(name usymbol.Symbol) (int32, bool) {
	v, found := n.datatypeIndex[name]
	return v, found
}

func (n *smt2SortNames) lookupDatatypeArity(name usymbol.Symbol) (uint32, bool) {
	v, found := n.datatypeArity[name]
	return v, found
}

// Parser reset clears only these two dictionaries. The next datatype-decl
// entry clears the arity map and ordered names before reading new syntax.
func (n *smt2SortNames) reset() {
	clear(n.sortParamIndex)
	clear(n.datatypeIndex)
}
