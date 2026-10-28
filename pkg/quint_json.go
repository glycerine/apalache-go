package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
)

type quintJSONOutput struct {
	Stage   string                     `json:"stage"`
	Modules []quintJSONModule          `json:"modules"`
	Types   map[string]quintJSONScheme `json:"types"`
	Table   json.RawMessage            `json:"table"`
}

type quintJSONModule struct {
	ID           json.Number    `json:"id"`
	Name         string         `json:"name"`
	Declarations []quintJSONDef `json:"declarations"`
}

type quintJSONScheme struct {
	Type          quintJSONType `json:"type"`
	TypeVariables []string      `json:"typeVariables"`
	RowVariables  []string      `json:"rowVariables"`
}

type quintJSONType struct {
	ID     json.Number     `json:"id"`
	Kind   string          `json:"kind"`
	Name   string          `json:"name"`
	Elem   *quintJSONType  `json:"elem"`
	Arg    *quintJSONType  `json:"arg"`
	Res    *quintJSONType  `json:"res"`
	Args   []quintJSONType `json:"args"`
	Fields *quintJSONRow   `json:"fields"`
}

type quintJSONRow struct {
	Kind   string                 `json:"kind"`
	Name   string                 `json:"name"`
	Fields []quintJSONRecordField `json:"fields"`
	Other  *quintJSONRow          `json:"other"`
}

type quintJSONRecordField struct {
	FieldName string        `json:"fieldName"`
	FieldType quintJSONType `json:"fieldType"`
}

type quintJSONDef struct {
	ID             json.Number    `json:"id"`
	Kind           string         `json:"kind"`
	Name           string         `json:"name"`
	Qualifier      string         `json:"qualifier"`
	Expr           *quintJSONExpr `json:"expr"`
	Assumption     *quintJSONExpr `json:"assumption"`
	TypeAnnotation *quintJSONType `json:"typeAnnotation"`
	Depth          json.Number    `json:"depth"`
}

type quintJSONExpr struct {
	ID     json.Number     `json:"id"`
	Kind   string          `json:"kind"`
	Name   string          `json:"name"`
	Opcode string          `json:"opcode"`
	Value  json.RawMessage `json:"value"`
	Args   []quintJSONExpr `json:"args"`
}

type quintJSONImporter struct {
	types map[string]quintJSONScheme
}

// ParseQuintJSON converts the typed JSON IR emitted by `quint typecheck --out`
// into the native Go Apalache IR. Unsupported Quint constructs return an error;
// they are never approximated or discarded.
func ParseQuintJSON(data []byte) (*Module, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var output quintJSONOutput
	if err := decoder.Decode(&output); err != nil {
		return nil, fmt.Errorf("input was not valid Quint JSON IR: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("input was not valid Quint JSON IR: trailing JSON value")
		}
		return nil, fmt.Errorf("input was not valid Quint JSON IR: %w", err)
	}
	if len(output.Modules) != 1 {
		return nil, fmt.Errorf("Quint JSON IR contains %d modules; exactly one is required", len(output.Modules))
	}
	importer := quintJSONImporter{types: output.Types}
	module := output.Modules[0]
	declarations := make([]Decl, 0, len(module.Declarations))
	for _, definition := range module.Declarations {
		declaration, include, err := importer.declaration(definition)
		if err != nil {
			return nil, fmt.Errorf("convert Quint declaration %s: %w", definition.Name, err)
		}
		if include {
			declarations = append(declarations, declaration)
		}
	}
	result := NewModule(module.Name, declarations)
	result.AddProperty(PropertyTypeChecked)
	return result, nil
}

