package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

const ApalacheIRJSONVersion = "1.0"
const apalacheIRJSONDescription = "https://apalache-mc.org/docs/adr/005adr-json.html"

type IRJSONOptions struct{ SourceStore *SourceStore }

type irJSONRoot struct {
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Modules     []irJSONModule `json:"modules"`
}

type irJSONModule struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Declarations []any  `json:"declarations"`
}

type irJSONPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}
type irJSONSource struct {
	Filename string         `json:"filename"`
	From     irJSONPosition `json:"from"`
	To       irJSONPosition `json:"to"`
}

func MarshalIRJSON(modules []*Module, options IRJSONOptions) ([]byte, error) {
	root := irJSONRoot{Name: "ApalacheIR", Version: ApalacheIRJSONVersion, Description: apalacheIRJSONDescription,
		Modules: make([]irJSONModule, len(modules))}
	for index, module := range modules {
		if module == nil {
			return nil, fmt.Errorf("module %d is nil", index)
		}
		encoded, err := encodeIRModule(module, options)
		if err != nil {
			return nil, err
		}
		root.Modules[index] = encoded
	}
	return json.Marshal(root)
}

func MarshalSingleModuleIRJSON(module *Module, options IRJSONOptions) ([]byte, error) {
	return MarshalIRJSON([]*Module{module}, options)
}

func encodeIRModule(module *Module, options IRJSONOptions) (irJSONModule, error) {
	result := irJSONModule{Kind: "TlaModule", Name: module.Name(), Declarations: make([]any, len(module.Declarations()))}
	for index, declaration := range module.Declarations() {
		encoded, err := encodeIRDeclaration(declaration, options)
		if err != nil {
			return result, fmt.Errorf("declaration %q: %w", declaration.Name(), err)
		}
		result.Declarations[index] = encoded
	}
	return result, nil
}

func jsonTypeTag(tag TypeTag) string {
	if typ, ok := tag.Type(); ok {
		return typ.String()
	}
	return "Untyped"
}

func jsonSource(id UID, options IRJSONOptions) any {
	if options.SourceStore == nil {
		return nil
	}
	location, found := options.SourceStore.Find(id)
	if !found {
		return "UNKNOWN"
	}
	return irJSONSource{Filename: location.Filename,
		From: irJSONPosition{Line: location.Region.Start.Line, Column: location.Region.Start.Column},
		To:   irJSONPosition{Line: location.Region.End.Line, Column: location.Region.End.Column}}
}

func encodeIRDeclaration(declaration Decl, options IRJSONOptions) (map[string]any, error) {
	result := map[string]any{"kind": "", "type": jsonTypeTag(declaration.Tag())}
	if source := jsonSource(declaration.ID(), options); source != nil {
		result["source"] = source
	}
	switch typed := declaration.(type) {
	case *ConstDecl:
		result["kind"], result["name"] = "TlaConstDecl", typed.Name()
	case *VarDecl:
		result["kind"], result["name"] = "TlaVarDecl", typed.Name()
	case *OperDecl:
		body, err := encodeIRExpression(typed.Body(), options)
		if err != nil {
			return nil, err
		}
		parameters := make([]map[string]any, len(typed.FormalParams()))
		for index, parameter := range typed.FormalParams() {
			parameters[index] = map[string]any{"kind": "OperParam", "name": parameter.Name, "arity": parameter.Arity}
		}
		result["kind"], result["name"], result["formalParams"] = "TlaOperDecl", typed.Name(), parameters
		result["isRecursive"], result["body"] = typed.IsRecursive(), body
	case *AssumeDecl:
		body, err := encodeIRExpression(typed.Body(), options)
		if err != nil {
			return nil, err
		}
		result["kind"], result["body"] = "TlaAssumeDecl", body
		if name, named := typed.DefinedName(); named {
			result["name"] = name
		}
	case *TheoremDecl:
		body, err := encodeIRExpression(typed.Body(), options)
		if err != nil {
			return nil, err
		}
		result["kind"], result["name"], result["body"] = "TlaTheoremDecl", typed.Name(), body
	default:
		return nil, fmt.Errorf("unsupported declaration type %T", declaration)
	}
	return result, nil
}

