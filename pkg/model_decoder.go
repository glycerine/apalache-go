package apalache

import (
	"fmt"
	"sort"
)

// SymbolicDecoder reconstructs TLA+ values from a satisfiable g3 model and
// the arena graph that produced it.
type SymbolicDecoder struct {
	encoder  *G3GroundEncoder
	rewriter *StratifiedRewriter
}

func NewSymbolicDecoder(encoder *G3GroundEncoder, rewriter *StratifiedRewriter) (*SymbolicDecoder, error) {
	if encoder == nil || rewriter == nil {
		return nil, fmt.Errorf("symbolic decoder requires an encoder and rewriter")
	}
	return &SymbolicDecoder{encoder: encoder, rewriter: rewriter}, nil
}

func (decoder *SymbolicDecoder) DecodeBinding(arena *PureArena, binding Binding) (map[string]Expr, error) {
	result := make(map[string]Expr, len(binding.Map()))
	names := make([]string, 0, len(binding.Map()))
	for name := range binding.Map() {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cell, _ := binding.Get(name)
		value, err := decoder.DecodeCell(arena, cell)
		if err != nil {
			return nil, fmt.Errorf("decode %s from %s: %w", name, cell, err)
		}
		result[name] = value
	}
	return result, nil
}

func (decoder *SymbolicDecoder) DecodeCell(arena *PureArena, cell ArenaCell) (Expr, error) {
	if arena == nil {
		return nil, fmt.Errorf("cannot decode a cell without an arena")
	}
	switch typed := cell.Type.(type) {
	case BasicCellType:
		return decoder.decodeBasic(arena, cell, typed.Type)
	case InfiniteSetCellType:
		switch cell.ID {
		case arena.IntSetCell().ID:
			return NewValueExpr(IntSet, Typed(mustSet(IntT1))), nil
		case arena.NatSetCell().ID:
			return NewValueExpr(NatSet, Typed(mustSet(IntT1))), nil
		default:
			return nil, fmt.Errorf("unknown infinite-set cell %s", cell)
		}
	case PowerSetCellType:
		domain, err := arena.Domain(cell)
		if err != nil {
			return nil, err
		}
		decoded, err := decoder.DecodeCell(arena, domain)
		if err != nil {
			return nil, err
		}
		return NewOperExpr(Op("SET_POWERSET"), Typed(typed.TLAType()), decoded)
	case FiniteFunctionSetCellType:
		domain, err := arena.Domain(cell)
		if err != nil {
			return nil, err
		}
		codomain, err := arena.Codomain(cell)
		if err != nil {
			return nil, err
		}
		from, err := decoder.DecodeCell(arena, domain)
		if err != nil {
			return nil, err
		}
		to, err := decoder.DecodeCell(arena, codomain)
		if err != nil {
			return nil, err
		}
		return NewOperExpr(Op("FUN_SET"), Typed(typed.TLAType()), from, to)
	default:
		return nil, fmt.Errorf("unsupported arena cell type %s", cell.Type)
	}
}

func (decoder *SymbolicDecoder) decodeBasic(arena *PureArena, cell ArenaCell, typ Type) (Expr, error) {
	switch typed := typ.(type) {
	case primitiveType:
		switch typed {
		case BoolT1:
			value, err := decoder.encoder.EvaluateBool(cell.NameExpr())
			return NewValueExpr(BoolValue(value), Typed(BoolT1)), err
		case IntT1:
			value, err := decoder.encoder.EvaluateInt(cell.NameExpr())
			if err != nil {
				return nil, err
			}
			return NewValueExpr(NewIntValue(value), Typed(IntT1)), nil
		case StrT1:
			return decoder.decodeUninterpreted(cell, StrT1)
		}
	case ConstType:
		return decoder.decodeUninterpreted(cell, typed)
	case SetType:
		return decoder.decodeFiniteSet(arena, cell, typed)
	case TupleType:
		return decoder.decodeTuple(arena, cell, typed)
	case SeqType:
		return decoder.decodeSequence(arena, cell, typed)
	case RecordType:
		return decoder.decodeLegacyRecord(arena, cell, typed)
	case RecordRowType:
		if _, open := typed.Row().Other(); open {
			return nil, fmt.Errorf("cannot decode an open row record")
		}
		return decoder.decodeRecord(arena, cell, typed.Row().Fields(), typed)
	case VariantType:
		return decoder.decodeVariant(arena, cell, typed)
	case FunType:
		return decoder.decodeFunction(arena, cell, typed)
	}
	return nil, fmt.Errorf("unsupported basic TLA+ type %s", typ)
}

