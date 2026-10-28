package apalache

import (
	"fmt"
	"strconv"
	"strings"
)

// CellType is the compact type system used by symbolic arena cells.
type CellType interface {
	Signature() string
	TLAType() Type
	String() string
	cellTypeNode()
}

// CompareCellTypes implements the signature ordering used by Scala CellT
// when cell types are keys in sorted collections.
func CompareCellTypes(left, right CellType) int {
	return strings.Compare(left.Signature(), right.Signature())
}

type UnknownCellType struct{}

func (UnknownCellType) cellTypeNode()     {}
func (UnknownCellType) Signature() string { return "u" }
func (UnknownCellType) String() string    { return "Unknown" }
func (UnknownCellType) TLAType() Type {
	typ, _ := NewConstType("UNKNOWN")
	return typ
}

type BasicCellType struct{ Type Type }

func (BasicCellType) cellTypeNode() {}
func (cell BasicCellType) Signature() string {
	switch typed := cell.Type.(type) {
	case ConstType:
		return "UT_" + typed.Name()
	case primitiveType:
		switch typed {
		case StrT1:
			return "UT_Str"
		case BoolT1:
			return "b"
		case IntT1:
			return "i"
		}
	case SetType:
		return "S" + BasicCellType{Type: typed.Element()}.Signature()
	case SeqType:
		return "Q_" + BasicCellType{Type: typed.Element()}.Signature()
	case FunType:
		return "f" + BasicCellType{Type: typed.Argument()}.Signature() + "_" + BasicCellType{Type: typed.Result()}.Signature()
	case TupleType:
		result := "T_"
		for index, element := range typed.Elements() {
			if index > 0 {
				result += "_"
			}
			result += BasicCellType{Type: element}.Signature()
		}
		return result
	case RecordType:
		return "R"
	}
	return "U"
}
func (cell BasicCellType) TLAType() Type { return cell.Type }
func (cell BasicCellType) String() string {
	if cell.Type == nil {
		return "<nil>"
	}
	return cell.Type.String()
}

type InfiniteSetCellType struct{ Element CellType }

func (InfiniteSetCellType) cellTypeNode()          {}
func (cell InfiniteSetCellType) Signature() string { return "Z" + cell.Element.Signature() }
func (cell InfiniteSetCellType) String() string    { return "InfSet[" + cell.Element.String() + "]" }
func (cell InfiniteSetCellType) TLAType() Type     { return mustSet(cell.Element.TLAType()) }

// IntegerRangeCellType represents a finite integer interval whose bounds are
// symbolic arena cells stored as the range cell's domain and codomain edges.
type IntegerRangeCellType struct{}

func (IntegerRangeCellType) cellTypeNode()     {}
func (IntegerRangeCellType) Signature() string { return "ZRange" }
func (IntegerRangeCellType) String() string    { return "IntRange" }
func (IntegerRangeCellType) TLAType() Type     { return mustSet(IntT1) }

type PowerSetCellType struct{ Domain Type }

func (PowerSetCellType) cellTypeNode() {}
func (cell PowerSetCellType) Signature() string {
	return "P" + BasicCellType{Type: cell.Domain}.Signature()
}
func (cell PowerSetCellType) String() string { return "PowSet[" + cell.Domain.String() + "]" }
func (cell PowerSetCellType) TLAType() Type  { return mustSet(cell.Domain) }

func NewPowerSetCellType(domain Type) (PowerSetCellType, error) {
	if _, ok := domain.(SetType); !ok {
		return PowerSetCellType{}, fmt.Errorf("powerset arena type requires a set domain, found %s", domain)
	}
	return PowerSetCellType{Domain: domain}, nil
}

type FiniteFunctionSetCellType struct{ Domain, Codomain CellType }