func encodeIRExpression(expression Expr, options IRJSONOptions) (map[string]any, error) {
	if _, null := expression.(*nullExpr); null {
		return map[string]any{"kind": "NullEx"}, nil
	}
	result := map[string]any{"kind": "", "type": jsonTypeTag(expression.Tag())}
	if source := jsonSource(expression.ID(), options); source != nil {
		result["source"] = source
	}
	switch typed := expression.(type) {
	case *NameExpr:
		result["kind"], result["name"] = "NameEx", typed.Name()
	case *ValueExpr:
		value, err := encodeIRValue(typed.Value())
		if err != nil {
			return nil, err
		}
		result["kind"], result["value"] = "ValEx", value
	case *OperExpr:
		arguments := make([]any, len(typed.Arguments()))
		for index, argument := range typed.Arguments() {
			encoded, err := encodeIRExpression(argument, options)
			if err != nil {
				return nil, err
			}
			arguments[index] = encoded
		}
		result["kind"], result["oper"], result["args"] = "OperEx", typed.Operator().Name(), arguments
	case *LetInExpr:
		body, err := encodeIRExpression(typed.Body(), options)
		if err != nil {
			return nil, err
		}
		declarations := make([]any, len(typed.Declarations()))
		for index, declaration := range typed.Declarations() {
			encoded, err := encodeIRDeclaration(declaration, options)
			if err != nil {
				return nil, err
			}
			declarations[index] = encoded
		}
		result["kind"], result["body"], result["decls"] = "LetInEx", body, declarations
	default:
		return nil, fmt.Errorf("unsupported expression type %T", expression)
	}
	return result, nil
}

func encodeIRValue(value Value) (map[string]any, error) {
	switch typed := value.(type) {
	case StringValue:
		return map[string]any{"kind": "TlaStr", "value": typed.Value()}, nil
	case BoolValue:
		return map[string]any{"kind": "TlaBool", "value": typed.Bool()}, nil
	case IntValue:
		integer := typed.BigInt()
		if integer.IsInt64() && integer.Int64() >= -2147483648 && integer.Int64() <= 2147483647 {
			return map[string]any{"kind": "TlaInt", "value": integer.Int64()}, nil
		}
		return map[string]any{"kind": "TlaInt", "value": map[string]any{"bigInt": integer.String()}}, nil
	case DecimalValue:
		text, err := finiteDecimalString(typed.Rat())
		if err != nil {
			return nil, err
		}
		return map[string]any{"kind": "TlaDecimal", "value": text}, nil
	case PredefinedSet:
		kind := map[PredefinedSet]string{BoolSet: "TlaBoolSet", IntSet: "TlaIntSet", StringSet: "TlaStrSet", NatSet: "TlaNatSet"}[typed]
		if kind == "" {
			return nil, fmt.Errorf("unsupported predefined set %s", typed.Name())
		}
		return map[string]any{"kind": kind}, nil
	default:
		return nil, fmt.Errorf("unsupported TLA value %T", value)
	}
}

func finiteDecimalString(rational *big.Rat) (string, error) {
	denominator := new(big.Int).Set(rational.Denom())
	two, five := 0, 0
	for new(big.Int).Mod(denominator, big.NewInt(2)).Sign() == 0 {
		denominator.Div(denominator, big.NewInt(2))
		two++
	}
	for new(big.Int).Mod(denominator, big.NewInt(5)).Sign() == 0 {
		denominator.Div(denominator, big.NewInt(5))
		five++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return "", fmt.Errorf("decimal value is not finite: %s", rational)
	}
	digits := two
	if five > digits {
		digits = five
	}
	return rational.FloatString(digits), nil
}

