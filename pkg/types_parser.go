package apalache

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type TypeParseError struct {
	Offset  int
	Message string
}

func (err *TypeParseError) Error() string {
	return fmt.Sprintf("type parse error at offset %d: %s", err.Offset, err.Message)
}

type typeTokenKind uint8

const (
	typeTokenEOF typeTokenKind = iota
	typeTokenIdentifier
	typeTokenNumber
	typeTokenLParen
	typeTokenRParen
	typeTokenLRow
	typeTokenRRow
	typeTokenLSparse
	typeTokenRSparse
	typeTokenLAngle
	typeTokenRAngle
	typeTokenLBracket
	typeTokenRBracket
	typeTokenLCurly
	typeTokenRCurly
	typeTokenComma
	typeTokenColon
	typeTokenPipe
	typeTokenDollar
	typeTokenArrow
	typeTokenDoubleArrow
	typeTokenEqual
)

type typeToken struct {
	kind   typeTokenKind
	text   string
	offset int
}

func lexType(text string) ([]typeToken, error) {
	tokens := make([]typeToken, 0)
	for offset := 0; offset < len(text); {
		if text[offset] == '/' && offset+1 < len(text) && text[offset+1] == '/' {
			offset += 2
			for offset < len(text) && text[offset] != '\n' {
				offset++
			}
			continue
		}
		if unicode.IsSpace(rune(text[offset])) {
			offset++
			continue
		}
		start := offset
		matchedCompound := false
		for _, candidate := range []struct {
			text string
			kind typeTokenKind
		}{
			{"->", typeTokenArrow}, {"=>", typeTokenDoubleArrow}, {"(|", typeTokenLRow}, {"|)", typeTokenRRow},
			{"<|", typeTokenLSparse}, {"|>", typeTokenRSparse}, {"<<", typeTokenLAngle}, {">>", typeTokenRAngle},
		} {
			if strings.HasPrefix(text[offset:], candidate.text) {
				tokens = append(tokens, typeToken{kind: candidate.kind, text: candidate.text, offset: offset})
				offset += len(candidate.text)
				matchedCompound = true
				break
			}
		}
		if matchedCompound {
			continue
		}
		if isTypeIdentifierStart(text[offset]) {
			offset++
			for offset < len(text) && isTypeIdentifierPart(text[offset]) {
				offset++
			}
			tokens = append(tokens, typeToken{kind: typeTokenIdentifier, text: text[start:offset], offset: start})
			continue
		}
		if text[offset] >= '0' && text[offset] <= '9' {
			offset++
			for offset < len(text) && text[offset] >= '0' && text[offset] <= '9' {
				offset++
			}
			tokens = append(tokens, typeToken{kind: typeTokenNumber, text: text[start:offset], offset: start})
			continue
		}
		kind, ok := map[byte]typeTokenKind{
			'(': typeTokenLParen, ')': typeTokenRParen, '[': typeTokenLBracket, ']': typeTokenRBracket,
			'{': typeTokenLCurly, '}': typeTokenRCurly, ',': typeTokenComma, ':': typeTokenColon,
			'|': typeTokenPipe, '$': typeTokenDollar, '=': typeTokenEqual,
		}[text[offset]]
		if !ok {
			return nil, &TypeParseError{Offset: offset, Message: fmt.Sprintf("unexpected character %q", text[offset])}
		}
		tokens = append(tokens, typeToken{kind: kind, text: text[offset : offset+1], offset: offset})
		offset++
	}
	tokens = append(tokens, typeToken{kind: typeTokenEOF, offset: len(text)})
	return tokens, nil
}

func isTypeIdentifierStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}
func isTypeIdentifierPart(value byte) bool {
	return isTypeIdentifierStart(value) || value >= '0' && value <= '9'
}

type typeParser struct {
	tokens []typeToken
	index  int
}

func ParseType(text string) (Type, error) {
	tokens, err := lexType(text)
	if err != nil {
		return nil, err
	}
	parser := &typeParser{tokens: tokens}
	result, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if parser.peek().kind != typeTokenEOF {
		return nil, parser.errorf("unexpected token %q", parser.peek().text)
	}
	return result, nil
}

