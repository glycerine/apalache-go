package apalache

import (
	"fmt"
	"io"
	"math/big"
	"regexp"
	"sort"
	"strings"
)

type ConfigConstExpr interface{ configConstExpr() }
type ConfigModelValue struct{ Name string }

func (ConfigModelValue) configConstExpr() {}

type ConfigIntValue struct{ Value *big.Int }

func (ConfigIntValue) configConstExpr() {}

type ConfigBoolValue struct{ Value bool }

func (ConfigBoolValue) configConstExpr() {}

type ConfigStringValue struct{ Value string }

func (ConfigStringValue) configConstExpr() {}

type ConfigSetValue struct{ Elements []ConfigConstExpr }

func (ConfigSetValue) configConstExpr() {}

var configModelStringPattern = regexp.MustCompile(`^([a-zA-Z0-9_]+)_OF_([A-Z_][A-Z0-9_]*)$`)

// ConfigConstToExpr converts a parsed TLC constant into typed Apalache IR.
func ConfigConstToExpr(value ConfigConstExpr) (Expr, error) {
	switch typed := value.(type) {
	case ConfigModelValue:
		return NewValueExpr(StringValue("ModelValue_"+typed.Name), Typed(StrT1)), nil
	case ConfigIntValue:
		if typed.Value == nil {
			return nil, fmt.Errorf("nil TLC integer value")
		}
		return NewValueExpr(NewIntValue(typed.Value), Typed(IntT1)), nil
	case ConfigBoolValue:
		return NewValueExpr(BoolValue(typed.Value), Typed(BoolT1)), nil
	case ConfigStringValue:
		valueType := Type(StrT1)
		if match := configModelStringPattern.FindStringSubmatch(typed.Value); match != nil {
			constant, err := NewConstType(match[2])
			if err != nil {
				return nil, err
			}
			valueType = constant
		}
		return NewValueExpr(StringValue(typed.Value), Typed(valueType)), nil
	case ConfigSetValue:
		elements := make([]Expr, len(typed.Elements))
		var elementType Type
		if len(elements) == 0 {
			variable, _ := NewVarType(0)
			elementType = variable
		}
		for index, element := range typed.Elements {
			converted, err := ConfigConstToExpr(element)
			if err != nil {
				return nil, err
			}
			typ, _ := converted.Tag().Type()
			if index == 0 {
				elementType = typ
			} else if !EqualTypes(elementType, typ) {
				return nil, fmt.Errorf("set elements have different types: %s and %s", elementType, typ)
			}
			elements[index] = converted
		}
		setType, _ := NewSetType(elementType)
		return NewOperExpr(Op("SET_ENUM"), Typed(setType), elements...)
	default:
		return nil, fmt.Errorf("unsupported TLC constant expression %T", value)
	}
}

type BehaviorSpec interface{ behaviorSpec() }
type InitNextSpec struct{ Init, Next string }

func (InitNextSpec) behaviorSpec() {}
func (spec InitNextSpec) String() string {
	return fmt.Sprintf("InitNextSpec(%s,%s)", spec.Init, spec.Next)
}

type TemporalSpec struct{ Name string }

func (TemporalSpec) behaviorSpec()       {}
func (spec TemporalSpec) String() string { return fmt.Sprintf("TemporalSpec(%s)", spec.Name) }

// NullSpec represents an unspecified TLC behavior.
type NullSpec struct{}

func (NullSpec) behaviorSpec()  {}
func (NullSpec) String() string { return "NullSpec()" }

type TLCConfig struct {
	ConstAssignments      map[string]ConfigConstExpr
	ConstAssignmentOrder  []string
	ConstReplacements     map[string]string
	ConstReplacementOrder []string
	StateConstraints      []string
	ActionConstraints     []string
	Invariants            []string
	TemporalProperties    []string
	Behavior              BehaviorSpec
	CheckDeadlock         *bool
}

func EmptyTLCConfig() TLCConfig {
	return TLCConfig{
		ConstAssignments: map[string]ConfigConstExpr{}, ConstReplacements: map[string]string{},
		StateConstraints: []string{}, ActionConstraints: []string{}, Invariants: []string{},
		TemporalProperties: []string{}, Behavior: NullSpec{},
	}
}

