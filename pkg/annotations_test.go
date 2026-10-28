package apalache

import (
	"errors"
	"math/big"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestParseAnnotationForms(t *testing.T) {
	parsed, err := ParseAnnotation(`  @greet("hello", 2021, true, Name)   `)
	if err != nil {
		t.Fatal(err)
	}
	want := Annotation{Name: "greet", Arguments: []AnnotationArg{
		AnnotationString{Text: "hello"}, AnnotationInteger{Value: big.NewInt(2021)}, AnnotationBoolean{Value: true}, AnnotationIdentifier{Name: "Name"},
	}}
	if !reflect.DeepEqual(parsed, want) {
		t.Fatalf("got %#v, want %#v", parsed, want)
	}
	if parsed.String() != `@greet("hello", 2021, true, Name)` {
		t.Fatalf("String = %q", parsed.String())
	}

	inline, err := ParseAnnotation("@type:  (Int, Int) \t =>\f Set(Int) ;")
	if err != nil {
		t.Fatal(err)
	}
	if inline.PrettyString() != "@type: (Int, Int) => Set(Int);" {
		t.Fatalf("pretty = %q", inline.PrettyString())
	}
	parameterless, err := ParseAnnotation("@pure")
	if err != nil || parameterless.String() != "@pure" {
		t.Fatalf("parameterless = %#v, %v", parameterless, err)
	}
}

func TestParseAnnotationMultilineAndBigInteger(t *testing.T) {
	parsed, err := ParseAnnotation("@greet(\"hello\",\n 123456789012345678901234567890,\n FALSE)")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Arguments[1].(AnnotationInteger).Value.String() != "123456789012345678901234567890" || parsed.Arguments[2].(AnnotationBoolean).Value {
		t.Fatalf("arguments = %#v", parsed.Arguments)
	}
	alias, err := ParseAnnotation("@typeAlias: MESSAGE =\n  Req({ ask: Int })\n  | Ack({ success: Bool });")
	if err != nil {
		t.Fatal(err)
	}
	if alias.Arguments[0].(AnnotationString).Text != "MESSAGE =\n Req({ ask: Int })\n | Ack({ success: Bool })" {
		t.Fatalf("inline multiline = %q", alias.Arguments[0])
	}
}

func TestParseAnnotationRejectsJunkAndMalformedInput(t *testing.T) {
	cases := []string{"", "plain", `@foo("unterminated)`, "@foo(1,)", "@foo: missing terminator", "@foo() trailing", "@foo(@bad)"}
	for _, input := range cases {
		_, err := ParseAnnotation(input)
		var parseError *AnnotationParseError
		if !errors.As(err, &parseError) {
			t.Errorf("%q: got %T %v", input, err, err)
		}
	}
}

func TestAnnotationRendering(t *testing.T) {
	annotation := Annotation{Name: "sweet", Arguments: []AnnotationArg{AnnotationBoolean{Value: false}, AnnotationInteger{Value: big.NewInt(-2)}}}
	if annotation.String() != "@sweet(false, -2)" || annotation.PrettyString() != annotation.String() {
		t.Fatalf("rendered %q / %q", annotation.String(), annotation.PrettyString())
	}
}

func TestAnnotationArgumentRenderingMatchesScalaInterpolation(t *testing.T) {
	if got := (AnnotationString{Text: "a\\b\n\"c"}).String(); got != "\"a\\b\n\"c\"" {
		t.Fatalf("string argument = %q", got)
	}
	if got := (AnnotationIdentifier{Name: "Alias"}).String(); got != "Alias" {
		t.Fatalf("identifier argument = %q", got)
	}
	if got := (AnnotationInteger{Value: nil}).String(); got != "0" {
		t.Fatalf("nil integer argument = %q", got)
	}
	if got := (AnnotationBoolean{Value: false}).String(); got != "false" {
		t.Fatalf("Boolean argument = %q", got)
	}
}

func TestStandardAnnotationsAndStoreProvider(t *testing.T) {
	if got := NewTypeAnnotation("Int"); got.PrettyString() != "@type: Int;" {
		t.Fatalf("type annotation = %q", got.PrettyString())
	}
	if got := NewTypeAliasAnnotation("ITEM = Int"); got.PrettyString() != "@typeAlias: ITEM = Int;" {
		t.Fatalf("type alias annotation = %q", got.PrettyString())
	}
	firstProvider := NewAnnotationStoreProvider()
	secondProvider := NewAnnotationStoreProvider()
	first := firstProvider.Get()
	if first == nil || first != firstProvider.Get() || first == secondProvider.Get() {
		t.Fatalf("provider stores are not singleton-per-provider: %p %p %p", first, firstProvider.Get(), secondProvider.Get())
	}
	id := NewUID()
	first.Set(id, []Annotation{NewTypeAnnotation("Bool")})
	if annotation, found := firstProvider.Get().Find(id, StandardAnnotationType); !found || annotation.PrettyString() != "@type: Bool;" {
		t.Fatalf("stored standard annotation = %v, %v", annotation, found)
	}
}

func TestParseAnnotationMatchesScalaRegressionCases(t *testing.T) {
	variant := "@typeAlias: MESSAGE =\n  Req({ ask: Int })\n  | Ack({ success: Bool });"
	parsed, err := ParseAnnotation(variant)
	if err != nil {
		t.Fatal(err)
	}
	want := "MESSAGE =\n Req({ ask: Int })\n | Ack({ success: Bool })"
	if parsed.Name != "typeAlias" || parsed.Arguments[0].(AnnotationString).Text != want {
		t.Fatalf("variant alias = %#v", parsed)
	}
	for _, input := range []string{
		"\\* junk @greet(\"hello\", 2021, true) trailing",
		"\n  @type: ACTION\n",
		"@require(ConstInit)\n@ensure(Init)",
	} {
		if parsed, parseErr := ParseAnnotation(input); parseErr == nil {
			t.Errorf("accepted regression input %q as %#v", input, parsed)
		}
	}
}

func TestParseAnnotationGeneratedGoodInputs(t *testing.T) {
	random := rand.New(rand.NewSource(0xa6607))
	for iteration := 0; iteration < 200; iteration++ {
		name := generatedAnnotationWord(random, 1+random.Intn(12))
		argumentCount := random.Intn(8)
		var arguments []AnnotationArg
		if argumentCount > 0 {
			arguments = make([]AnnotationArg, argumentCount)
		}
		for index := range arguments {
			switch random.Intn(4) {
			case 0:
				arguments[index] = AnnotationString{Text: generatedAnnotationAlphaNumeric(random, random.Intn(20))}
			case 1:
				arguments[index] = AnnotationInteger{Value: big.NewInt(int64(random.Int31()) - int64(1<<30))}
			case 2:
				arguments[index] = AnnotationBoolean{Value: random.Intn(2) == 0}
			default:
				arguments[index] = AnnotationIdentifier{Name: generatedAnnotationWord(random, 1+random.Intn(12))}
			}
		}
		annotation := Annotation{Name: name, Arguments: arguments}
		parsed, err := ParseAnnotation(annotation.String())
		if err != nil || !reflect.DeepEqual(parsed, annotation) {
			t.Fatalf("iteration %d parse(%q) = %#v, %v; want %#v", iteration, annotation.String(), parsed, err, annotation)
		}
	}
}

func generatedAnnotationWord(random *rand.Rand, length int) string {
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_"
	continuation := letters + "0123456789"
	var result strings.Builder
	result.WriteByte(letters[random.Intn(len(letters))])
	for index := 1; index < length; index++ {
		result.WriteByte(continuation[random.Intn(len(continuation))])
	}
	return result.String()
}

func generatedAnnotationAlphaNumeric(random *rand.Rand, length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buffer := make([]byte, length)
	for index := range buffer {
		buffer[index] = alphabet[random.Intn(len(alphabet))]
	}
	return string(buffer)
}

func TestParseAnnotationGeneratedBadInputsDoNotPanic(t *testing.T) {
	for index := 0; index < 300; index++ {
		_, _ = ParseAnnotation(strconv.Itoa(index) + "not-an-annotation")
	}
}
