package apalache

import (
	"math/rand"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
)

func TestPreprocessAnnotationCommentsScalaEdgeCases(t *testing.T) {
	for _, input := range []string{"", "\\*", "(* aaa *)"} {
		clean, annotations := PreprocessAnnotationComments(input)
		withoutComments := strings.TrimSpace(regexp.MustCompile(`(\\\*|\(\*|\*\))`).ReplaceAllString(input, ""))
		if (withoutComments != "") != (strings.TrimSpace(clean) != "" || len(annotations) != 0) {
			t.Fatalf("input %q became clean=%q annotations=%q", input, clean, annotations)
		}
	}
	clean, annotations := PreprocessAnnotationComments(`" "`)
	if strings.TrimSpace(clean) != `" "` || len(annotations) != 0 {
		t.Fatalf("quotes changed: clean=%q annotations=%q", clean, annotations)
	}
	for _, input := range []string{"(*\n@type: Int;\n", "\\*@type: Int;"} {
		clean, annotations = PreprocessAnnotationComments(input)
		if strings.TrimSpace(clean) != "" || !reflect.DeepEqual(annotations, []string{"@type: Int;"}) {
			t.Fatalf("start annotation %q = clean %q annotations %q", input, clean, annotations)
		}
	}
	alias := "@typeAlias: MESSAGE = { tag: \"req\", ask: Int }\n | { tag: \"ack\", success: Bool };"
	_, annotations = PreprocessAnnotationComments("(*\n  " + alias + "\n *)")
	if !reflect.DeepEqual(annotations, []string{alias}) {
		t.Fatalf("pipe alias annotations = %q", annotations)
	}
}

func TestPreprocessAnnotationCommentsIsTotalOnGeneratedASCII(t *testing.T) {
	random := rand.New(rand.NewSource(0xc0ffee))
	commentTokens := regexp.MustCompile(`(\\\*|\(\*|\*\))`)
	for iteration := 0; iteration < 300; iteration++ {
		buffer := make([]byte, random.Intn(200))
		for index := range buffer {
			buffer[index] = byte(random.Intn(128))
		}
		input := string(buffer)
		clean, annotations := PreprocessAnnotationComments(input)
		withoutComments := strings.TrimSpace(commentTokens.ReplaceAllString(input, ""))
		if (withoutComments != "") != (strings.TrimSpace(clean) != "" || len(annotations) != 0) {
			t.Fatalf("iteration %d lost content: input=%q clean=%q annotations=%q", iteration, input, clean, annotations)
		}
	}
}

func TestPreprocessAnnotationCommentsRemovesDelimitersAndNestedComments(t *testing.T) {
	input := "(* T-Rex\n   D-Rex*)\n   (* zz.*)\n (* \\* uuuuvvv*)abc\n(*\n  aaaa (* bbbb *) cccc\n*)\n"
	want := " T-Rex\n   D-Rex\n    zz.\n  abc\n\n  aaaa  cccc\n\n"
	clean, annotations := PreprocessAnnotationComments(input)
	if clean != want || len(annotations) != 0 {
		t.Fatalf("clean:\n%q\nwant:\n%q\nannotations=%v", clean, want, annotations)
	}
}

func TestPreprocessAnnotationCommentsExtractsCandidates(t *testing.T) {
	input := "\\* @annotation(\"a\", 1) (aaa)\n" +
		"\\* not an annotation: john@example.org\n" +
		"\\* @semi: foo ; xxx\n" +
		"\\* @multi: aaa\n\\* bbb\n\\* ccc;ddd\n" +
		"\\* zzz@bbb(1)\n"
	wantClean := "  (aaa)\n not an annotation: john@example.org\n  xxx\n ddd\n zzz@bbb(1)\n"
	wantAnnotations := []string{"@annotation(\"a\", 1)", "@semi: foo ;", "@multi: aaa\n bbb\n ccc;"}
	clean, annotations := PreprocessAnnotationComments(input)
	if clean != wantClean || !reflect.DeepEqual(annotations, wantAnnotations) {
		t.Fatalf("clean=%q\nannotations=%q", clean, annotations)
	}
	for _, candidate := range annotations {
		if _, err := ParseAnnotation(candidate); err != nil {
			t.Errorf("candidate %q: %v", candidate, err)
		}
	}
}