func ParseTypeAlias(text string) (string, Type, error) {
	tokens, err := lexType(text)
	if err != nil {
		return "", nil, err
	}
	parser := &typeParser{tokens: tokens}
	nameToken, err := parser.expect(typeTokenIdentifier, "alias name")
	if err != nil {
		return "", nil, err
	}
	name, ok := parseAliasName(nameToken.text)
	if !ok {
		return "", nil, parser.errorAt(nameToken, "expected an uppercase or camelCase alias name")
	}
	if _, err := parser.expect(typeTokenEqual, "="); err != nil {
		return "", nil, err
	}
	typ, err := parser.parseExpression()
	if err != nil {
		return "", nil, err
	}
	if parser.peek().kind != typeTokenEOF {
		return "", nil, parser.errorf("unexpected token %q", parser.peek().text)
	}
	return name, typ, nil
}

var camelCaseAlias = regexp.MustCompile(`^[a-z]+(?:[A-Z0-9][a-z0-9]*)*$`)

func parseAliasName(name string) (string, bool) {
	if strings.ToUpper(name) == name {
		if _, err := NewConstType(name); err == nil {
			return name, true
		}
	}
	if camelCaseAlias.MatchString(name) {
		return "$" + name, true
	}
	return "", false
}

func (parser *typeParser) parseExpression() (Type, error) {
	left, operatorArgs, err := parser.parseHead()
	if err != nil {
		return nil, err
	}
	if parser.accept(typeTokenDoubleArrow) {
		resultType, err := parser.parseExpression()
		if err != nil {
			return nil, err
		}
		args := operatorArgs
		if args == nil {
			args = []Type{left}
		}
		result, constructorErr := NewOperatorType(args, resultType)
		return result, constructorErr
	}
	if operatorArgs != nil {
		if len(operatorArgs) == 0 {
			return nil, parser.errorf("empty parenthesized type list must be followed by =>")
		}
		left, err = NewTupleType(operatorArgs...)
		if err != nil {
			return nil, err
		}
	}
	if parser.accept(typeTokenArrow) {
		resultType, err := parser.parseExpression()
		if err != nil {
			return nil, err
		}
		result, constructorErr := NewFunType(left, resultType)
		return result, constructorErr
	}
	return left, nil
}

// parseHead returns either a type or an argument list recognized inside
// parentheses. A non-nil argument list must be consumed by =>.
func (parser *typeParser) parseHead() (Type, []Type, error) {
	if parser.accept(typeTokenLParen) {
		if parser.accept(typeTokenRParen) {
			return nil, []Type{}, nil
		}
		first, err := parser.parseExpression()
		if err != nil {
			return nil, nil, err
		}
		if parser.accept(typeTokenComma) {
			arguments := []Type{first}
			for {
				argument, parseErr := parser.parseExpression()
				if parseErr != nil {
					return nil, nil, parseErr
				}
				arguments = append(arguments, argument)
				if !parser.accept(typeTokenComma) {
					break
				}
			}
			if _, err := parser.expect(typeTokenRParen, ")"); err != nil {
				return nil, nil, err
			}
			return nil, arguments, nil
		}
		if _, err := parser.expect(typeTokenRParen, ")"); err != nil {
			return nil, nil, err
		}
		return first, nil, nil
	}
	typ, err := parser.parseAtomic()
	return typ, nil, err
}

func (parser *typeParser) parseAtomic() (Type, error) {
	token := parser.peek()
	switch token.kind {
	case typeTokenIdentifier:
		parser.index++
		switch token.text {
		case "Int":
			return IntT1, nil
		case "Real":
			return RealT1, nil
		case "Bool":
			return BoolT1, nil
		case "Str":
			return StrT1, nil
		case "Set":
			return parser.parseUnaryTypeConstructor(func(element Type) (Type, error) { return NewSetType(element) })
		case "Seq":
			return parser.parseUnaryTypeConstructor(func(element Type) (Type, error) { return NewSeqType(element) })
		case "Variant":
			return parser.parseVariantConstructor()
		}
		if parser.peek().kind == typeTokenLParen {
			return parser.parseVariant(token)
		}
		if variable, err := ParseVarType(token.text); err == nil {
			return variable, nil
		}
		constant, err := NewConstType(token.text)
		if err == nil && strings.ToUpper(token.text) == token.text {
			return constant, nil
		}
		return nil, parser.errorAt(token, "expected a type variable, constant, or constructor")
	case typeTokenDollar:
		parser.index++
		name, err := parser.expect(typeTokenIdentifier, "alias name")
		if err != nil {
			return nil, err
		}
		if !camelCaseAlias.MatchString(name.text) {
			return nil, parser.errorAt(name, "invalid alias reference")
		}
		return NewConstType("$" + name.text)
	case typeTokenLAngle:
		return parser.parseTuple()
	case typeTokenLSparse:
		return parser.parseSparseTuple()
	case typeTokenLBracket:
		return parser.parseLegacyRecord()
	case typeTokenLRow:
		return parser.parseRow()
	case typeTokenLCurly:
		return parser.parseRowRecord()
	default:
		return nil, parser.errorf("expected a type, found %q", token.text)
	}
}