func (FiniteFunctionSetCellType) cellTypeNode() {}
func (cell FiniteFunctionSetCellType) Signature() string {
	return "F%" + cell.Domain.Signature() + "_%" + cell.Codomain.Signature()
}
func (cell FiniteFunctionSetCellType) String() string {
	return "FinFunSet[" + cell.Domain.String() + ", " + cell.Codomain.String() + "]"
}
func (cell FiniteFunctionSetCellType) TLAType() Type {
	return mustSet(mustFun(cell.ArgumentType(), cell.ResultType()))
}
func (cell FiniteFunctionSetCellType) ArgumentType() Type {
	return collectionElementType(cell.Domain)
}
func (cell FiniteFunctionSetCellType) ResultType() Type {
	return collectionElementType(cell.Codomain)
}

func NewFiniteFunctionSetCellType(domain, codomain CellType) (FiniteFunctionSetCellType, error) {
	if !isFiniteCollectionCellType(domain) {
		return FiniteFunctionSetCellType{}, fmt.Errorf("function-set domain must be a finite set or powerset, found %s", domain)
	}
	if !isFiniteCollectionCellType(codomain) {
		switch codomain.(type) {
		case FiniteFunctionSetCellType, InfiniteSetCellType:
		default:
			return FiniteFunctionSetCellType{}, fmt.Errorf("function-set codomain has unsupported type %s", codomain)
		}
	}
	return FiniteFunctionSetCellType{Domain: domain, Codomain: codomain}, nil
}

func isFiniteCollectionCellType(cell CellType) bool {
	switch typed := cell.(type) {
	case BasicCellType:
		_, ok := typed.Type.(SetType)
		return ok
	case PowerSetCellType:
		return true
	default:
		return false
	}
}

func collectionElementType(cell CellType) Type {
	switch typed := cell.(type) {
	case BasicCellType:
		if set, ok := typed.Type.(SetType); ok {
			return set.Element()
		}
	case PowerSetCellType:
		return typed.Domain
	case InfiniteSetCellType:
		return typed.Element.TLAType()
	case FiniteFunctionSetCellType:
		return mustFun(typed.ArgumentType(), typed.ResultType())
	}
	panic(fmt.Sprintf("unexpected collection cell type %s", cell))
}

func CellTypeFromType(typ Type) (CellType, error) {
	if typ == nil {
		return nil, fmt.Errorf("cannot make an arena cell type from nil")
	}
	switch typed := typ.(type) {
	case primitiveType:
		if typed == RealT1 {
			return nil, fmt.Errorf("unsupported arena type %s", typed)
		}
	case VarType:
		return nil, fmt.Errorf("unexpected type variable %s in arena", typed)
	case SparseTupleType:
		return nil, fmt.Errorf("unexpected sparse tuple type %s in arena", typed)
	case OperatorType:
		return nil, fmt.Errorf("unexpected operator type %s in arena", typed)
	case RowType:
		return nil, fmt.Errorf("unexpected row type %s in arena", typed)
	case RecordRowType:
		if _, open := typed.Row().Other(); open {
			return nil, fmt.Errorf("polymorphic records are not supported by the arena")
		}
	case VariantType:
		if _, open := typed.Row().Other(); open {
			return nil, fmt.Errorf("polymorphic variants are not supported by the arena")
		}
	}
	return BasicCellType{Type: typ}, nil
}

func CellTypeFromTag(tag TypeTag) (CellType, error) {
	typ, ok := tag.Type()
	if !ok {
		return nil, fmt.Errorf("expected a typed arena expression")
	}
	return CellTypeFromType(typ)
}

func EqualCellTypes(left, right CellType) bool {
	switch l := left.(type) {
	case UnknownCellType:
		_, ok := right.(UnknownCellType)
		return ok
	case BasicCellType:
		r, ok := right.(BasicCellType)
		return ok && EqualTypes(l.Type, r.Type)
	case InfiniteSetCellType:
		r, ok := right.(InfiniteSetCellType)
		return ok && EqualCellTypes(l.Element, r.Element)
	case IntegerRangeCellType:
		_, ok := right.(IntegerRangeCellType)
		return ok
	case PowerSetCellType:
		r, ok := right.(PowerSetCellType)
		return ok && EqualTypes(l.Domain, r.Domain)
	case FiniteFunctionSetCellType:
		r, ok := right.(FiniteFunctionSetCellType)
		return ok && EqualCellTypes(l.Domain, r.Domain) && EqualCellTypes(l.Codomain, r.Codomain)
	default:
		return false
	}
}

