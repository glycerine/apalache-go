package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

type quintJSONOutput struct {
	Stage    string                     `json:"stage"`
	Modules  []quintJSONModule          `json:"modules"`
	Types    map[string]quintJSONScheme `json:"types"`
	Table    json.RawMessage            `json:"table"`
	Effects  json.RawMessage            `json:"effects"`
	Warnings json.RawMessage            `json:"warnings"`
	Errors   json.RawMessage            `json:"errors"`
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
	Doc            string         `json:"doc"`
	Qualifier      string         `json:"qualifier"`
	Expr           *quintJSONExpr `json:"expr"`
	Assumption     *quintJSONExpr `json:"assumption"`
	Type           *quintJSONType `json:"type"`
	TypeAnnotation *quintJSONType `json:"typeAnnotation"`
	Depth          json.Number    `json:"depth"`
}

type quintJSONExpr struct {
	ID        json.Number            `json:"id"`
	Kind      string                 `json:"kind"`
	Name      string                 `json:"name"`
	Opcode    string                 `json:"opcode"`
	Value     json.RawMessage        `json:"value"`
	Args      []quintJSONExpr        `json:"args"`
	Params    []quintJSONLambdaParam `json:"params"`
	Qualifier string                 `json:"qualifier"`
	Expr      *quintJSONExpr         `json:"expr"`
	Opdef     *quintJSONDef          `json:"opdef"`
}

type quintJSONLambdaParam struct {
	ID             json.Number    `json:"id"`
	Name           string         `json:"name"`
	Kind           string         `json:"kind"`
	TypeAnnotation *quintJSONType `json:"typeAnnotation"`
	Depth          json.Number    `json:"depth"`
}

func (output quintJSONOutput) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Stage   string                     `json:"stage"`
		Modules []quintJSONModule          `json:"modules"`
		Types   map[string]quintJSONScheme `json:"types"`
		Table   json.RawMessage            `json:"table"`
	}{output.Stage, output.Modules, output.Types, output.Table})
}

func (definition quintJSONDef) MarshalJSON() ([]byte, error) {
	switch definition.Kind {
	case "def":
		return json.Marshal(struct {
			Kind      string         `json:"kind"`
			ID        json.Number    `json:"id"`
			Name      string         `json:"name"`
			Qualifier string         `json:"qualifier"`
			Expr      *quintJSONExpr `json:"expr"`
		}{definition.Kind, definition.ID, definition.Name, definition.Qualifier, definition.Expr})
	case "const", "var":
		return json.Marshal(struct {
			Kind           string         `json:"kind"`
			ID             json.Number    `json:"id"`
			Name           string         `json:"name"`
			TypeAnnotation *quintJSONType `json:"typeAnnotation"`
		}{definition.Kind, definition.ID, definition.Name, definition.TypeAnnotation})
	case "assume":
		return json.Marshal(struct {
			Kind       string         `json:"kind"`
			ID         json.Number    `json:"id"`
			Name       string         `json:"name"`
			Assumption *quintJSONExpr `json:"assumption"`
		}{definition.Kind, definition.ID, definition.Name, definition.Assumption})
	case "typedef":
		if definition.Type == nil {
			return json.Marshal(struct {
				Kind string      `json:"kind"`
				ID   json.Number `json:"id"`
				Name string      `json:"name"`
			}{definition.Kind, definition.ID, definition.Name})
		}
		return json.Marshal(struct {
			Kind string         `json:"kind"`
			ID   json.Number    `json:"id"`
			Name string         `json:"name"`
			Type *quintJSONType `json:"type"`
		}{definition.Kind, definition.ID, definition.Name, definition.Type})
	default:
		return nil, fmt.Errorf("unsupported Quint definition kind %q", definition.Kind)
	}
}