func (decoder *SymbolicDecoder) decodeLegacyRecord(arena *PureArena, cell ArenaCell, typ RecordType) (Expr, error) {
	domain, err := arena.Domain(cell)
	if err != nil {
		return nil, fmt.Errorf("decode legacy record domain: %w", err)
	}
	decodedDomain, err := decoder.DecodeCell(arena, domain)
	if err != nil {
		return nil, err
	}
	domainSet, ok := decodedDomain.(*OperExpr)
	if !ok || domainSet.Operator().Name() != "SET_ENUM" {
		return nil, fmt.Errorf("legacy record %s has a malformed decoded domain", cell)
	}
	present := make(map[string]struct{}, len(domainSet.Arguments()))
	for _, element := range domainSet.Arguments() {
		valueExpression, ok := element.(*ValueExpr)
		if !ok {
			return nil, fmt.Errorf("legacy record %s has a nonliteral domain key", cell)
		}
		value, ok := valueExpression.Value().(StringValue)
		if !ok {
			return nil, fmt.Errorf("legacy record %s has a nonstring domain key", cell)
		}
		present[string(value)] = struct{}{}
	}
	fields := typ.Fields()
	keys := sortedStringKeys(fields)
	pointers := arena.GetHas(cell)
	if len(pointers) != len(keys) {
		return nil, fmt.Errorf("record %s has %d of %d totalized fields", cell, len(pointers), len(keys))
	}
	arguments := make([]Expr, 0, len(present)*2)
	for index, key := range keys {
		if _, found := present[key]; !found {
			continue
		}
		value, decodeErr := decoder.DecodeCell(arena, pointers[index].Element())
		if decodeErr != nil {
			return nil, decodeErr
		}
		arguments = append(arguments, NewValueExpr(StringValue(key), Typed(StrT1)), value)
	}
	return NewOperExpr(Op("RECORD"), Typed(typ), arguments...)
}

func (decoder *SymbolicDecoder) decodeUninterpreted(cell ArenaCell, typ Type) (Expr, error) {
	if literal, found := decoder.rewriter.LiteralCache().FindLiteral(cell); found {
		return NewValueExpr(StringValue(literal.Index), Typed(typ)), nil
	}
	for _, candidate := range decoder.rewriter.LiteralCache().Values() {
		if !EqualTypes(candidate.Type.TLAType(), typ) {
			continue
		}
		equal := MustOperExpr(Op("EQ"), Typed(BoolT1), cell.NameExpr(), candidate.NameExpr())
		matches, err := decoder.encoder.EvaluateBool(equal)
		if err != nil {
			return nil, err
		}
		if matches {
			literal, _ := decoder.rewriter.LiteralCache().FindLiteral(candidate)
			return NewValueExpr(StringValue(literal.Index), Typed(typ)), nil
		}
	}
	return NewValueExpr(StringValue(fmt.Sprintf("FRESH%d", cell.ID)), Typed(typ)), nil
}

func (decoder *SymbolicDecoder) decodeFiniteSet(arena *PureArena, cell ArenaCell, typ SetType) (Expr, error) {
	elements := make([]Expr, 0)
	for _, pointer := range arena.GetHas(cell) {
		included := true
		if _, fixed := pointer.(FixedElementPointer); !fixed {
			var err error
			included, err = decoder.encoder.EvaluateBool(pointer.ToSMT())
			if err != nil {
				return nil, err
			}
		}
		if included {
			decoded, err := decoder.DecodeCell(arena, pointer.Element())
			if err != nil {
				return nil, err
			}
			elements = append(elements, decoded)
		}
	}
	sortDecodedExpressions(elements)
	elements = uniqueDecodedExpressions(elements)
	return NewOperExpr(Op("SET_ENUM"), Typed(typ), elements...)
}

func (decoder *SymbolicDecoder) decodeTuple(arena *PureArena, cell ArenaCell, typ TupleType) (Expr, error) {
	pointers := arena.GetHas(cell)
	if len(pointers) != len(typ.Elements()) {
		return nil, fmt.Errorf("tuple %s has %d of %d elements", cell, len(pointers), len(typ.Elements()))
	}
	elements := make([]Expr, len(pointers))
	for index, pointer := range pointers {
		var err error
		elements[index], err = decoder.DecodeCell(arena, pointer.Element())
		if err != nil {
			return nil, err
		}
	}
	return NewOperExpr(Op("TUPLE"), Typed(typ), elements...)
}

