package apalache

import "fmt"

// DefaultValueCache creates the canonical totalization value for each type.
// Values are built through the ordinary rewriter, so all scalar constraints
// and structural arena edges are generated exactly as for source expressions.
type DefaultValueCache struct {
	rewriter *StratifiedRewriter
	values   map[string]ArenaCell
}

type DefaultValueCacheSnapshot struct {
	values map[string]ArenaCell
}

func NewDefaultValueCache(rewriter *StratifiedRewriter) *DefaultValueCache {
	return &DefaultValueCache{rewriter: rewriter, values: make(map[string]ArenaCell)}
}

func (cache *DefaultValueCache) Get(typ Type) (ArenaCell, bool) {
	if typ == nil {
		return ArenaCell{}, false
	}
	cell, found := cache.values[typ.String()]
	return cell, found
}

func (cache *DefaultValueCache) GetOrCreate(scope RewriterScope, typ Type) (RewriterScope, ArenaCell, error) {
	if typ == nil {
		return scope, ArenaCell{}, fmt.Errorf("default value type cannot be nil")
	}
	if cell, found := cache.values[typ.String()]; found {
		if _, err := scope.Arena.FindCellByName(cell.String()); err == nil {
			return scope, cell, nil
		}
		delete(cache.values, typ.String())
	}
	if functionType, ok := typ.(FunType); ok {
		var domain, relation ArenaCell
		var err error
		scope, domain, err = cache.GetOrCreate(scope, mustSet(functionType.Argument()))
		if err != nil {
			return scope, ArenaCell{}, err
		}
		scope, relation, err = cache.GetOrCreate(scope, functionRelationType(functionType))
		if err != nil {
			return scope, ArenaCell{}, err
		}
		newArena, function := scope.Arena.AppendCellType(BasicCellType{Type: functionType})
		scope.Arena = newArena.SetDomain(function, domain).SetCodomain(function, relation)
		cache.values[typ.String()] = function
		return scope, function, nil
	}
	expression, err := defaultValueExpression(typ)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	scope, cell, err := cache.rewriter.Rewrite(expression, scope)
	if err != nil {
		return scope, ArenaCell{}, err
	}
	cache.values[typ.String()] = cell
	return scope, cell, nil
}

func (cache *DefaultValueCache) Dispose() { cache.values = make(map[string]ArenaCell) }

func (cache *DefaultValueCache) Snapshot() DefaultValueCacheSnapshot {
	return DefaultValueCacheSnapshot{values: cloneMap(cache.values)}
}

func (cache *DefaultValueCache) Recover(snapshot DefaultValueCacheSnapshot) {
	cache.values = cloneMap(snapshot.values)
}

func defaultValueExpression(typ Type) (Expr, error) {
	switch typed := typ.(type) {
	case primitiveType:
		switch typed {
		case BoolT1:
			return NewValueExpr(BoolValue(false), Typed(BoolT1)), nil
		case IntT1:
			return NewValueExpr(Int64Value(0), Typed(IntT1)), nil
		case StrT1:
			return NewValueExpr(StringValue("None"), Typed(StrT1)), nil
		}
	case ConstType:
		return NewValueExpr(StringValue(ConstructModelValue(typed.Name(), "None")), Typed(typed)), nil
	case SetType:
		return MustOperExpr(Op("SET_ENUM"), Typed(typed)), nil
	case TupleType:
		elementTypes := typed.Elements()
		elements := make([]Expr, len(elementTypes))
		for index, elementType := range elementTypes {
			value, err := defaultValueExpression(elementType)
			if err != nil {
				return nil, err
			}
			elements[index] = value
		}
		return MustOperExpr(Op("TUPLE"), Typed(typed), elements...), nil
	case SeqType:
		return MustOperExpr(Op("TUPLE"), Typed(typed)), nil
	case RecordType:
		return defaultRecordExpression(typed, typed.Fields())
	case RecordRowType:
		if _, open := typed.Row().Other(); open {
			return nil, fmt.Errorf("cannot construct a default for open record type %s", typed)
		}
		return defaultRecordExpression(typed, typed.Row().Fields())
	case VariantType:
		if _, open := typed.Row().Other(); open {
			return nil, fmt.Errorf("cannot construct a default for open variant type %s", typed)
		}
		options := typed.Row().Fields()
		keys := sortedStringKeys(options)
		if len(keys) == 0 {
			return nil, fmt.Errorf("cannot construct a default for empty variant type %s", typed)
		}
		value, err := defaultValueExpression(options[keys[0]])
		if err != nil {
			return nil, err
		}
		return MustOperExpr(Op("Variants!Variant"), Typed(typed),
			NewValueExpr(StringValue(keys[0]), Typed(StrT1)), value), nil
	}
	return nil, fmt.Errorf("default values are not yet implemented for %s", typ)
}

func defaultRecordExpression(typ Type, fields map[string]Type) (Expr, error) {
	keys := sortedStringKeys(fields)
	arguments := make([]Expr, 0, len(keys)*2)
	for _, key := range keys {
		value, err := defaultValueExpression(fields[key])
		if err != nil {
			return nil, err
		}
		arguments = append(arguments, NewValueExpr(StringValue(key), Typed(StrT1)), value)
	}
	return MustOperExpr(Op("RECORD"), Typed(typ), arguments...), nil
}