func (expression quintJSONExpr) MarshalJSON() ([]byte, error) {
	switch expression.Kind {
	case "name":
		return json.Marshal(struct {
			Kind string      `json:"kind"`
			ID   json.Number `json:"id"`
			Name string      `json:"name"`
		}{expression.Kind, expression.ID, expression.Name})
	case "bool", "str":
		return json.Marshal(struct {
			Kind  string          `json:"kind"`
			ID    json.Number     `json:"id"`
			Value json.RawMessage `json:"value"`
		}{expression.Kind, expression.ID, expression.Value})
	case "int":
		text := string(expression.Value)
		var quoted string
		if len(text) > 0 && text[0] == '"' {
			if err := json.Unmarshal(expression.Value, &quoted); err != nil {
				return nil, fmt.Errorf("invalid Quint integer: %w", err)
			}
			text = quoted
		}
		if _, ok := new(big.Int).SetString(text, 10); !ok {
			return nil, fmt.Errorf("invalid Quint integer %q", text)
		}
		return json.Marshal(struct {
			Kind  string          `json:"kind"`
			ID    json.Number     `json:"id"`
			Value json.RawMessage `json:"value"`
		}{expression.Kind, expression.ID, json.RawMessage(text)})
	case "app":
		return json.Marshal(struct {
			Kind   string          `json:"kind"`
			ID     json.Number     `json:"id"`
			Opcode string          `json:"opcode"`
			Args   []quintJSONExpr `json:"args"`
		}{expression.Kind, expression.ID, expression.Opcode, expression.Args})
	case "lambda":
		return json.Marshal(struct {
			Kind      string                 `json:"kind"`
			ID        json.Number            `json:"id"`
			Params    []quintJSONLambdaParam `json:"params"`
			Qualifier string                 `json:"qualifier"`
			Expr      *quintJSONExpr         `json:"expr"`
		}{expression.Kind, expression.ID, expression.Params, expression.Qualifier, expression.Expr})
	case "let":
		return json.Marshal(struct {
			Kind  string         `json:"kind"`
			ID    json.Number    `json:"id"`
			Opdef *quintJSONDef  `json:"opdef"`
			Expr  *quintJSONExpr `json:"expr"`
		}{expression.Kind, expression.ID, expression.Opdef, expression.Expr})
	default:
		return nil, fmt.Errorf("unsupported Quint expression kind %q", expression.Kind)
	}
}

func (parameter quintJSONLambdaParam) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
	}{parameter.ID, parameter.Name})
}

func (typ quintJSONType) MarshalJSON() ([]byte, error) {
	switch typ.Kind {
	case "bool", "int", "str":
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{typ.Kind})
	case "const", "var":
		return json.Marshal(struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		}{typ.Kind, typ.Name})
	case "set", "list":
		return json.Marshal(struct {
			Kind string         `json:"kind"`
			Elem *quintJSONType `json:"elem"`
		}{typ.Kind, typ.Elem})
	case "fun":
		return json.Marshal(struct {
			Kind string         `json:"kind"`
			Arg  *quintJSONType `json:"arg"`
			Res  *quintJSONType `json:"res"`
		}{typ.Kind, typ.Arg, typ.Res})
	case "oper":
		return json.Marshal(struct {
			Kind string          `json:"kind"`
			Args []quintJSONType `json:"args"`
			Res  *quintJSONType  `json:"res"`
		}{typ.Kind, typ.Args, typ.Res})
	case "tup", "rec", "sum":
		return json.Marshal(struct {
			Kind   string        `json:"kind"`
			Fields *quintJSONRow `json:"fields"`
		}{typ.Kind, typ.Fields})
	default:
		return nil, fmt.Errorf("unsupported Quint type kind %q", typ.Kind)
	}
}

func (row quintJSONRow) MarshalJSON() ([]byte, error) {
	switch row.Kind {
	case "row":
		return json.Marshal(struct {
			Kind   string                 `json:"kind"`
			Fields []quintJSONRecordField `json:"fields"`
			Other  *quintJSONRow          `json:"other"`
		}{row.Kind, row.Fields, row.Other})
	case "var":
		return json.Marshal(struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		}{row.Kind, row.Name})
	case "empty":
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{row.Kind})
	default:
		return nil, fmt.Errorf("unsupported Quint row kind %q", row.Kind)
	}
}

// MarshalQuintJSONIR validates Quint's typed JSON IR and writes it back in
// the canonical tagged-union schema used by the Scala QuintIR writers. Type
// source IDs and output diagnostics are intentionally omitted, as in Scala.
func MarshalQuintJSONIR(data []byte, pretty bool) ([]byte, error) {
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
	if err := validateQuintJSONOutput(output); err != nil {
		return nil, err
	}
	if pretty {
		return json.MarshalIndent(output, "", "  ")
	}
	return json.Marshal(output)
}