func UnmarshalIRJSON(data []byte, sourceStore *SourceStore) ([]*Module, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]json.RawMessage
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode ApalacheIR JSON: %w", err)
	}
	var version string
	if err := requiredJSON(root, "version", &version); err != nil {
		return nil, err
	}
	if version != ApalacheIRJSONVersion {
		return nil, fmt.Errorf("JSON version is %s, expected %s", version, ApalacheIRJSONVersion)
	}
	var rawModules []json.RawMessage
	if err := requiredJSON(root, "modules", &rawModules); err != nil {
		return nil, err
	}
	modules := make([]*Module, len(rawModules))
	for index, raw := range rawModules {
		module, err := decodeIRModule(raw, sourceStore)
		if err != nil {
			return nil, fmt.Errorf("module %d: %w", index, err)
		}
		modules[index] = module
	}
	return modules, nil
}

func UnmarshalSingleModuleIRJSON(data []byte, sourceStore *SourceStore) (*Module, error) {
	modules, err := UnmarshalIRJSON(data, sourceStore)
	if err != nil {
		return nil, err
	}
	if len(modules) != 1 {
		return nil, fmt.Errorf("JSON included %d modules, expected one", len(modules))
	}
	return modules[0], nil
}

func decodeIRModule(raw json.RawMessage, store *SourceStore) (*Module, error) {
	object, err := rawObject(raw)
	if err != nil {
		return nil, err
	}
	var kind, name string
	if err = requiredJSON(object, "kind", &kind); err != nil {
		return nil, err
	}
	if kind != "TlaModule" {
		return nil, fmt.Errorf("JSON kind is %s, expected TlaModule", kind)
	}
	if err = requiredJSON(object, "name", &name); err != nil {
		return nil, err
	}
	var rawDeclarations []json.RawMessage
	if err = requiredJSON(object, "declarations", &rawDeclarations); err != nil {
		return nil, err
	}
	declarations := make([]Decl, len(rawDeclarations))
	for index, rawDeclaration := range rawDeclarations {
		declarations[index], err = decodeIRDeclaration(rawDeclaration, store)
		if err != nil {
			return nil, fmt.Errorf("declaration %d: %w", index, err)
		}
	}
	return NewModule(name, declarations), nil
}

func decodeIRDeclaration(raw json.RawMessage, store *SourceStore) (Decl, error) {
	object, err := rawObject(raw)
	if err != nil {
		return nil, err
	}
	kind, tag, err := decodeKindAndTag(object, true)
	if err != nil {
		return nil, err
	}
	var declaration Decl
	switch kind {
	case "TlaConstDecl", "TlaVarDecl":
		var name string
		if err = requiredJSON(object, "name", &name); err != nil {
			return nil, err
		}
		if kind == "TlaConstDecl" {
			declaration = NewConstDecl(name, tag)
		} else {
			declaration = NewVarDecl(name, tag)
		}
	case "TlaOperDecl":
		var name string
		var recursive bool
		var params []struct {
			Kind  string `json:"kind"`
			Name  string `json:"name"`
			Arity int    `json:"arity"`
		}
		if err = requiredJSON(object, "name", &name); err != nil {
			return nil, err
		}
		if err = requiredJSON(object, "isRecursive", &recursive); err != nil {
			return nil, err
		}
		if err = requiredJSON(object, "formalParams", &params); err != nil {
			return nil, err
		}
		parameters := make([]OperatorParam, len(params))
		for i, p := range params {
			if p.Kind != "OperParam" {
				return nil, fmt.Errorf("%s is not a valid OperParam kind", p.Kind)
			}
			parameters[i] = OperatorParam{Name: p.Name, Arity: p.Arity}
		}
		bodyRaw, ok := object["body"]
		if !ok {
			return nil, missingJSONField("body")
		}
		body, e := decodeIRExpression(bodyRaw, store)
		if e != nil {
			return nil, e
		}
		op := NewOperDecl(name, parameters, body, tag)
		op.SetRecursive(recursive)
		declaration = op
	case "TlaAssumeDecl":
		bodyRaw, ok := object["body"]
		if !ok {
			return nil, missingJSONField("body")
		}
		body, e := decodeIRExpression(bodyRaw, store)
		if e != nil {
			return nil, e
		}
		if rawName, ok := object["name"]; ok {
			var name string
			if e = json.Unmarshal(rawName, &name); e != nil {
				return nil, e
			}
			declaration = NewNamedAssumeDecl(name, body, tag)
		} else {
			declaration = NewUnnamedAssumeDecl(body, tag)
		}
	case "TlaTheoremDecl":
		var name string
		if err = requiredJSON(object, "name", &name); err != nil {
			return nil, err
		}
		bodyRaw, ok := object["body"]
		if !ok {
			return nil, missingJSONField("body")
		}
		body, e := decodeIRExpression(bodyRaw, store)
		if e != nil {
			return nil, e
		}
		declaration = NewTheoremDecl(name, body, tag)
	default:
		return nil, fmt.Errorf("%s is not a valid TlaDecl kind", kind)
	}
	if err = decodeIRSource(object, declaration.ID(), store); err != nil {
		return nil, err
	}
	return declaration, nil
}

