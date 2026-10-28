package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
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
	ID   json.Number     `json:"id"`
	Kind string          `json:"kind"`
	Name string          `json:"name"`
	Elem *quintJSONType  `json:"elem"`
	Arg  *quintJSONType  `json:"arg"`
	Res  *quintJSONType  `json:"res"`
	Args []quintJSONType `json:"args"`
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
}

func (importer quintJSONImporter) typeOf(id json.Number) (Type, error) {
	scheme, found := importer.types[id.String()]
	if !found {
		return nil, fmt.Errorf("no type found for id %s", id)
	}
	return convertQuintJSONType(scheme.Type)
}

func convertQuintJSONType(source quintJSONType) (Type, error) {
	switch source.Kind {
	case "bool":
		return BoolT1, nil
	case "int":
		return IntT1, nil
	case "str":
		return StrT1, nil
	case "set":
		if source.Elem == nil {
			return nil, fmt.Errorf("set type has no element")
		}
		element, err := convertQuintJSONType(*source.Elem)
		if err != nil {
			return nil, err
		}
		return NewSetType(element)
	case "list":
		if source.Elem == nil {
			return nil, fmt.Errorf("list type has no element")
		}
		element, err := convertQuintJSONType(*source.Elem)
		if err != nil {
			return nil, err
		}
		return NewSeqType(element)
	case "fun":
		if source.Arg == nil || source.Res == nil {
			return nil, fmt.Errorf("function type is incomplete")
		}
		argument, err := convertQuintJSONType(*source.Arg)
		if err != nil {
			return nil, err
		}
		result, err := convertQuintJSONType(*source.Res)
		if err != nil {
			return nil, err
		}
		return NewFunType(argument, result)
	case "oper":
		arguments := make([]Type, len(source.Args))
		for index := range source.Args {
			converted, err := convertQuintJSONType(source.Args[index])
			if err != nil {
				return nil, err
			}
			arguments[index] = converted
		}
		if source.Res == nil {
			return nil, fmt.Errorf("operator type has no result")
		}
		result, err := convertQuintJSONType(*source.Res)
		if err != nil {
			return nil, err
		}
		return NewOperatorType(arguments, result)
	default:
		return nil, fmt.Errorf("unsupported Quint type kind %q", source.Kind)
	}
}