type quintJSONImporter struct {
	types   map[string]quintJSONScheme
	nullary map[string]struct{}
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
	if err := validateQuintJSONOutput(output); err != nil {
		return nil, err
	}
	if len(output.Modules) != 1 {
		return nil, fmt.Errorf("Quint JSON IR contains %d modules; exactly one is required", len(output.Modules))
	}
	module := output.Modules[0]
	nullary := make(map[string]struct{})
	for _, definition := range module.Declarations {
		if definition.Kind == "def" && definition.Qualifier != "run" && definition.Expr != nil && definition.Expr.Kind != "lambda" {
			nullary[definition.Name] = struct{}{}
		}
	}
	importer := quintJSONImporter{types: output.Types, nullary: nullary}
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

func validateQuintJSONOutput(output quintJSONOutput) error {
	missing := make([]string, 0, 4)
	if output.Stage == "" {
		missing = append(missing, "stage")
	}
	if output.Modules == nil {
		missing = append(missing, "modules")
	}
	if output.Types == nil {
		missing = append(missing, "types")
	}
	if output.Table == nil {
		missing = append(missing, "table")
	}
	if len(missing) != 0 {
		return fmt.Errorf("input was not valid Quint JSON IR: missing keys in dictionary: %s", strings.Join(missing, ", "))
	}
	return nil
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
		expression := *definition.Expr
		parameters := []OperatorParam(nil)
		declarationType := mustOperator(nil, typ)
		if expression.Kind == "lambda" {
			operatorType, ok := typ.(OperatorType)
			if !ok {
				return nil, false, fmt.Errorf("lambda definition has non-operator type %s", typ)
			}
			if expression.Expr == nil {
				return nil, false, fmt.Errorf("lambda definition has no body")
			}
			argumentTypes := operatorType.Arguments()
			if len(expression.Params) != len(argumentTypes) {
				return nil, false, fmt.Errorf("lambda has %d parameters but its type has %d arguments", len(expression.Params), len(argumentTypes))
			}
			parameters = make([]OperatorParam, len(expression.Params))
			for index, parameter := range expression.Params {
				parameters[index] = ValueParam(parameter.Name)
				if argumentType, isOperator := argumentTypes[index].(OperatorType); isOperator {
					parameters[index].Arity = len(argumentType.Arguments())
				}
			}
			expression = *expression.Expr
			declarationType = typ
		}
		body, exprErr := importer.expression(expression)
		if exprErr != nil {
			return nil, false, exprErr
		}
		return NewOperDecl(definition.Name, parameters, body, Typed(declarationType)), true, nil
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
		switch expression.Name {
		case "Bool":
			return NewValueExpr(BoolSet, Typed(mustSet(BoolT1))), nil
		case "Int":
			return NewValueExpr(IntSet, Typed(mustSet(IntT1))), nil
		case "Nat":
			return NewValueExpr(NatSet, Typed(mustSet(IntT1))), nil
		}
		if _, isNullary := importer.nullary[expression.Name]; isNullary {
			operatorType := mustOperator(nil, typ)
			name := NewNameExpr(expression.Name, Typed(operatorType))
			return NewOperExpr(Op("OPER_APP"), Typed(typ), name)
		}
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
	case "lambda":
		operatorType, ok := typ.(OperatorType)
		if !ok {
			return nil, fmt.Errorf("lambda has non-operator type %s", typ)
		}
		if expression.Expr == nil {
			return nil, fmt.Errorf("lambda has no body")
		}
		argumentTypes := operatorType.Arguments()
		if len(expression.Params) != len(argumentTypes) {
			return nil, fmt.Errorf("lambda has %d parameters but its type has %d arguments", len(expression.Params), len(argumentTypes))
		}
		body, bodyErr := importer.expression(*expression.Expr)
		if bodyErr != nil {
			return nil, bodyErr
		}
		parameters := make([]OperatorParam, len(expression.Params))
		for index, parameter := range expression.Params {
			parameters[index] = ValueParam(parameter.Name)
			if argumentType, isOperator := argumentTypes[index].(OperatorType); isOperator {
				parameters[index].Arity = len(argumentType.Arguments())
			}
		}
		name := "__QUINT_LAMBDA_" + expression.ID.String()
		declaration := NewOperDecl(name, parameters, body, Typed(operatorType))
		return NewLetInExpr(NewNameExpr(name, Typed(operatorType)), Typed(operatorType), declaration), nil
	case "let":
		if expression.Opdef == nil || expression.Expr == nil {
			return nil, fmt.Errorf("let expression is incomplete")
		}
		definition := expression.Opdef
		if labelIndex := strings.LastIndex(definition.Name, "__label_"); labelIndex >= 0 {
			scope, scopeErr := importer.expression(*expression.Expr)
			if scopeErr != nil {
				return nil, scopeErr
			}
			label := strings.TrimPrefix(definition.Name[labelIndex:], "__label_")
			return NewOperExpr(Op("LABEL"), Typed(typ), scope, NewValueExpr(StringValue(label), Typed(StrT1)))
		}
		if definition.Qualifier == "nondet" {
			if definition.Expr == nil || definition.Expr.Kind != "app" || len(definition.Expr.Args) != 1 ||
				(definition.Expr.Opcode != "oneOf" && definition.Expr.Opcode != "apalache::generate") {
				return nil, fmt.Errorf("unsupported nondeterministic Quint let definition %q", definition.Name)
			}
			boundType, typeErr := importer.typeOf(definition.Expr.ID)
			if typeErr != nil {
				return nil, typeErr
			}
			domain, domainErr := importer.expression(definition.Expr.Args[0])
			if domainErr != nil {
				return nil, domainErr
			}
			if definition.Expr.Opcode == "apalache::generate" {
				generated := MustOperExpr(Op("Apalache!Gen"), Typed(boundType), domain)
				domain = MustOperExpr(Op("SET_ENUM"), Typed(mustSet(boundType)), generated)
			}
			scope, scopeErr := importer.expression(*expression.Expr)
			if scopeErr != nil {
				return nil, scopeErr
			}
			bound := NewNameExpr(definition.Name, Typed(boundType))
			return NewOperExpr(Op("EXISTS3"), Typed(typ), bound, domain, scope)
		}
		declaration, declarationErr := importer.localOperDeclaration(*definition)
		if declarationErr != nil {
			return nil, declarationErr
		}
		scopeImporter := importer
		if definition.Expr != nil && definition.Expr.Kind != "lambda" {
			scopeImporter.nullary = cloneMap(importer.nullary)
			scopeImporter.nullary[definition.Name] = struct{}{}
		}
		scope, scopeErr := scopeImporter.expression(*expression.Expr)
		if scopeErr != nil {
			return nil, scopeErr
		}
		return NewLetInExpr(scope, Typed(typ), declaration), nil
	case "app":
		if expression.Opcode == "exists" || expression.Opcode == "forall" || expression.Opcode == "filter" ||
			expression.Opcode == "map" || expression.Opcode == "mapBy" {
			return importer.bindingExpression(expression, typ)
		}
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
			if len(arguments)%2 != 0 {
				return nil, fmt.Errorf("Rec expects alternating field/value arguments, found %d arguments", len(arguments))
			}
			for index := 0; index < len(arguments); index += 2 {
				field, ok := arguments[index].(*ValueExpr)
				if !ok {
					return nil, fmt.Errorf("Rec field argument %d is not a string literal", index)
				}
				if _, ok = field.Value().(StringValue); !ok {
					return nil, fmt.Errorf("Rec field argument %d is not a string literal", index)
				}
			}
			return NewOperExpr(Op("RECORD"), Typed(typ), arguments...)
		case "variant":
			if len(arguments) != 2 {
				return nil, fmt.Errorf("variant expects two arguments, found %d", len(arguments))
			}
			label, ok := arguments[0].(*ValueExpr)
			if !ok {
				return nil, fmt.Errorf("variant label is not a string literal")
			}
			if _, ok = label.Value().(StringValue); !ok {
				return nil, fmt.Errorf("variant label is not a string literal")
			}
			if _, ok = typ.(VariantType); !ok {
				return nil, fmt.Errorf("variant has non-sum type %s", typ)
			}
			return NewOperExpr(Op("Variants!Variant"), Typed(typ), arguments...)
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
		case "matchVariant":
			return convertQuintMatchVariant(arguments, typ)
		case "Map":
			functionType, ok := typ.(FunType)
			if !ok {
				return nil, fmt.Errorf("Map has non-function type %s", typ)
			}
			pairType := mustTuple(functionType.Argument(), functionType.Result())
			pairs := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(pairType)), arguments...)
			return NewOperExpr(Op("Apalache!SetAsFun"), Typed(typ), pairs)
		case "fold", "foldl":
			if len(arguments) != 3 {
				return nil, fmt.Errorf("%s expects three arguments, found %d", expression.Opcode, len(arguments))
			}
			operator := "Apalache!ApaFoldSet"
			if expression.Opcode == "foldl" {
				operator = "Apalache!ApaFoldSeqLeft"
			}
			return NewOperExpr(Op(operator), Typed(typ), arguments[2], arguments[1], arguments[0])
		case "nth":
			if len(arguments) != 2 {
				return nil, fmt.Errorf("nth expects two arguments, found %d", len(arguments))
			}
			index := MustOperExpr(Op("PLUS"), Typed(IntT1), arguments[1], NewValueExpr(Int64Value(1), Typed(IntT1)))
			return NewOperExpr(Op("FUN_APP"), Typed(typ), arguments[0], index)
		case "replaceAt", "slice":
			if len(arguments) != 3 {
				return nil, fmt.Errorf("%s expects three arguments, found %d", expression.Opcode, len(arguments))
			}
			from := MustOperExpr(Op("PLUS"), Typed(IntT1), arguments[1], NewValueExpr(Int64Value(1), Typed(IntT1)))
			if expression.Opcode == "slice" {
				return NewOperExpr(Op("Sequences!SubSeq"), Typed(typ), arguments[0], from, arguments[2])
			}
			path := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1)), from)
			return NewOperExpr(Op("EXCEPT"), Typed(typ), arguments[0], path, arguments[2])
		case "set", "with":
			if len(arguments) != 3 {
				return nil, fmt.Errorf("%s expects three arguments, found %d", expression.Opcode, len(arguments))
			}
			keyType := mustTagType(arguments[1].Tag())
			path := MustOperExpr(Op("TUPLE"), Typed(mustTuple(keyType)), arguments[1])
			return NewOperExpr(Op("EXCEPT"), Typed(typ), arguments[0], path, arguments[2])
		case "setBy":
			if len(arguments) != 3 {
				return nil, fmt.Errorf("setBy expects three arguments, found %d", len(arguments))
			}
			functionType, ok := typ.(FunType)
			if !ok {
				return nil, fmt.Errorf("setBy has non-function type %s", typ)
			}
			keyType := mustTagType(arguments[1].Tag())
			path := MustOperExpr(Op("TUPLE"), Typed(mustTuple(keyType)), arguments[1])
			valueType := functionType.Result()
			cacheType := mustOperator(nil, typ)
			cacheName := "__QUINT_SETBY_" + expression.ID.String()
			cache := NewNameExpr(cacheName, Typed(cacheType))
			cachedMap := MustOperExpr(Op("OPER_APP"), Typed(typ), cache)
			oldValue := MustOperExpr(Op("FUN_APP"), Typed(valueType), cachedMap, arguments[1])
			newValue := MustOperExpr(Op("OPER_APP"), Typed(valueType), arguments[2], oldValue)
			updated := MustOperExpr(Op("EXCEPT"), Typed(typ), cachedMap, path, newValue)
			declaration := NewOperDecl(cacheName, nil, arguments[0], Typed(cacheType))
			return NewLetInExpr(updated, Typed(typ), declaration), nil
		case "chooseSome":
			if len(arguments) != 1 {
				return nil, fmt.Errorf("chooseSome expects one argument, found %d", len(arguments))
			}
			binder := NewNameExpr("__QUINT_CHOOSE_"+expression.ID.String(), Typed(typ))
			return NewOperExpr(Op("CHOOSE3"), Typed(typ), binder, arguments[0], NewValueExpr(BoolValue(true), Typed(BoolT1)))
		case "indices":
			if len(arguments) != 1 {
				return nil, fmt.Errorf("indices expects one argument, found %d", len(arguments))
			}
			return convertQuintIndices(expression.ID, arguments[0], typ)
		case "put":
			if len(arguments) != 3 {
				return nil, fmt.Errorf("put expects three arguments, found %d", len(arguments))
			}
			return convertQuintPut(expression.ID, arguments, typ)
		case "range":
			if len(arguments) != 2 {
				return nil, fmt.Errorf("range expects two arguments, found %d", len(arguments))
			}
			return convertQuintRange(expression.ID, arguments[0], arguments[1], typ)
		case "select":
			if len(arguments) != 2 {
				return nil, fmt.Errorf("select expects two arguments, found %d", len(arguments))
			}
			return convertQuintSelect(expression.ID, arguments[0], arguments[1], typ)
		case "oneOf", "repeated":
			return nil, fmt.Errorf("unsupported Quint builtin opcode %q", expression.Opcode)
		}
		operator, found := quintSimpleOperators[expression.Opcode]
		if !found {
			argumentTypes := make([]Type, len(arguments))
			for index, argument := range arguments {
				argumentType, typed := argument.Tag().Type()
				if !typed {
					return nil, fmt.Errorf("argument %d to Quint operator %q is untyped", index, expression.Opcode)
				}
				argumentTypes[index] = argumentType
			}
			operatorType, typeErr := NewOperatorType(argumentTypes, typ)
			if typeErr != nil {
				return nil, typeErr
			}
			name := NewNameExpr(expression.Opcode, Typed(operatorType))
			return NewOperExpr(Op("OPER_APP"), Typed(typ), append([]Expr{name}, arguments...)...)
		}
		return NewOperExpr(Op(operator), Typed(typ), arguments...)
	default:
		return nil, fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
}

