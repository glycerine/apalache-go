package apalache

import (
	"fmt"
	"math/big"
	"strings"
)

type AnnotationTokenKind string

const (
	AnnotationIdentifierToken   AnnotationTokenKind = "IDENT"
	AnnotationAtIdentifierToken AnnotationTokenKind = "AT_IDENT"
	AnnotationStringToken       AnnotationTokenKind = "STRING"
	AnnotationInlineStringToken AnnotationTokenKind = "INLINE_STRING"
	AnnotationNumberToken       AnnotationTokenKind = "NUMBER"
	AnnotationBooleanToken      AnnotationTokenKind = "BOOLEAN"
	AnnotationCommaToken        AnnotationTokenKind = "COMMA"
	AnnotationDotToken          AnnotationTokenKind = "DOT"
	AnnotationLeftParenToken    AnnotationTokenKind = "LPAREN"
	AnnotationRightParenToken   AnnotationTokenKind = "RPAREN"
	AnnotationNewlineToken      AnnotationTokenKind = "NL"
)

// AnnotationToken is the Go counterpart of Scala's annotation token case
// classes. Exactly one of Text, Number, or Boolean carries a value for token
// kinds that need one.
type AnnotationToken struct {
	Kind    AnnotationTokenKind
	Text    string
	Number  *big.Int
	Boolean bool
}

// LexAnnotation tokenizes one preprocessed annotation. It deliberately does
// not search through surrounding comments or prose; that is the comment
// preprocessor's responsibility.
func LexAnnotation(input string) ([]AnnotationToken, error) {
	tokens := make([]AnnotationToken, 0)
	for offset := 0; ; {
		for offset < len(input) && (input[offset] == ' ' || input[offset] == '\t' || input[offset] == '\f') {
			offset++
		}
		if offset == len(input) {
			if len(tokens) == 0 {
				return nil, &AnnotationParseError{Offset: offset, Message: "expected annotation token"}
			}
			return tokens, nil
		}
		start := offset
		switch input[offset] {
		case '(':
			tokens = append(tokens, AnnotationToken{Kind: AnnotationLeftParenToken})
			offset++
		case ')':
			tokens = append(tokens, AnnotationToken{Kind: AnnotationRightParenToken})
			offset++
		case ',':
			tokens = append(tokens, AnnotationToken{Kind: AnnotationCommaToken})
			offset++
		case '.':
			tokens = append(tokens, AnnotationToken{Kind: AnnotationDotToken})
			offset++
		case '\n':
			tokens = append(tokens, AnnotationToken{Kind: AnnotationNewlineToken})
			offset++
		case '\r':
			if offset+1 >= len(input) || input[offset+1] != '\n' {
				return nil, &AnnotationParseError{Offset: offset, Message: "unexpected character"}
			}
			tokens = append(tokens, AnnotationToken{Kind: AnnotationNewlineToken})
			offset += 2
		case '@':
			offset++
			nameStart := offset
			if offset >= len(input) || !annotationIdentifierStart(input[offset]) {
				return nil, &AnnotationParseError{Offset: start, Message: "unexpected character"}
			}
			offset++
			for offset < len(input) && annotationIdentifierPart(input[offset]) {
				offset++
			}
			tokens = append(tokens, AnnotationToken{Kind: AnnotationAtIdentifierToken, Text: input[nameStart:offset]})
		case '"':
			offset++
			textStart := offset
			for offset < len(input) && input[offset] != '"' && annotationStringChar(input[offset]) {
				offset++
			}
			if offset >= len(input) || input[offset] != '"' {
				return nil, &AnnotationParseError{Offset: offset, Message: "unterminated or invalid string"}
			}
			tokens = append(tokens, AnnotationToken{Kind: AnnotationStringToken, Text: input[textStart:offset]})
			offset++
		case ':':
			offset++
			textStart := offset
			for offset < len(input) && input[offset] != ';' && annotationInlineStringChar(input[offset]) {
				offset++
			}
			if offset >= len(input) || input[offset] != ';' {
				return nil, &AnnotationParseError{Offset: offset, Message: "unterminated or invalid inline string"}
			}
			text := strings.TrimSpace(input[textStart:offset])
			text = annotationInlineSpace.ReplaceAllString(text, " ")
			tokens = append(tokens, AnnotationToken{Kind: AnnotationInlineStringToken, Text: text})
			offset++
		default:
			if input[offset] == '-' || input[offset] >= '0' && input[offset] <= '9' {
				if input[offset] == '-' {
					offset++
				}
				digits := offset
				for offset < len(input) && input[offset] >= '0' && input[offset] <= '9' {
					offset++
				}
				if digits == offset {
					return nil, &AnnotationParseError{Offset: start, Message: "unexpected character"}
				}
				number, ok := new(big.Int).SetString(input[start:offset], 10)
				if !ok {
					return nil, fmt.Errorf("invalid annotation integer %q", input[start:offset])
				}
				tokens = append(tokens, AnnotationToken{Kind: AnnotationNumberToken, Number: number})
			} else if annotationIdentifierStart(input[offset]) {
				offset++
				for offset < len(input) && annotationIdentifierPart(input[offset]) {
					offset++
				}
				word := input[start:offset]
				switch word {
				case "TRUE", "true", "True":
					tokens = append(tokens, AnnotationToken{Kind: AnnotationBooleanToken, Boolean: true})
				case "FALSE", "false", "False":
					tokens = append(tokens, AnnotationToken{Kind: AnnotationBooleanToken})
				default:
					tokens = append(tokens, AnnotationToken{Kind: AnnotationIdentifierToken, Text: word})
				}
			} else {
				return nil, &AnnotationParseError{Offset: offset, Message: "unexpected character; missing ')' or ';'?"}
			}
		}
	}
}

func annotationIdentifierStart(ch byte) bool { return ch == '_' || isASCIIAlpha(ch) }
func annotationIdentifierPart(ch byte) bool {
	return annotationIdentifierStart(ch) || ch >= '0' && ch <= '9'
}
func annotationStringChar(ch byte) bool {
	return annotationIdentifierPart(ch) || strings.ContainsRune("~!@#$%^&*-+=|(){}[],:;`'<>.?/\\ \t\r\f\n", rune(ch))
}
func annotationInlineStringChar(ch byte) bool {
	return annotationIdentifierPart(ch) || strings.ContainsRune("~!#$%^&*-+=|(){}[],:`'<>.?/\\ \t\r\f\n", rune(ch))
}