func (importer quintJSONImporter) declaration(definition quintJSONDef) (Decl, bool, error) {
	typ, err := importer.typeOf(definition.ID)
	if err != nil && definition.Kind != "assume" && definition.Kind != "typedef" {
		return nil, false, err
	}
	switch definition.Kind {
	case "var":
		return NewVarDecl(definition.Name, Typed(typ)), true, nil
	case "const":
		return NewConstDecl(definition.Name, Typed(typ)), true, nil
	case "def":
		if definition.Qualifier == "run" {
			return nil, false, nil
		}
		if definition.Expr == nil {
			return nil, false, fmt.Errorf("operator definition has no expression")
		}
		body, exprErr := importer.expression(*definition.Expr)
		if exprErr != nil {
			return nil, false, exprErr
		}
		return NewOperDecl(definition.Name, nil, body, Typed(mustOperator(nil, typ))), true, nil
	case "assume":
		if definition.Assumption == nil {
			return nil, false, fmt.Errorf("assumption has no expression")
		}
		body, exprErr := importer.expression(*definition.Assumption)
		if exprErr != nil {
			return nil, false, exprErr
		}
		if definition.Name == "_" {
			return NewUnnamedAssumeDecl(body, Typed(BoolT1)), true, nil
		}
		return NewNamedAssumeDecl(definition.Name, body, Typed(BoolT1)), true, nil
	case "typedef":
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("unsupported definition kind %q", definition.Kind)
	}
}