func (importer quintJSONImporter) localOperDeclaration(definition quintJSONDef) (*OperDecl, error) {
	if definition.Expr == nil {
		return nil, fmt.Errorf("local operator definition %q has no expression", definition.Name)
	}
	typ, err := importer.typeOf(definition.ID)
	if err != nil {
		return nil, err
	}
	expression := *definition.Expr
	parameters := []OperatorParam(nil)
	declarationType := mustOperator(nil, typ)
	if expression.Kind == "lambda" {
		operatorType, ok := typ.(OperatorType)
		if !ok || expression.Expr == nil {
			return nil, fmt.Errorf("local lambda definition %q has invalid type or body", definition.Name)
		}
		argumentTypes := operatorType.Arguments()
		if len(expression.Params) != len(argumentTypes) {
			return nil, fmt.Errorf("local lambda %q has %d parameters but %d argument types", definition.Name, len(expression.Params), len(argumentTypes))
		}
		parameters = make([]OperatorParam, len(expression.Params))
		for index, parameter := range expression.Params {
			parameters[index] = ValueParam(parameter.Name)
			if argumentType, isOperator := argumentTypes[index].(OperatorType); isOperator {
				parameters[index].Arity = len(argumentType.Arguments())
			}
		}
		expression = *expression.Expr
		declarationType = operatorType
	}
	body, err := importer.expression(expression)
	if err != nil {
		return nil, err
	}
	return NewOperDecl(definition.Name, parameters, body, Typed(declarationType)), nil
}