func (config TLCConfig) AddConstAssignments(more map[string]ConfigConstExpr) TLCConfig {
	result := cloneTLCConfig(config)
	if result.ConstAssignments == nil {
		result.ConstAssignments = make(map[string]ConfigConstExpr)
	}
	keys := sortedConfigKeys(more)
	for _, key := range keys {
		if _, exists := result.ConstAssignments[key]; !exists {
			result.ConstAssignmentOrder = append(result.ConstAssignmentOrder, key)
		}
		result.ConstAssignments[key] = cloneConfigConstExpr(more[key])
	}
	return result
}

func (config TLCConfig) AddConstReplacements(more map[string]string) TLCConfig {
	result := cloneTLCConfig(config)
	if result.ConstReplacements == nil {
		result.ConstReplacements = make(map[string]string)
	}
	keys := sortedConfigKeys(more)
	for _, key := range keys {
		if _, exists := result.ConstReplacements[key]; !exists {
			result.ConstReplacementOrder = append(result.ConstReplacementOrder, key)
		}
		result.ConstReplacements[key] = more[key]
	}
	return result
}

func (config TLCConfig) AddStateConstraints(more []string) TLCConfig {
	result := cloneTLCConfig(config)
	result.StateConstraints = append(result.StateConstraints, more...)
	return result
}

func (config TLCConfig) AddActionConstraints(more []string) TLCConfig {
	result := cloneTLCConfig(config)
	result.ActionConstraints = append(result.ActionConstraints, more...)
	return result
}

func (config TLCConfig) AddInvariants(more []string) TLCConfig {
	result := cloneTLCConfig(config)
	result.Invariants = append(result.Invariants, more...)
	return result
}

func (config TLCConfig) AddTemporalProperties(more []string) TLCConfig {
	result := cloneTLCConfig(config)
	result.TemporalProperties = append(result.TemporalProperties, more...)
	return result
}

func (config TLCConfig) SetBehaviorSpecUnlessNull(spec BehaviorSpec) (TLCConfig, error) {
	result := cloneTLCConfig(config)
	if spec == nil {
		spec = NullSpec{}
	}
	if result.Behavior == nil {
		result.Behavior = NullSpec{}
	}
	_, currentNull := result.Behavior.(NullSpec)
	_, nextNull := spec.(NullSpec)
	if !currentNull && !nextNull {
		return TLCConfig{}, &TLCConfigParseError{Message: fmt.Sprintf("Found several behavior specifications: %v and %v", result.Behavior, spec)}
	}
	if !nextNull {
		result.Behavior = spec
	}
	return result, nil
}

func (config TLCConfig) SetCheckDeadlockUnlessUnset(setting *bool) (TLCConfig, error) {
	result := cloneTLCConfig(config)
	if result.CheckDeadlock != nil && setting != nil && *result.CheckDeadlock != *setting {
		return TLCConfig{}, &TLCConfigParseError{Message: fmt.Sprintf(
			"Found conflicting CHECK_DEADLOCK declarations: %t and %t", *result.CheckDeadlock, *setting)}
	}
	if result.CheckDeadlock == nil && setting != nil {
		result.CheckDeadlock = clonePointer(setting)
	}
	return result, nil
}

func (config TLCConfig) Join(other TLCConfig) (TLCConfig, error) {
	result := config.AddConstReplacementsInOrder(other.ConstReplacements, other.ConstReplacementOrder)
	result = result.AddConstAssignmentsInOrder(other.ConstAssignments, other.ConstAssignmentOrder)
	result = result.AddStateConstraints(other.StateConstraints)
	result = result.AddActionConstraints(other.ActionConstraints)
	result = result.AddInvariants(other.Invariants)
	result = result.AddTemporalProperties(other.TemporalProperties)
	var err error
	result, err = result.SetBehaviorSpecUnlessNull(other.Behavior)
	if err != nil {
		return TLCConfig{}, err
	}
	return result.SetCheckDeadlockUnlessUnset(other.CheckDeadlock)
}