func decodeIRExpression(raw json.RawMessage, store *SourceStore) (Expr, error) {
	object, err := rawObject(raw)
	if err != nil {
		return nil, err
	}
	var kind string
	if err = requiredJSON(object, "kind", &kind); err != nil {
		return nil, err
	}
	if kind == "NullEx" {
		return NullExpr(), nil
	}
	_, tag, err := decodeKindAndTag(object, true)
	if err != nil {
		return nil, err
	}
	var expression Expr
	switch kind {
	case "NameEx":
		var name string
		if err = requiredJSON(object, "name", &name); err != nil {
			return nil, err
		}
		expression = NewNameExpr(name, tag)
	case "ValEx":
		valueRaw, ok := object["value"]
		if !ok {
			return nil, missingJSONField("value")
		}
		value, e := decodeIRValue(valueRaw)
		if e != nil {
			return nil, e
		}
		expression = NewValueExpr(value, tag)
	case "OperEx":
		var name string
		var argumentsRaw []json.RawMessage
		if err = requiredJSON(object, "oper", &name); err != nil {
			return nil, err
		}
		if err = requiredJSON(object, "args", &argumentsRaw); err != nil {
			return nil, err
		}
		operator, found := LookupOperator(name)
		if !found && strings.HasPrefix(name, "Apalache!SmtMap_") {
			mapped, mappedFound := LookupOperator(strings.TrimPrefix(name, "Apalache!SmtMap_"))
			if mappedFound {
				operator, found = NewSMTMapOperator(mapped), true
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown operator %q", name)
		}
		arguments := make([]Expr, len(argumentsRaw))
		for i, argRaw := range argumentsRaw {
			arguments[i], err = decodeIRExpression(argRaw, store)
			if err != nil {
				return nil, err
			}
		}
		expression, err = NewOperExpr(operator, tag, arguments...)
		if err != nil {
			return nil, err
		}
	case "LetInEx":
		bodyRaw, ok := object["body"]
		if !ok {
			return nil, missingJSONField("body")
		}
		body, e := decodeIRExpression(bodyRaw, store)
		if e != nil {
			return nil, e
		}
		var declarationsRaw []json.RawMessage
		if e = requiredJSON(object, "decls", &declarationsRaw); e != nil {
			return nil, e
		}
		declarations := make([]*OperDecl, len(declarationsRaw))
		for i, declRaw := range declarationsRaw {
			decl, e := decodeIRDeclaration(declRaw, store)
			if e != nil {
				return nil, e
			}
			op, ok := decl.(*OperDecl)
			if !ok {
				return nil, fmt.Errorf("LET-IN declaration %d is %T, expected operator", i, decl)
			}
			declarations[i] = op
		}
		expression = NewLetInExpr(body, tag, declarations...)
	default:
		return nil, fmt.Errorf("%s is not a valid TlaEx kind", kind)
	}
	if err = decodeIRSource(object, expression.ID(), store); err != nil {
		return nil, err
	}
	return expression, nil
}

func decodeIRValue(raw json.RawMessage) (Value, error) {
	object, err := rawObject(raw)
	if err != nil {
		return nil, err
	}
	var kind string
	if err = requiredJSON(object, "kind", &kind); err != nil {
		return nil, err
	}
	switch kind {
	case "TlaStr":
		var value string
		err = requiredJSON(object, "value", &value)
		return StringValue(value), err
	case "TlaBool":
		var value bool
		err = requiredJSON(object, "value", &value)
		return BoolValue(value), err
	case "TlaDecimal":
		var value string
		if err = requiredJSON(object, "value", &value); err != nil {
			return nil, err
		}
		return NewDecimalValue(value)
	case "TlaInt":
		valueRaw, ok := object["value"]
		if !ok {
			return nil, missingJSONField("value")
		}
		var wrapped map[string]string
		if json.Unmarshal(valueRaw, &wrapped) == nil && wrapped["bigInt"] != "" {
			integer, ok := new(big.Int).SetString(wrapped["bigInt"], 10)
			if !ok {
				return nil, fmt.Errorf("invalid big integer %q", wrapped["bigInt"])
			}
			return NewIntValue(integer), nil
		}
		var number json.Number
		if err = json.Unmarshal(valueRaw, &number); err != nil {
			return nil, err
		}
		integer, ok := new(big.Int).SetString(number.String(), 10)
		if !ok {
			return nil, fmt.Errorf("invalid integer %q", number)
		}
		return NewIntValue(integer), nil
	case "TlaBoolSet":
		return BoolSet, nil
	case "TlaIntSet":
		return IntSet, nil
	case "TlaStrSet":
		return StringSet, nil
	case "TlaNatSet":
		return NatSet, nil
	default:
		return nil, fmt.Errorf("%s is not a valid TlaValue kind", kind)
	}
}

func decodeKindAndTag(object map[string]json.RawMessage, requireTag bool) (string, TypeTag, error) {
	var kind string
	if err := requiredJSON(object, "kind", &kind); err != nil {
		return "", Untyped(), err
	}
	if !requireTag {
		return kind, Untyped(), nil
	}
	var text string
	if err := requiredJSON(object, "type", &text); err != nil {
		return "", Untyped(), err
	}
	if text == "Untyped" {
		return kind, Untyped(), nil
	}
	typ, err := ParseType(text)
	if err != nil {
		return "", Untyped(), fmt.Errorf("error in type annotation %q: %w", text, err)
	}
	return kind, Typed(typ), nil
}

func decodeIRSource(object map[string]json.RawMessage, id UID, store *SourceStore) error {
	if store == nil {
		return nil
	}
	raw, ok := object["source"]
	if !ok || bytes.Equal(raw, []byte(`"UNKNOWN"`)) {
		return nil
	}
	var source irJSONSource
	if err := json.Unmarshal(raw, &source); err != nil {
		return fmt.Errorf("invalid source location: %w", err)
	}
	return store.Add(id, SourceLocation{Filename: source.Filename, Region: NewSourceRegion(source.From.Line, source.From.Column, source.To.Line, source.To.Column)})
}
func rawObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, fmt.Errorf("expected JSON object")
	}
	return object, nil
}
func missingJSONField(name string) error { return fmt.Errorf("JSON has no field named [%s]", name) }
func requiredJSON(object map[string]json.RawMessage, name string, target any) error {
	raw, ok := object[name]
	if !ok {
		return missingJSONField(name)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("field %s: %w", name, err)
	}
	return nil
}