func quintAnonymousOperator(id json.Number, parameters []OperatorParam, parameterTypes []Type, result Type, body Expr) Expr {
	operatorType := mustOperator(parameterTypes, result)
	name := "__QUINT_LAMBDA_" + id.String()
	declaration := NewOperDecl(name, parameters, body, Typed(operatorType))
	return NewLetInExpr(NewNameExpr(name, Typed(operatorType)), Typed(operatorType), declaration)
}

func convertQuintIndices(id json.Number, sequence Expr, resultType Type) (Expr, error) {
	setOfInts := mustSet(IntT1)
	nullarySet := mustOperator(nil, setOfInts)
	nameText := "__QUINT_DOMAIN_" + id.String()
	name := NewNameExpr(nameText, Typed(nullarySet))
	domain := MustOperExpr(Op("OPER_APP"), Typed(setOfInts), name)
	empty := MustOperExpr(Op("SET_ENUM"), Typed(setOfInts))
	zero := NewValueExpr(Int64Value(0), Typed(IntT1))
	withZero := MustOperExpr(Op("SET_UNION2"), Typed(setOfInts), domain,
		MustOperExpr(Op("SET_ENUM"), Typed(setOfInts), zero))
	length := MustOperExpr(Op("Sequences!Len"), Typed(IntT1), sequence)
	withoutLength := MustOperExpr(Op("SET_MINUS"), Typed(setOfInts), withZero,
		MustOperExpr(Op("SET_ENUM"), Typed(setOfInts), length))
	body := MustOperExpr(Op("IF_THEN_ELSE"), Typed(resultType),
		MustOperExpr(Op("EQ"), Typed(BoolT1), domain, empty), empty, withoutLength)
	declaration := NewOperDecl(nameText, nil, MustOperExpr(Op("DOMAIN"), Typed(setOfInts), sequence), Typed(nullarySet))
	return NewLetInExpr(body, Typed(resultType), declaration), nil
}