func (decoder *SymbolicDecoder) decodeSequence(arena *PureArena, cell ArenaCell, typ SeqType) (Expr, error) {
	length, _, pointers, err := unpackSequence(arena, cell)
	if err != nil {
		return nil, err
	}
	value, err := decoder.encoder.EvaluateInt(length.NameExpr())
	if err != nil {
		return nil, err
	}
	count := 0
	if value.Sign() > 0 && (!value.IsInt64() || value.Int64() > int64(len(pointers))) {
		count = len(pointers)
	} else if value.Sign() > 0 {
		count = int(value.Int64())
	}
	elements := make([]Expr, count)
	for index := range count {
		elements[index], err = decoder.DecodeCell(arena, pointers[index].Element())
		if err != nil {
			return nil, err
		}
	}
	return NewOperExpr(Op("TUPLE"), Typed(typ), elements...)
}

func (decoder *SymbolicDecoder) decodeRecord(arena *PureArena, cell ArenaCell, fields map[string]Type, typ Type) (Expr, error) {
	keys := sortedStringKeys(fields)
	pointers := arena.GetHas(cell)
	if len(pointers) != len(keys) {
		return nil, fmt.Errorf("record %s has %d of %d fields", cell, len(pointers), len(keys))
	}
	arguments := make([]Expr, 0, len(keys)*2)
	for index, key := range keys {
		value, err := decoder.DecodeCell(arena, pointers[index].Element())
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, NewValueExpr(StringValue(key), Typed(StrT1)), value)
	}
	return NewOperExpr(Op("RECORD"), Typed(typ), arguments...)
}

func (decoder *SymbolicDecoder) decodeVariant(arena *PureArena, cell ArenaCell, typ VariantType) (Expr, error) {
	tagCell, err := variantTag(arena, cell)
	if err != nil {
		return nil, err
	}
	tagExpression, err := decoder.DecodeCell(arena, tagCell)
	if err != nil {
		return nil, err
	}
	tagValue, ok := tagExpression.(*ValueExpr)
	if !ok {
		return nil, fmt.Errorf("variant tag is not a value")
	}
	tag, ok := tagValue.Value().(StringValue)
	if !ok {
		return nil, fmt.Errorf("variant tag is not a string")
	}
	valueCell, err := variantField(arena, cell, string(tag))
	if err != nil {
		return nil, err
	}
	value, err := decoder.DecodeCell(arena, valueCell)
	if err != nil {
		return nil, err
	}
	return NewOperExpr(Op("Variants!Variant"), Typed(typ),
		NewValueExpr(StringValue(tag), Typed(StrT1)), value)
}

func (decoder *SymbolicDecoder) decodeFunction(arena *PureArena, cell ArenaCell, typ FunType) (Expr, error) {
	relation, err := arena.Codomain(cell)
	if err != nil {
		return nil, err
	}
	pairType, _ := NewTupleType(typ.Argument(), typ.Result())
	pairs := make([]Expr, 0)
	seen := make(map[string]struct{})
	for _, pointer := range arena.GetHas(relation) {
		included := true
		if _, fixed := pointer.(FixedElementPointer); !fixed {
			included, err = decoder.encoder.EvaluateBool(pointer.ToSMT())
			if err != nil {
				return nil, err
			}
		}
		if !included {
			continue
		}
		components := arena.GetHas(pointer.Element())
		if len(components) != 2 {
			return nil, fmt.Errorf("function relation pair %s has %d components", pointer.Element(), len(components))
		}
		key, err := decoder.DecodeCell(arena, components[0].Element())
		if err != nil {
			return nil, err
		}
		keyText, err := FormatTLAExpression(key)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[keyText]; duplicate {
			continue
		}
		seen[keyText] = struct{}{}
		value, err := decoder.DecodeCell(arena, components[1].Element())
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, MustOperExpr(Op("TUPLE"), Typed(pairType), key, value))
	}
	sortDecodedExpressions(pairs)
	relationType := mustSet(pairType)
	set := MustOperExpr(Op("SET_ENUM"), Typed(relationType), pairs...)
	return NewOperExpr(Op("Apalache!SetAsFun"), Typed(typ), set)
}

func sortDecodedExpressions(expressions []Expr) {
	sort.SliceStable(expressions, func(left, right int) bool {
		leftText, leftErr := FormatTLAExpression(expressions[left])
		rightText, rightErr := FormatTLAExpression(expressions[right])
		if leftErr != nil || rightErr != nil {
			return expressions[left].ID().Compare(expressions[right].ID()) < 0
		}
		return leftText < rightText
	})
}

func uniqueDecodedExpressions(expressions []Expr) []Expr {
	if len(expressions) < 2 {
		return expressions
	}
	result := expressions[:0]
	previous := ""
	for index, expression := range expressions {
		text, err := FormatTLAExpression(expression)
		if err != nil {
			text = expression.ID().String()
		}
		if index == 0 || text != previous {
			result = append(result, expression)
		}
		previous = text
	}
	return result
}