func (parser *typeParser) parseUnaryTypeConstructor(constructor func(Type) (Type, error)) (Type, error) {
	if _, err := parser.expect(typeTokenLParen, "("); err != nil {
		return nil, err
	}
	element, err := parser.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := parser.expect(typeTokenRParen, ")"); err != nil {
		return nil, err
	}
	return constructor(element)
}

func (parser *typeParser) parseTuple() (Type, error) {
	parser.index++
	if parser.peek().kind == typeTokenRAngle {
		return nil, parser.errorf("tuple type must contain at least one element")
	}
	elements := make([]Type, 0)
	for {
		element, err := parser.parseExpression()
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
		if !parser.accept(typeTokenComma) {
			break
		}
	}
	if _, err := parser.expect(typeTokenRAngle, ">>"); err != nil {
		return nil, err
	}
	return NewTupleType(elements...)
}

func (parser *typeParser) parseSparseTuple() (Type, error) {
	parser.index++
	fields := make(map[int]Type)
	if !parser.accept(typeTokenRSparse) {
		for {
			number, err := parser.expect(typeTokenNumber, "field number")
			if err != nil {
				return nil, err
			}
			index, conversionErr := strconv.Atoi(number.text)
			if conversionErr != nil {
				return nil, parser.errorAt(number, "field number is too large")
			}
			if _, err := parser.expect(typeTokenColon, ":"); err != nil {
				return nil, err
			}
			typ, err := parser.parseExpression()
			if err != nil {
				return nil, err
			}
			fields[index] = typ
			if !parser.accept(typeTokenComma) {
				break
			}
		}
		if _, err := parser.expect(typeTokenRSparse, "|>"); err != nil {
			return nil, err
		}
	}
	return NewSparseTupleType(fields)
}

func (parser *typeParser) parseLegacyRecord() (Type, error) {
	parser.index++
	fields := make(map[string]Type)
	if !parser.accept(typeTokenRBracket) {
		for {
			name, typ, err := parser.parseTypedField()
			if err != nil {
				return nil, err
			}
			fields[name] = typ
			if !parser.accept(typeTokenComma) {
				break
			}
		}
		if _, err := parser.expect(typeTokenRBracket, "]"); err != nil {
			return nil, err
		}
	}
	return NewRecordType(fields)
}

func (parser *typeParser) parseRow() (Type, error) {
	parser.index++
	if parser.accept(typeTokenRRow) {
		return NewRowType(nil, nil)
	}
	if parser.peek().kind == typeTokenIdentifier && parser.peekN(1).kind == typeTokenRRow {
		token := parser.peek()
		variable, err := ParseVarType(token.text)
		if err != nil {
			return nil, parser.errorAt(token, "expected a row variable")
		}
		parser.index += 2
		return NewRowType(nil, &variable)
	}

	fields := make(map[string]Type)
	var tail *VarType
	for {
		name, typ, err := parser.parseTypedField()
		if err != nil {
			return nil, err
		}
		fields[name] = typ
		if parser.accept(typeTokenRRow) {
			break
		}
		if _, err := parser.expect(typeTokenPipe, "| or |)"); err != nil {
			return nil, err
		}
		if parser.peek().kind == typeTokenIdentifier && parser.peekN(1).kind == typeTokenRRow {
			token := parser.peek()
			variable, parseErr := ParseVarType(token.text)
			if parseErr != nil {
				return nil, parser.errorAt(token, "expected a row variable")
			}
			tail = &variable
			parser.index += 2
			break
		}
	}
	return NewRowType(fields, tail)
}

func (parser *typeParser) parseRowRecord() (Type, error) {
	parser.index++
	fields := make(map[string]Type)
	var tail *VarType
	if parser.accept(typeTokenRCurly) {
		row, _ := NewRowType(fields, nil)
		return NewRecordRowType(row), nil
	}
	if parser.peek().kind == typeTokenIdentifier && parser.peekN(1).kind == typeTokenRCurly {
		token := parser.peek()
		variable, err := ParseVarType(token.text)
		if err != nil {
			return nil, parser.errorAt(token, "expected a row variable")
		}
		parser.index += 2
		row, _ := NewRowType(fields, &variable)
		return NewRecordRowType(row), nil
	}
	for {
		nameToken := parser.peek()
		name, typ, err := parser.parseTypedField()
		if err != nil {
			return nil, err
		}
		if _, duplicate := fields[name]; duplicate {
			return nil, parser.errorAt(nameToken, "duplicate record field "+name)
		}
		fields[name] = typ
		if parser.accept(typeTokenRCurly) {
			break
		}
		if _, err := parser.expect(typeTokenComma, ", or }"); err != nil {
			return nil, err
		}
		if parser.peek().kind == typeTokenIdentifier && parser.peekN(1).kind == typeTokenRCurly {
			token := parser.peek()
			variable, parseErr := ParseVarType(token.text)
			if parseErr != nil {
				return nil, parser.errorAt(token, "expected a row variable")
			}
			tail = &variable
			parser.index += 2
			break
		}
	}
	row, _ := NewRowType(fields, tail)
	return NewRecordRowType(row), nil
}