func convertQuintPut(id json.Number, arguments []Expr, resultType Type) (Expr, error) {
	functionType, ok := resultType.(FunType)
	if !ok {
		return nil, fmt.Errorf("put has non-function type %s", resultType)
	}
	keyType := functionType.Argument()
	keySet := mustSet(keyType)
	mapNameText := "__QUINT_PUT_MAP_" + id.String()
	mapOperatorType := mustOperator(nil, resultType)
	mapName := NewNameExpr(mapNameText, Typed(mapOperatorType))
	mapValue := MustOperExpr(Op("OPER_APP"), Typed(resultType), mapName)
	domainNameText := "__QUINT_PUT_DOMAIN_" + id.String()
	domainOperatorType := mustOperator(nil, keySet)
	domainName := NewNameExpr(domainNameText, Typed(domainOperatorType))
	domainValue := MustOperExpr(Op("OPER_APP"), Typed(keySet), domainName)
	binder := NewNameExpr("__QUINT_PUT_"+id.String(), Typed(keyType))
	domain := MustOperExpr(Op("SET_UNION2"), Typed(keySet),
		MustOperExpr(Op("SET_ENUM"), Typed(keySet), arguments[1]),
		domainValue)
	body := MustOperExpr(Op("IF_THEN_ELSE"), Typed(functionType.Result()),
		MustOperExpr(Op("EQ"), Typed(BoolT1), binder, arguments[1]), arguments[2],
		MustOperExpr(Op("FUN_APP"), Typed(functionType.Result()), mapValue, binder))
	composed := MustOperExpr(Op("FUN_CTOR"), Typed(resultType), body, binder, domain)
	mapDeclaration := NewOperDecl(mapNameText, nil, arguments[0], Typed(mapOperatorType))
	domainDeclaration := NewOperDecl(domainNameText, nil,
		MustOperExpr(Op("DOMAIN"), Typed(keySet), mapValue), Typed(domainOperatorType))
	return NewLetInExpr(composed, Typed(resultType), mapDeclaration, domainDeclaration), nil
}