func (config TLCConfig) AddConstAssignmentsInOrder(more map[string]ConfigConstExpr, preferred []string) TLCConfig {
	result := cloneTLCConfig(config)
	if result.ConstAssignments == nil {
		result.ConstAssignments = make(map[string]ConfigConstExpr)
	}
	for _, key := range orderedConfigKeys(more, preferred) {
		if _, exists := result.ConstAssignments[key]; !exists {
			result.ConstAssignmentOrder = append(result.ConstAssignmentOrder, key)
		}
		result.ConstAssignments[key] = cloneConfigConstExpr(more[key])
	}
	return result
}

func (config TLCConfig) AddConstReplacementsInOrder(more map[string]string, preferred []string) TLCConfig {
	result := cloneTLCConfig(config)
	if result.ConstReplacements == nil {
		result.ConstReplacements = make(map[string]string)
	}
	for _, key := range orderedConfigKeys(more, preferred) {
		if _, exists := result.ConstReplacements[key]; !exists {
			result.ConstReplacementOrder = append(result.ConstReplacementOrder, key)
		}
		result.ConstReplacements[key] = more[key]
	}
	return result
}

func sortedConfigKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type TLCConfigParseError struct {
	Line, Column int
	Message      string
}

func (err *TLCConfigParseError) Error() string {
	return fmt.Sprintf("Config parse error at %d.%d: %s", err.Line, err.Column, err.Message)
}

type tlcToken struct {
	kind, text   string
	line, column int
}

func (token tlcToken) String() string {
	switch token.kind {
	case "ident":
		return fmt.Sprintf("identifier '%s'", token.text)
	case "string":
		return fmt.Sprintf("string \"%s\"", token.text)
	case "number":
		return fmt.Sprintf("number '%s'", token.text)
	case "boolean":
		return fmt.Sprintf("boolean '%s'", token.text)
	case "eof":
		return "end of input"
	default:
		return token.text
	}
}

func ParseTLCConfig(text string) (TLCConfig, error) {
	tokens, err := lexTLCConfig(text)
	if err != nil {
		return TLCConfig{}, err
	}
	p := tlcConfigParser{tokens: tokens}
	return p.parse()
}

// ParseTLCConfigReader is the streaming-input entry point corresponding to
// Scala's TlcConfigParser.apply(Reader).
func ParseTLCConfigReader(reader io.Reader) (TLCConfig, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return TLCConfig{}, err
	}
	return ParseTLCConfig(string(data))
}

func lexTLCConfig(text string) ([]tlcToken, error) {
	var tokens []tlcToken
	line, column := 1, 1
	advance := func(ch byte) {
		if ch == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	for i := 0; i < len(text); {
		ch := text[i]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\f' || ch == '\n' {
			advance(ch)
			i++
			continue
		}
		if ch == '\\' && i+1 < len(text) && text[i+1] == '*' {
			for i < len(text) && text[i] != '\n' {
				advance(text[i])
				i++
			}
			continue
		}
		if ch == '(' && i+1 < len(text) && text[i+1] == '*' {
			startLine, startColumn := line, column
			advance(ch)
			advance('*')
			i += 2
			closed := false
			for i < len(text) {
				if text[i] == '*' && i+1 < len(text) && text[i+1] == ')' {
					advance('*')
					advance(')')
					i += 2
					closed = true
					break
				}
				advance(text[i])
				i++
			}
			if !closed {
				return nil, &TLCConfigParseError{startLine, startColumn, "unterminated comment"}
			}
			continue
		}
		startLine, startColumn := line, column
		if ch == '<' && i+1 < len(text) && text[i+1] == '-' {
			tokens = append(tokens, tlcToken{"<-", "<-", line, column})
			advance(ch)
			advance('-')
			i += 2
			continue
		}
		if ch == '=' || ch == '{' || ch == '}' || ch == ',' {
			tokens = append(tokens, tlcToken{string(ch), string(ch), line, column})
			advance(ch)
			i++
			continue
		}
		if ch == '"' {
			i++
			advance(ch)
			start := i
			for i < len(text) && text[i] != '"' && text[i] != '\n' {
				if !isTLCConfigStringByte(text[i]) {
					return nil, &TLCConfigParseError{startLine, startColumn, "unexpected character"}
				}
				i++
				advance(text[i-1])
			}
			if i >= len(text) || text[i] != '"' {
				return nil, &TLCConfigParseError{startLine, startColumn, "unterminated string"}
			}
			tokens = append(tokens, tlcToken{"string", text[start:i], startLine, startColumn})
			advance(text[i])
			i++
			continue
		}
		if ch == '-' || ch >= '0' && ch <= '9' {
			start := i
			if ch == '-' {
				advance(ch)
				i++
				if i >= len(text) || text[i] < '0' || text[i] > '9' {
					return nil, &TLCConfigParseError{startLine, startColumn, "expected digits after '-'"}
				}
			}
			for i < len(text) && text[i] >= '0' && text[i] <= '9' {
				advance(text[i])
				i++
			}
			tokens = append(tokens, tlcToken{"number", text[start:i], startLine, startColumn})
			continue
		}
		if ch == '_' || isASCIIAlpha(ch) {
			start := i
			for i < len(text) && (text[i] == '_' || isASCIIAlpha(text[i]) || isASCIIDigit(text[i])) {
				advance(text[i])
				i++
			}
			word := text[start:i]
			kind := "ident"
			if word == "TRUE" || word == "FALSE" {
				kind = "boolean"
			} else if canonical := tlcKeyword(word); canonical != "" {
				kind = canonical
			}
			tokens = append(tokens, tlcToken{kind, word, startLine, startColumn})
			continue
		}
		return nil, &TLCConfigParseError{line, column, fmt.Sprintf("unexpected character %q", ch)}
	}
	tokens = append(tokens, tlcToken{"eof", "", line, column})
	return tokens, nil
}

func isASCIIDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func isTLCConfigStringByte(ch byte) bool {
	if isASCIIAlpha(ch) || isASCIIDigit(ch) || ch == ' ' || ch == '_' {
		return true
	}
	return strings.ContainsRune("~!@#$%^&*\\-+=|(){}[],:;`'<>.?/", rune(ch))
}

func tlcKeyword(word string) string {
	switch word {
	case "CONSTANT", "CONSTANTS":
		return "CONSTANT"
	case "INIT", "NEXT", "SPECIFICATION", "SYMMETRY", "VIEW", "ALIAS", "POSTCONDITION", "CHECK_DEADLOCK":
		return word
	case "INVARIANT", "INVARIANTS":
		return "INVARIANT"
	case "PROPERTY", "PROPERTIES":
		return "PROPERTY"
	case "CONSTRAINT", "CONSTRAINTS":
		return "CONSTRAINT"
	case "ACTION_CONSTRAINT", "ACTION_CONSTRAINTS":
		return "ACTION_CONSTRAINT"
	}
	return ""
}

type tlcConfigParser struct {
	tokens []tlcToken
	at     int
	config TLCConfig
}

func (p *tlcConfigParser) current() tlcToken { return p.tokens[p.at] }
func (p *tlcConfigParser) take(kind string) (tlcToken, error) {
	token := p.current()
	if token.kind != kind {
		return token, p.fail(token, fmt.Sprintf("expected %s, found %s", kind, token.text))
	}
	p.at++
	return token, nil
}
func (p *tlcConfigParser) fail(token tlcToken, message string) error {
	return &TLCConfigParseError{token.line, token.column, message}
}
func (p *tlcConfigParser) parse() (TLCConfig, error) {
	p.config = EmptyTLCConfig()
	options := 0
	for p.current().kind != "eof" {
		options++
		if err := p.parseOption(); err != nil {
			return TLCConfig{}, err
		}
	}
	if options == 0 {
		return TLCConfig{}, p.fail(p.current(), "expected at least one configuration option")
	}
	return p.config, nil
}
func (p *tlcConfigParser) parseOption() error {
	switch p.current().kind {
	case "CONSTANT":
		return p.parseConstants()
	case "CONSTRAINT":
		return p.parseNames(&p.config.StateConstraints)
	case "ACTION_CONSTRAINT":
		return p.parseNames(&p.config.ActionConstraints)
	case "INVARIANT":
		return p.parseNames(&p.config.Invariants)
	case "PROPERTY":
		return p.parseNames(&p.config.TemporalProperties)
	case "INIT", "NEXT":
		return p.parseInitNext()
	case "SPECIFICATION":
		p.at++
		name, err := p.take("ident")
		if err != nil {
			return err
		}
		return p.setBehavior(TemporalSpec{Name: name.text}, name)
	case "CHECK_DEADLOCK":
		keyword := p.current()
		p.at++
		value, err := p.take("boolean")
		if err != nil {
			return err
		}
		flag := value.text == "TRUE"
		if p.config.CheckDeadlock != nil && *p.config.CheckDeadlock != flag {
			return p.fail(keyword, "found conflicting CHECK_DEADLOCK declarations")
		}
		p.config.CheckDeadlock = &flag
		return nil
	case "SYMMETRY", "VIEW", "ALIAS", "POSTCONDITION":
		p.at++
		_, err := p.take("ident")
		return err
	default:
		return p.fail(p.current(), fmt.Sprintf("unexpected token %s", p.current().text))
	}
}
func (p *tlcConfigParser) parseConstants() error {
	keyword := p.current()
	p.at++
	count := 0
	for p.current().kind == "ident" && p.at+1 < len(p.tokens) && (p.tokens[p.at+1].kind == "=" || p.tokens[p.at+1].kind == "<-") {
		count++
		name := p.current().text
		p.at++
		operator := p.current().kind
		p.at++
		if operator == "<-" {
			replacement, err := p.take("ident")
			if err != nil {
				return err
			}
			if _, exists := p.config.ConstReplacements[name]; !exists {
				p.config.ConstReplacementOrder = append(p.config.ConstReplacementOrder, name)
			}
			p.config.ConstReplacements[name] = replacement.text
		} else {
			value, err := p.parseConstExpr()
			if err != nil {
				return err
			}
			if _, exists := p.config.ConstAssignments[name]; !exists {
				p.config.ConstAssignmentOrder = append(p.config.ConstAssignmentOrder, name)
			}
			p.config.ConstAssignments[name] = value
		}
	}
	if count == 0 {
		return p.fail(keyword, "expected a constant binding `A <- B` or `A = B`")
	}
	return nil
}
func (p *tlcConfigParser) parseConstExpr() (ConfigConstExpr, error) {
	token := p.current()
	p.at++
	switch token.kind {
	case "ident":
		return ConfigModelValue{Name: token.text}, nil
	case "string":
		return ConfigStringValue{Value: token.text}, nil
	case "boolean":
		return ConfigBoolValue{Value: token.text == "TRUE"}, nil
	case "number":
		value, ok := new(big.Int).SetString(token.text, 10)
		if !ok {
			return nil, p.fail(token, "invalid integer")
		}
		return ConfigIntValue{Value: value}, nil
	case "{":
		var values []ConfigConstExpr
		if p.current().kind == "}" {
			p.at++
			return ConfigSetValue{Elements: values}, nil
		}
		for {
			value, err := p.parseConstExpr()
			if err != nil {
				return nil, err
			}
			values = append(values, value)
			if p.current().kind == "}" {
				p.at++
				return ConfigSetValue{Elements: values}, nil
			}
			if _, err = p.take(","); err != nil {
				return nil, err
			}
		}
	default:
		return nil, p.fail(token, "expected a constant expression")
	}
}
func (p *tlcConfigParser) parseNames(target *[]string) error {
	keyword := p.current()
	p.at++
	start := len(*target)
	for p.current().kind == "ident" {
		*target = append(*target, p.current().text)
		p.at++
	}
	if len(*target) == start {
		return p.fail(keyword, "expected at least one identifier")
	}
	return nil
}
func (p *tlcConfigParser) parseInitNext() error {
	first := p.current()
	p.at++
	firstName, err := p.take("ident")
	if err != nil {
		return err
	}
	expected := "NEXT"
	if first.kind == "NEXT" {
		expected = "INIT"
	}
	if _, err = p.take(expected); err != nil {
		return err
	}
	secondName, err := p.take("ident")
	if err != nil {
		return err
	}
	spec := InitNextSpec{Init: firstName.text, Next: secondName.text}
	if first.kind == "NEXT" {
		spec = InitNextSpec{Init: secondName.text, Next: firstName.text}
	}
	return p.setBehavior(spec, first)
}
func (p *tlcConfigParser) setBehavior(spec BehaviorSpec, token tlcToken) error {
	if _, empty := p.config.Behavior.(NullSpec); !empty {
		return p.fail(token, fmt.Sprintf("found several behavior specifications: %v and %v", p.config.Behavior, spec))
	}
	p.config.Behavior = spec
	return nil
}
