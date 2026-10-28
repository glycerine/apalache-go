package apalache

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

type AnnotationStore struct {
	mu      sync.RWMutex
	entries map[UID][]Annotation
}

func NewAnnotationStore() *AnnotationStore {
	return &AnnotationStore{entries: make(map[UID][]Annotation)}
}
func (store *AnnotationStore) Set(id UID, annotations []Annotation) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.entries[id] = cloneAnnotations(annotations)
}
func (store *AnnotationStore) Get(id UID) []Annotation {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return cloneAnnotations(store.entries[id])
}

func cloneAnnotations(annotations []Annotation) []Annotation {
	result := make([]Annotation, len(annotations))
	for index, annotation := range annotations {
		result[index] = Annotation{Name: annotation.Name, Arguments: make([]AnnotationArg, len(annotation.Arguments))}
		for argumentIndex, argument := range annotation.Arguments {
			switch typed := argument.(type) {
			case AnnotationInteger:
				if typed.Value != nil {
					result[index].Arguments[argumentIndex] = AnnotationInteger{Value: new(big.Int).Set(typed.Value)}
				} else {
					result[index].Arguments[argumentIndex] = typed
				}
			default:
				result[index].Arguments[argumentIndex] = argument
			}
		}
	}
	return result
}
func (store *AnnotationStore) Find(id UID, name string) (Annotation, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, annotation := range store.entries[id] {
		if annotation.Name == name {
			return cloneAnnotations([]Annotation{annotation})[0], true
		}
	}
	return Annotation{}, false
}

type AnnotationArg interface {
	fmt.Stringer
	annotationArg()
}
type AnnotationString struct{ Text string }

func (AnnotationString) annotationArg()   {}
func (a AnnotationString) String() string { return `"` + a.Text + `"` }

type AnnotationIdentifier struct{ Name string }

func (AnnotationIdentifier) annotationArg()   {}
func (a AnnotationIdentifier) String() string { return a.Name }

type AnnotationInteger struct{ Value *big.Int }

func (AnnotationInteger) annotationArg() {}
func (a AnnotationInteger) String() string {
	if a.Value == nil {
		return "0"
	}
	return a.Value.String()
}

type AnnotationBoolean struct{ Value bool }

func (AnnotationBoolean) annotationArg() {}
func (a AnnotationBoolean) String() string {
	if a.Value {
		return "true"
	}
	return "false"
}

type Annotation struct {
	Name      string
	Arguments []AnnotationArg
}

const (
	StandardAnnotationType      = "type"
	StandardAnnotationTypeAlias = "typeAlias"
)

func NewTypeAnnotation(typeText string) Annotation {
	return Annotation{Name: StandardAnnotationType, Arguments: []AnnotationArg{AnnotationString{Text: typeText}}}
}

func NewTypeAliasAnnotation(typeText string) Annotation {
	return Annotation{Name: StandardAnnotationTypeAlias, Arguments: []AnnotationArg{AnnotationString{Text: typeText}}}
}

// AnnotationStoreProvider owns the singleton annotation store used by one
// explicitly constructed application graph.
type AnnotationStoreProvider struct{ store *AnnotationStore }

func NewAnnotationStoreProvider() *AnnotationStoreProvider {
	return &AnnotationStoreProvider{store: NewAnnotationStore()}
}

func (provider *AnnotationStoreProvider) Get() *AnnotationStore {
	if provider == nil {
		return nil
	}
	return provider.store
}

func (annotation Annotation) String() string {
	if len(annotation.Arguments) == 0 {
		return "@" + annotation.Name
	}
	parts := make([]string, len(annotation.Arguments))
	for i, arg := range annotation.Arguments {
		parts[i] = arg.String()
	}
	return "@" + annotation.Name + "(" + strings.Join(parts, ", ") + ")"
}
func (annotation Annotation) PrettyString() string {
	if len(annotation.Arguments) == 1 {
		if text, ok := annotation.Arguments[0].(AnnotationString); ok {
			return fmt.Sprintf("@%s: %s;", annotation.Name, text.Text)
		}
	}
	return annotation.String()
}