const ArenaCellNamePrefix = "$C$"

type ArenaCell struct {
	ID            int
	Type          CellType
	Unconstrained bool
}

func NewArenaCell(id int, cellType CellType, unconstrained ...bool) ArenaCell {
	if cellType == nil {
		panic("arena cell requires a type")
	}
	value := false
	if len(unconstrained) != 0 {
		value = unconstrained[0]
	}
	return ArenaCell{ID: id, Type: cellType, Unconstrained: value}
}

func (cell ArenaCell) String() string { return fmt.Sprintf("%s%d", ArenaCellNamePrefix, cell.ID) }
func (cell ArenaCell) ToBuilder() BuilderT {
	return TLA.Name(cell.String(), cell.Type.TLAType())
}
func (cell ArenaCell) NameExpr() *NameExpr {
	return NewNameExpr(cell.String(), Typed(cell.Type.TLAType()))
}
func (cell ArenaCell) Compare(other ArenaCell) int {
	switch {
	case cell.ID < other.ID:
		return -1
	case cell.ID > other.ID:
		return 1
	default:
		return 0
	}
}
func (cell ArenaCell) Equal(other ArenaCell) bool { return cell.ID == other.ID }

func IsArenaCellName(name string) bool { return stringsHasArenaPrefix(name) }

func ArenaCellIDFromName(name string) (int, error) {
	if !stringsHasArenaPrefix(name) {
		return 0, fmt.Errorf("expected a cell name, found %s", name)
	}
	id, err := strconv.Atoi(name[len(ArenaCellNamePrefix):])
	if err != nil {
		return 0, fmt.Errorf("invalid arena cell name %s", name)
	}
	return id, nil
}

func stringsHasArenaPrefix(name string) bool {
	return len(name) >= len(ArenaCellNamePrefix) && name[:len(ArenaCellNamePrefix)] == ArenaCellNamePrefix
}

type ElementPointer interface {
	Element() ArenaCell
	ToSMT() Expr
	Generalize() SMTElementPointer
	Restrict(Expr) SMTElementPointer
	elementPointerNode()
}

type FixedElementPointer struct{ Cell ArenaCell }

func (FixedElementPointer) elementPointerNode()        {}
func (pointer FixedElementPointer) Element() ArenaCell { return pointer.Cell }
func (pointer FixedElementPointer) ToSMT() Expr {
	return NewValueExpr(BoolValue(true), Typed(BoolT1))
}
func (pointer FixedElementPointer) Generalize() SMTElementPointer {
	return SMTElementPointer{Cell: pointer.Cell, Expression: pointer.ToSMT()}
}
func (pointer FixedElementPointer) Restrict(condition Expr) SMTElementPointer {
	return SMTElementPointer{Cell: pointer.Cell, Expression: condition}
}

type SMTElementPointer struct {
	Cell       ArenaCell
	Expression Expr
}

func (SMTElementPointer) elementPointerNode()                   {}
func (pointer SMTElementPointer) Element() ArenaCell            { return pointer.Cell }
func (pointer SMTElementPointer) ToSMT() Expr                   { return pointer.Expression }
func (pointer SMTElementPointer) Generalize() SMTElementPointer { return pointer }
func (pointer SMTElementPointer) Restrict(condition Expr) SMTElementPointer {
	conditionType, err := ExpressionType(condition)
	if err != nil || !EqualTypes(conditionType, BoolT1) {
		panic(fmt.Sprintf("element-pointer restriction requires a Boolean condition, found %v", conditionType))
	}
	pointerType, err := ExpressionType(pointer.Expression)
	if err != nil || !EqualTypes(pointerType, BoolT1) {
		panic(fmt.Sprintf("SMT element pointer requires a Boolean expression, found %v", pointerType))
	}
	return SMTElementPointer{Cell: pointer.Cell, Expression: MustOperExpr(Op("AND"), Typed(BoolT1), pointer.Expression, condition)}
}