func (parser *typeParser) parseTypedField() (string, Type, error) {
	name, err := parser.expect(typeTokenIdentifier, "field name")
	if err != nil {
		return "", nil, err
	}
	if isReservedTypeWord(name.text) {
		return "", nil, parser.errorAt(name, "reserved type word cannot be a field name")
	}
	if _, err := parser.expect(typeTokenColon, ":"); err != nil {
		return "", nil, err
	}
	typ, err := parser.parseExpression()
	return name.text, typ, err
}

func (parser *typeParser) parseVariant(firstTag typeToken) (Type, error) {
	fields := make(map[string]Type)
	tag := firstTag
	var tail *VarType
	for {
		if _, duplicate := fields[tag.text]; duplicate {
			return nil, parser.errorAt(tag, "duplicate variant tag "+tag.text)
		}
		if _, err := parser.expect(typeTokenLParen, "("); err != nil {
			return nil, err
		}
		valueType, err := parser.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := parser.expect(typeTokenRParen, ")"); err != nil {
			return nil, err
		}
		fields[tag.text] = valueType
		// In a row, a pipe followed by "name:" separates row fields rather
		// than extending the variant that is the current field's value.
		if parser.peek().kind == typeTokenPipe && parser.peekN(1).kind == typeTokenIdentifier && parser.peekN(2).kind == typeTokenColon {
			break
		}
		if !parser.accept(typeTokenPipe) {
			break
		}
		next, err := parser.expect(typeTokenIdentifier, "variant tag or row variable")
		if err != nil {
			return nil, err
		}
		if parser.peek().kind == typeTokenLParen {
			tag = next
			continue
		}
		variable, parseErr := ParseVarType(next.text)
		if parseErr != nil {
			return nil, parser.errorAt(next, "expected a variant row variable")
		}
		tail = &variable
		break
	}
	row, _ := NewRowType(fields, tail)
	return NewVariantType(row), nil
}

func (parser *typeParser) parseVariantConstructor() (Type, error) {
	if _, err := parser.expect(typeTokenLParen, "("); err != nil {
		return nil, err
	}
	var tail *VarType
	if !parser.accept(typeTokenRParen) {
		token, err := parser.expect(typeTokenIdentifier, "variant row variable")
		if err != nil {
			return nil, err
		}
		variable, parseErr := ParseVarType(token.text)
		if parseErr != nil {
			return nil, parser.errorAt(token, "expected a variant row variable")
		}
		tail = &variable
		if _, err := parser.expect(typeTokenRParen, ")"); err != nil {
			return nil, err
		}
	}
	row, _ := NewRowType(nil, tail)
	return NewVariantType(row), nil
}

func isReservedTypeWord(text string) bool {
	switch text {
	case "Int", "Real", "Bool", "Str", "Set", "Seq", "Variant", "Rec":
		return true
	default:
		return false
	}
}

func (parser *typeParser) peek() typeToken { return parser.peekN(0) }
func (parser *typeParser) peekN(offset int) typeToken {
	index := parser.index + offset
	if index >= len(parser.tokens) {
		return parser.tokens[len(parser.tokens)-1]
	}
	return parser.tokens[index]
}
func (parser *typeParser) accept(kind typeTokenKind) bool {
	if parser.peek().kind != kind {
		return false
	}
	parser.index++
	return true
}
func (parser *typeParser) expect(kind typeTokenKind, expected string) (typeToken, error) {
	token := parser.peek()
	if token.kind != kind {
		return typeToken{}, parser.errorAt(token, "expected "+expected)
	}
	parser.index++
	return token, nil
}
func (parser *typeParser) errorf(format string, args ...any) error {
	return parser.errorAt(parser.peek(), fmt.Sprintf(format, args...))
}
func (*typeParser) errorAt(token typeToken, message string) error {
	return &TypeParseError{Offset: token.offset, Message: message}
}