func convertQuintRange(id json.Number, low, high Expr, resultType Type) (Expr, error) {
	parameter := ValueParam("__QUINT_RANGE_INDEX_" + id.String())
	index := NewNameExpr(parameter.Name, Typed(IntT1))
	body := MustOperExpr(Op("MINUS"), Typed(IntT1),
		MustOperExpr(Op("PLUS"), Typed(IntT1), low, index), NewValueExpr(Int64Value(1), Typed(IntT1)))
	lambda := quintAnonymousOperator(id, []OperatorParam{parameter}, []Type{IntT1}, IntT1, body)
	length := MustOperExpr(Op("MINUS"), Typed(IntT1), high, low)
	return NewOperExpr(Op("Apalache!MkSeq"), Typed(resultType), length, lambda)
}

func convertQuintSelect(id json.Number, sequence, predicate Expr, resultType Type) (Expr, error) {
	sequenceType, ok := resultType.(SeqType)
	if !ok {
		return nil, fmt.Errorf("select has non-sequence type %s", resultType)
	}
	elementType := sequenceType.Element()
	resultParam := ValueParam("__QUINT_SELECT_RESULT_" + id.String())
	elementParam := ValueParam("__QUINT_SELECT_ELEMENT_" + id.String())
	result := NewNameExpr(resultParam.Name, Typed(resultType))
	element := NewNameExpr(elementParam.Name, Typed(elementType))
	test := MustOperExpr(Op("OPER_APP"), Typed(BoolT1), predicate, element)
	body := MustOperExpr(Op("IF_THEN_ELSE"), Typed(resultType), test,
		MustOperExpr(Op("Sequences!Append"), Typed(resultType), result, element), result)
	lambda := quintAnonymousOperator(id, []OperatorParam{resultParam, elementParam}, []Type{resultType, elementType}, resultType, body)
	empty := MustOperExpr(Op("TUPLE"), Typed(resultType))
	return NewOperExpr(Op("Apalache!ApaFoldSeqLeft"), Typed(resultType), lambda, empty, sequence)
}