type AnnotationParseError struct {
	Offset  int
	Message string
}

func (err *AnnotationParseError) Error() string {
	return fmt.Sprintf("annotation parse error at offset %d: %s", err.Offset, err.Message)
}

type annotationParser struct {
	text string
	at   int
}

func ParseAnnotation(text string) (Annotation, error) {
	tokens, err := LexAnnotation(text)
	if err != nil {
		return Annotation{}, err
	}
	return parseAnnotationTokens(tokens)
}

func parseAnnotationTokens(tokens []AnnotationToken) (Annotation, error) {
	at := 0
	fail := func(message string) (Annotation, error) {
		return Annotation{}, &AnnotationParseError{Offset: at, Message: message}
	}
	if len(tokens) == 0 || tokens[0].Kind != AnnotationAtIdentifierToken {
		return fail("expected @identifier")
	}
	result := Annotation{Name: tokens[0].Text}
	at = 1
	if at == len(tokens) {
		return result, nil
	}
	if tokens[at].Kind == AnnotationInlineStringToken {
		result.Arguments = []AnnotationArg{AnnotationString{Text: tokens[at].Text}}
		at++
		if at != len(tokens) {
			return fail("unexpected trailing input")
		}
		return result, nil
	}
	if tokens[at].Kind != AnnotationLeftParenToken {
		return fail("expected annotation arguments")
	}
	at++
	if at < len(tokens) && tokens[at].Kind == AnnotationRightParenToken {
		at++
		if at != len(tokens) {
			return fail("unexpected trailing input")
		}
		return result, nil
	}
	for {
		if at >= len(tokens) {
			return fail("expected argument")
		}
		token := tokens[at]
		var argument AnnotationArg
		switch token.Kind {
		case AnnotationStringToken:
			argument = AnnotationString{Text: token.Text}
		case AnnotationIdentifierToken:
			argument = AnnotationIdentifier{Name: token.Text}
		case AnnotationNumberToken:
			argument = AnnotationInteger{Value: new(big.Int).Set(token.Number)}
		case AnnotationBooleanToken:
			argument = AnnotationBoolean{Value: token.Boolean}
		default:
			return fail("expected string, integer, Boolean, or identifier")
		}
		result.Arguments = append(result.Arguments, argument)
		at++
		if at < len(tokens) && tokens[at].Kind == AnnotationRightParenToken {
			at++
			break
		}
		for at < len(tokens) && tokens[at].Kind == AnnotationNewlineToken {
			at++
		}
		if at >= len(tokens) || tokens[at].Kind != AnnotationCommaToken {
			return fail("expected ',' or ')'")
		}
		at++
		for at < len(tokens) && tokens[at].Kind == AnnotationNewlineToken {
			at++
		}
	}
	if at != len(tokens) {
		return fail("unexpected trailing input")
	}
	return result, nil
}