func (importer quintJSONImporter) expression(expression quintJSONExpr) (Expr, error) {
	typ, err := importer.typeOf(expression.ID)
	if err != nil {
		return nil, err
	}
	switch expression.Kind {
	case "name":
		return NewNameExpr(expression.Name, Typed(typ)), nil
	case "bool":
		var value bool
		if err = json.Unmarshal(expression.Value, &value); err != nil {
			return nil, err
		}
		return NewValueExpr(BoolValue(value), Typed(BoolT1)), nil
	case "str":
		var value string
		if err = json.Unmarshal(expression.Value, &value); err != nil {
			return nil, err
		}
		return NewValueExpr(StringValue(value), Typed(StrT1)), nil
	case "int":
		var value json.Number
		decoder := json.NewDecoder(bytes.NewReader(expression.Value))
		decoder.UseNumber()
		if err = decoder.Decode(&value); err != nil {
			var text string
			if stringErr := json.Unmarshal(expression.Value, &text); stringErr != nil {
				return nil, err
			}
			value = json.Number(text)
		}
		integer, ok := new(big.Int).SetString(value.String(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid integer %q", value)
		}
		return NewValueExpr(NewIntValue(integer), Typed(IntT1)), nil
	case "app":
		arguments := make([]Expr, len(expression.Args))
		for index := range expression.Args {
			arguments[index], err = importer.expression(expression.Args[index])
			if err != nil {
				return nil, err
			}
		}
		if expression.Opcode == "assign" {
			if len(arguments) != 2 {
				return nil, fmt.Errorf("assign expects two arguments, found %d", len(arguments))
			}
			prime := MustOperExpr(Op("PRIME"), arguments[0].Tag(), arguments[0])
			return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, arguments[1]), nil
		}
		switch expression.Opcode {
		case "Set":
			return NewOperExpr(Op("SET_ENUM"), Typed(typ), arguments...)
		case "List":
			return NewOperExpr(Op("TUPLE"), Typed(typ), arguments...)
		case "Tup":
			if len(arguments) == 0 {
				return NewOperExpr(Op("RECORD"), Typed(typ),
					NewValueExpr(StringValue("tag"), Typed(StrT1)),
					NewValueExpr(StringValue("UNIT"), Typed(StrT1)))
			}
			return NewOperExpr(Op("TUPLE"), Typed(typ), arguments...)
		case "Rec":
			return NewOperExpr(Op("RECORD"), Typed(typ), arguments...)
		case "contains":
			if len(arguments) != 2 {
				return nil, fmt.Errorf("contains expects two arguments, found %d", len(arguments))
			}
			return NewOperExpr(Op("SET_IN"), Typed(typ), arguments[1], arguments[0])
		case "weakFair", "strongFair":
			if len(arguments) != 2 {
				return nil, fmt.Errorf("%s expects two arguments, found %d", expression.Opcode, len(arguments))
			}
			operator := "WEAK_FAIRNESS"
			if expression.Opcode == "strongFair" {
				operator = "STRONG_FAIRNESS"
			}
			return NewOperExpr(Op(operator), Typed(typ), arguments[1], arguments[0])
		case "assert":
			if len(arguments) != 1 {
				return nil, fmt.Errorf("assert expects one argument, found %d", len(arguments))
			}
			return arguments[0], nil
		case "fail":
			return NewOperExpr(Op("NOT"), Typed(typ), arguments...)
		}
		operator, found := quintSimpleOperators[expression.Opcode]
		if !found {
			return nil, fmt.Errorf("unsupported Quint opcode %q", expression.Opcode)
		}
		return NewOperExpr(Op(operator), Typed(typ), arguments...)
	default:
		return nil, fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
}

var quintSimpleOperators = map[string]string{
	"eq": "EQ", "neq": "NE", "iff": "EQUIV", "implies": "IMPLIES", "not": "NOT",
	"and": "AND", "or": "OR", "iadd": "PLUS", "isub": "MINUS", "imul": "MULT",
	"idiv": "DIV", "imod": "MOD", "ipow": "POW", "ilt": "LT", "igt": "GT",
	"ilte": "LE", "igte": "GE", "iuminus": "UNARY_MINUS", "actionAll": "AND",
	"actionAny": "OR", "next": "PRIME", "enabled": "ENABLED", "always": "GLOBALLY",
	"eventually": "EVENTUALLY", "leadsTo": "LEADS_TO", "ite": "IF_THEN_ELSE",
	"in": "SET_IN", "notin": "SET_NOT_IN", "union": "SET_UNION2",
	"intersect": "SET_INTERSECT", "exclude": "SET_MINUS", "subseteq": "SET_SUBSET_EQ",
	"powerset": "SET_POWERSET", "flatten": "SET_UNARY_UNION", "allLists": "Sequences!Seq",
	"isFinite": "FiniteSets!IsFiniteSet", "size": "FiniteSets!Cardinality", "to": "INT_RANGE",
	"append": "Sequences!Append", "concat": "Sequences!Concat", "head": "Sequences!Head",
	"tail": "Sequences!Tail", "length": "Sequences!Len", "item": "FUN_APP",
	"tuples": "SET_TIMES", "field": "FUN_APP", "fieldNames": "DOMAIN", "get": "FUN_APP",
	"keys": "DOMAIN", "setOfMaps": "FUN_SET", "orKeep": "STUTTER",
	"mustChange": "NO_STUTTER", "then": "COMPOSE",
}

func (importer quintJSONImporter) typeOf(id json.Number) (Type, error) {
	scheme, found := importer.types[id.String()]
	if !found {
		return nil, fmt.Errorf("no type found for id %s", id)
	}
	return convertQuintJSONType(scheme.Type)
}

func convertQuintJSONType(source quintJSONType) (Type, error) {
	return (&quintJSONTypeConverter{variables: make(map[string]int)}).convert(source)
}

type quintJSONTypeConverter struct {
	variables map[string]int
}

func (converter *quintJSONTypeConverter) variable(name string) VarType {
	number, found := converter.variables[name]
	if !found {
		number = len(converter.variables)
		converter.variables[name] = number
	}
	variable, _ := NewVarType(number)
	return variable
}

func (converter *quintJSONTypeConverter) convert(source quintJSONType) (Type, error) {
	switch source.Kind {
	case "bool":
		return BoolT1, nil
	case "int":
		return IntT1, nil
	case "str":
		return StrT1, nil
	case "const":
		constant, err := NewConstType(source.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid Quint constant type %q: %w", source.Name, err)
		}
		return constant, nil
	case "var":
		return converter.variable(source.Name), nil
	case "set":
		if source.Elem == nil {
			return nil, fmt.Errorf("set type has no element")
		}
		element, err := converter.convert(*source.Elem)
		if err != nil {
			return nil, err
		}
		return NewSetType(element)
	case "list":
		if source.Elem == nil {
			return nil, fmt.Errorf("list type has no element")
		}
		element, err := converter.convert(*source.Elem)
		if err != nil {
			return nil, err
		}
		return NewSeqType(element)
	case "fun":
		if source.Arg == nil || source.Res == nil {
			return nil, fmt.Errorf("function type is incomplete")
		}
		argument, err := converter.convert(*source.Arg)
		if err != nil {
			return nil, err
		}
		result, err := converter.convert(*source.Res)
		if err != nil {
			return nil, err
		}
		return NewFunType(argument, result)
	case "oper":
		arguments := make([]Type, len(source.Args))
		for index := range source.Args {
			converted, err := converter.convert(source.Args[index])
			if err != nil {
				return nil, err
			}
			arguments[index] = converted
		}
		if source.Res == nil {
			return nil, fmt.Errorf("operator type has no result")
		}
		result, err := converter.convert(*source.Res)
		if err != nil {
			return nil, err
		}
		return NewOperatorType(arguments, result)
	case "tup":
		if source.Fields == nil {
			return nil, fmt.Errorf("tuple type has no row")
		}
		return converter.tuple(*source.Fields)
	case "rec", "sum":
		if source.Fields == nil {
			return nil, fmt.Errorf("%s type has no row", source.Kind)
		}
		row, err := converter.row(*source.Fields)
		if err != nil {
			return nil, err
		}
		if source.Kind == "rec" {
			return NewRecordRowType(row), nil
		}
		return NewVariantType(row), nil
	default:
		return nil, fmt.Errorf("unsupported Quint type kind %q", source.Kind)
	}
}

func (converter *quintJSONTypeConverter) row(source quintJSONRow) (RowType, error) {
	fields := make(map[string]Type)
	var other *VarType
	for {
		switch source.Kind {
		case "empty":
			return NewRowType(fields, other)
		case "var":
			variable := converter.variable(source.Name)
			other = &variable
			return NewRowType(fields, other)
		case "row":
			for _, field := range source.Fields {
				converted, err := converter.convert(field.FieldType)
				if err != nil {
					return RowType{}, fmt.Errorf("convert row field %q: %w", field.FieldName, err)
				}
				fields[field.FieldName] = converted
			}
			if source.Other == nil {
				return RowType{}, fmt.Errorf("row has no tail")
			}
			source = *source.Other
		default:
			return RowType{}, fmt.Errorf("unsupported Quint row kind %q", source.Kind)
		}
	}
}

func (converter *quintJSONTypeConverter) tuple(source quintJSONRow) (Type, error) {
	// Quint's empty tuple is its unit type, encoded by Apalache as a record.
	if source.Kind == "empty" || source.Kind == "row" && len(source.Fields) == 0 {
		row, _ := NewRowType(map[string]Type{"tag": StrT1}, nil)
		return NewRecordRowType(row), nil
	}
	fields := make(map[int]Type)
	open := false
	for {
		switch source.Kind {
		case "empty":
			indices := make([]int, 0, len(fields))
			for index := range fields {
				indices = append(indices, index)
			}
			sort.Ints(indices)
			elements := make([]Type, len(indices))
			for offset, index := range indices {
				elements[offset] = fields[index]
			}
			return NewTupleType(elements...)
		case "var":
			converter.variable(source.Name)
			open = true
		case "row":
			for _, field := range source.Fields {
				zeroBased, err := strconv.Atoi(field.FieldName)
				if err != nil {
					return nil, fmt.Errorf("tuple field %q is not an integer index", field.FieldName)
				}
				converted, err := converter.convert(field.FieldType)
				if err != nil {
					return nil, fmt.Errorf("convert tuple field %q: %w", field.FieldName, err)
				}
				fields[zeroBased+1] = converted
			}
			if source.Other == nil {
				return nil, fmt.Errorf("tuple row has no tail")
			}
			source = *source.Other
			continue
		default:
			return nil, fmt.Errorf("unsupported Quint row kind %q", source.Kind)
		}
		if open {
			return NewSparseTupleType(fields)
		}
	}
}