func TestPreprocessAnnotationCommentsHandlesStringsAndStarts(t *testing.T) {
	input := "\\* @adventurous(\"@IgnoreMe(false, 42)\", 1) (aaa)\n" +
		"\\* @beware: of :) ;\n" +
		"\\* type annotation @type: Set(Set(a)) ;\n" +
		"\\* @type: a\n\\*          => Int\n\\* ;\n"
	clean, annotations := PreprocessAnnotationComments(input)
	want := []string{"@adventurous(\"@IgnoreMe(false, 42)\", 1)", "@beware: of :) ;", "@type: Set(Set(a)) ;", "@type: a\n          => Int\n ;"}
	if stringsTrimSpace(clean) != "(aaa)\n \n type annotation" || !reflect.DeepEqual(annotations, want) {
		t.Fatalf("clean=%q annotations=%q", clean, annotations)
	}
	_, atStart := PreprocessAnnotationComments("\\*@type: Int;")
	if !reflect.DeepEqual(atStart, []string{"@type: Int;"}) {
		t.Fatalf("start annotations=%v", atStart)
	}
}

func TestPreprocessAnnotationCommentsReturnsUnclosedCandidate(t *testing.T) {
	clean, annotations := PreprocessAnnotationComments("\\* xx @annotation(\"a\",\n(* aaa @semi: bar*)\n")
	if clean != " xx : bar\n" || !reflect.DeepEqual(annotations, []string{"@annotation(\"a\",\n aaa @semi"}) {
		t.Fatalf("clean=%q annotations=%q", clean, annotations)
	}
	if _, err := ParseAnnotation(annotations[0]); err == nil {
		t.Fatal("unclosed candidate parsed successfully")
	}
}

func TestFindAnnotationCandidatesPreservesSourcePositions(t *testing.T) {
	input := "---- MODULE A ----\nValue == \"@type\"\n  (* @typeAlias: ITEM = Int; *)\n\\* @type: ITEM;\nCONSTANT N\n"
	candidates := FindAnnotationCandidates(input)
	if len(candidates) != 2 {
		t.Fatalf("FindAnnotationCandidates() = %#v", candidates)
	}
	if candidates[0].Text != "@typeAlias: ITEM = Int;" || candidates[0].Position != (SourcePosition{Line: 3, Column: 6}) {
		t.Fatalf("alias candidate = %#v", candidates[0])
	}
	if candidates[1].Text != "@type: ITEM;" || candidates[1].Position != (SourcePosition{Line: 4, Column: 4}) {
		t.Fatalf("type candidate = %#v", candidates[1])
	}
}

func TestAnnotationCandidateIdentifiesUnterminatedInlineForm(t *testing.T) {
	candidates := FindAnnotationCandidates("\\* @type: ACTION\nValue == TRUE\n")
	if len(candidates) != 1 || !candidates[0].UnterminatedInline() {
		t.Fatalf("unterminated candidates = %#v", candidates)
	}
	complete := FindAnnotationCandidates("\\* @type: ACTION;\nValue == TRUE\n")
	if len(complete) != 1 || complete[0].UnterminatedInline() {
		t.Fatalf("complete candidates = %#v", complete)
	}
	parenthesized := FindAnnotationCandidates("\\* @type(\"ACTION\")\nValue == TRUE\n")
	if len(parenthesized) != 1 || parenthesized[0].UnterminatedInline() {
		t.Fatalf("parenthesized candidates = %#v", parenthesized)
	}
}

func stringsTrimSpace(value string) string {
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\n' || value[start] == '\t' || value[start] == '\r') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\n' || value[end-1] == '\t' || value[end-1] == '\r') {
		end--
	}
	return value[start:end]
}

func TestAnnotationStoreCopiesAndSupportsConcurrentAccess(t *testing.T) {
	store := NewAnnotationStore()
	id := NewUID()
	annotations := []Annotation{{Name: "type", Arguments: []AnnotationArg{AnnotationString{Text: "Int"}}}}
	store.Set(id, annotations)
	annotations[0].Name = "changed"
	if found, ok := store.Find(id, "type"); !ok || found.Name != "type" {
		t.Fatalf("Find = %#v, %v", found, ok)
	}
	copy := store.Get(id)
	copy[0].Name = "mutated"
	if _, ok := store.Find(id, "type"); !ok {
		t.Fatal("Get exposed store slice")
	}
	var wait sync.WaitGroup
	for index := 0; index < 32; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			store.Set(id, []Annotation{{Name: "type"}})
			store.Get(id)
			store.Find(id, "type")
		}()
	}
	wait.Wait()
}