func parseAnnotationDirect(text string) (Annotation, error) {
	p := annotationParser{text: text}
	p.space(false)
	if !p.consume('@') {
		return Annotation{}, p.fail("expected @identifier")
	}
	name, ok := p.identifier()
	if !ok {
		return Annotation{}, p.fail("expected annotation name")
	}
	result := Annotation{Name: name}
	p.space(false)
	if p.at == len(p.text) {
		return result, nil
	}
	switch p.text[p.at] {
	case '(':
		p.at++
		p.space(false)
		if p.consume(')') {
			p.space(false)
			if p.at != len(p.text) {
				return Annotation{}, p.fail("unexpected trailing input")
			}
			return result, nil
		}
		for {
			argument, err := p.argument()
			if err != nil {
				return Annotation{}, err
			}
			result.Arguments = append(result.Arguments, argument)
			p.space(false)
			if p.consume(')') {
				break
			}
			p.newlines()
			p.space(false)
			if !p.consume(',') {
				return Annotation{}, p.fail("expected ',' or ')'")
			}
			p.space(true)
		}
	case ':':
		p.at++
		start := p.at
		semicolon := strings.IndexByte(p.text[p.at:], ';')
		if semicolon < 0 {
			return Annotation{}, p.fail("expected ';'")
		}
		end := p.at + semicolon
		inline := p.text[start:end]
		p.at = end + 1
		inline = strings.TrimSpace(annotationInlineSpace.ReplaceAllString(inline, " "))
		result.Arguments = []AnnotationArg{AnnotationString{Text: inline}}
	default:
		return Annotation{}, p.fail("expected annotation arguments")
	}
	p.space(false)
	if p.at != len(p.text) {
		return Annotation{}, p.fail("unexpected trailing input")
	}
	return result, nil
}

var annotationInlineSpace = regexp.MustCompile(`[ \t\f]+`)

func (p *annotationParser) argument() (AnnotationArg, error) {
	p.space(false)
	if p.at >= len(p.text) {
		return nil, p.fail("expected argument")
	}
	if p.text[p.at] == '"' {
		p.at++
		start := p.at
		for p.at < len(p.text) && p.text[p.at] != '"' {
			p.at++
		}
		if p.at >= len(p.text) {
			return nil, p.fail("unterminated string")
		}
		value := p.text[start:p.at]
		p.at++
		return AnnotationString{Text: value}, nil
	}
	start := p.at
	if p.text[p.at] == '-' {
		p.at++
	}
	for p.at < len(p.text) && p.text[p.at] >= '0' && p.text[p.at] <= '9' {
		p.at++
	}
	if p.at > start && (p.text[start] != '-' || p.at > start+1) {
		number, ok := new(big.Int).SetString(p.text[start:p.at], 10)
		if ok {
			return AnnotationInteger{Value: number}, nil
		}
	}
	p.at = start
	word, ok := p.identifier()
	if !ok {
		return nil, p.fail("expected string, integer, Boolean, or identifier")
	}
	switch strings.ToLower(word) {
	case "true":
		return AnnotationBoolean{Value: true}, nil
	case "false":
		return AnnotationBoolean{Value: false}, nil
	}
	return AnnotationIdentifier{Name: word}, nil
}

func (p *annotationParser) newlines() {
	for p.at < len(p.text) {
		if p.text[p.at] == '\n' {
			p.at++
			continue
		}
		if p.text[p.at] == '\r' && p.at+1 < len(p.text) && p.text[p.at+1] == '\n' {
			p.at += 2
			continue
		}
		return
	}
}
func (p *annotationParser) identifier() (string, bool) {
	if p.at >= len(p.text) || !(p.text[p.at] == '_' || isASCIIAlpha(p.text[p.at])) {
		return "", false
	}
	start := p.at
	p.at++
	for p.at < len(p.text) && (p.text[p.at] == '_' || isASCIIAlpha(p.text[p.at]) || p.text[p.at] >= '0' && p.text[p.at] <= '9') {
		p.at++
	}
	return p.text[start:p.at], true
}
func isASCIIAlpha(ch byte) bool { return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' }
func (p *annotationParser) space(includeNewlines bool) {
	for p.at < len(p.text) {
		r := rune(p.text[p.at])
		if p.text[p.at] == '\n' || p.text[p.at] == '\r' {
			if !includeNewlines {
				return
			}
			p.at++
			continue
		}
		if unicode.IsSpace(r) {
			p.at++
			continue
		}
		return
	}
}
func (p *annotationParser) consume(ch byte) bool {
	if p.at < len(p.text) && p.text[p.at] == ch {
		p.at++
		return true
	}
	return false
}
func (p *annotationParser) fail(message string) error {
	return &AnnotationParseError{Offset: p.at, Message: message}
}