func (importer quintJSONImporter) bindingExpression(expression quintJSONExpr, typ Type) (Expr, error) {
	if len(expression.Args) != 2 {
		return nil, fmt.Errorf("%s expects two arguments, found %d", expression.Opcode, len(expression.Args))
	}
	domain, err := importer.expression(expression.Args[0])
	if err != nil {
		return nil, err
	}
	binderExpression := expression.Args[1]
	var binders []Expr
	var binderTypes []Type
	var body Expr
	if binderExpression.Kind == "lambda" && len(binderExpression.Params) > 0 && binderExpression.Expr != nil {
		binders = make([]Expr, len(binderExpression.Params))
		binderTypes = make([]Type, len(binderExpression.Params))
		for index, parameter := range binderExpression.Params {
			parameterType, typeErr := importer.typeOf(parameter.ID)
			if typeErr != nil {
				return nil, typeErr
			}
			name := parameter.Name
			if name == "_" {
				name = "__QUINT_UNDERSCORE_" + parameter.ID.String()
			}
			binderTypes[index] = parameterType
			binders[index] = NewNameExpr(name, Typed(parameterType))
		}
		body, err = importer.expression(*binderExpression.Expr)
		if err != nil {
			return nil, err
		}
	} else if binderExpression.Kind == "name" {
		binderType, typeErr := importer.typeOf(binderExpression.ID)
		if typeErr != nil {
			return nil, typeErr
		}
		operatorType, isOperator := binderType.(OperatorType)
		if !isOperator || len(operatorType.Arguments()) == 0 {
			return nil, fmt.Errorf("%s requires a non-nullary operator as its second argument", expression.Opcode)
		}
		binderTypes = operatorType.Arguments()
		binders = make([]Expr, len(binderTypes))
		for index, binderType := range binderTypes {
			name := fmt.Sprintf("__QUINT_VAR_%s_%d", expression.ID, index)
			binders[index] = NewNameExpr(name, Typed(binderType))
		}
		operatorName := NewNameExpr(binderExpression.Name, Typed(operatorType))
		body = MustOperExpr(Op("OPER_APP"), Typed(operatorType.Result()), append([]Expr{operatorName}, binders...)...)
	} else {
		return nil, fmt.Errorf("%s requires a non-nullary lambda or operator as its second argument", expression.Opcode)
	}
	var binder Expr = binders[0]
	if len(binders) > 1 {
		binder = MustOperExpr(Op("TUPLE"), Typed(mustTuple(binderTypes...)), binders...)
	}
	switch expression.Opcode {
	case "exists":
		return NewOperExpr(Op("EXISTS3"), Typed(typ), binder, domain, body)
	case "forall":
		return NewOperExpr(Op("FORALL3"), Typed(typ), binder, domain, body)
	case "filter":
		return NewOperExpr(Op("SET_FILTER"), Typed(typ), binder, domain, body)
	case "map":
		return NewOperExpr(Op("SET_MAP"), Typed(typ), body, binder, domain)
	case "mapBy":
		return NewOperExpr(Op("FUN_CTOR"), Typed(typ), body, binder, domain)
	default:
		panic("unreachable Quint binding operator")
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
	"setToMap": "Apalache!SetAsFun",
}

func convertQuintMatchVariant(arguments []Expr, resultType Type) (Expr, error) {
	if len(arguments) < 3 || len(arguments)%2 == 0 {
		return nil, fmt.Errorf("matchVariant expects a value and label/eliminator pairs, found %d arguments", len(arguments))
	}
	variantType, ok := mustTagType(arguments[0].Tag()).(VariantType)
	if !ok {
		return nil, fmt.Errorf("matchVariant value has non-variant type %s", mustTagType(arguments[0].Tag()))
	}
	rowFields := variantType.Row().Fields()
	pairs := make([]Expr, 0, len(arguments)-1)
	var defaultCase Expr
	for index := 1; index < len(arguments); index += 2 {
		labelExpr := arguments[index]
		labelValue, valueOK := labelExpr.(*ValueExpr)
		if !valueOK {
			return nil, fmt.Errorf("matchVariant label is not a string literal")
		}
		labelString, stringOK := labelValue.Value().(StringValue)
		if !stringOK {
			return nil, fmt.Errorf("matchVariant label is not a string literal")
		}
		label := labelString.Value()
		eliminator := arguments[index+1]
		if label == "_" {
			empty := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)))
			defaultCase = MustOperExpr(Op("OPER_APP"), Typed(resultType), eliminator, empty)
			continue
		}
		fieldType, found := rowFields[label]
		if !found {
			return nil, fmt.Errorf("matchVariant label %q is absent from %s", label, variantType)
		}
		tag := MustOperExpr(Op("Variants!VariantTag"), Typed(StrT1), arguments[0])
		condition := MustOperExpr(Op("EQ"), Typed(BoolT1), tag, labelExpr)
		value := MustOperExpr(Op("Variants!VariantGetUnsafe"), Typed(fieldType), labelExpr, arguments[0])
		applied := MustOperExpr(Op("OPER_APP"), Typed(resultType), eliminator, value)
		pairs = append(pairs, condition, applied)
	}
	if defaultCase != nil {
		return NewOperExpr(Op("CASE_OTHER"), Typed(resultType), append([]Expr{defaultCase}, pairs...)...)
	}
	return NewOperExpr(Op("CASE"), Typed(resultType), pairs...)
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
