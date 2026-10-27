package apalache

import (
	"fmt"
	"math/big"
	"regexp"
	"unicode"
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

type TemporalSpec struct{ Name string }

func (TemporalSpec) behaviorSpec() {}

type TLCConfig struct {
	ConstAssignments   map[string]ConfigConstExpr
	ConstReplacements  map[string]string
	StateConstraints   []string
	ActionConstraints  []string
	Invariants         []string
	TemporalProperties []string
	Behavior           BehaviorSpec
	CheckDeadlock      *bool
}

type TLCConfigParseError struct {
	Line, Column int
	Message      string
}

func (err *TLCConfigParseError) Error() string {
	return fmt.Sprintf("config parse error at %d:%d: %s", err.Line, err.Column, err.Message)
}

type tlcToken struct {
	kind, text   string
	line, column int
}

func ParseTLCConfig(text string) (TLCConfig, error) {
	tokens, err := lexTLCConfig(text)
	if err != nil {
		return TLCConfig{}, err
	}
	p := tlcConfigParser{tokens: tokens}
	return p.parse()
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
		if unicode.IsSpace(rune(ch)) {
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
		if ch == '_' || unicode.IsLetter(rune(ch)) {
			start := i
			for i < len(text) && (text[i] == '_' || unicode.IsLetter(rune(text[i])) || unicode.IsDigit(rune(text[i]))) {
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
	p.config = TLCConfig{ConstAssignments: map[string]ConfigConstExpr{}, ConstReplacements: map[string]string{}}
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
			p.config.ConstReplacements[name] = replacement.text
		} else {
			value, err := p.parseConstExpr()
			if err != nil {
				return err
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
	if p.config.Behavior != nil {
		return p.fail(token, fmt.Sprintf("found several behavior specifications: %v and %v", p.config.Behavior, spec))
	}
	p.config.Behavior = spec
	return nil
}
