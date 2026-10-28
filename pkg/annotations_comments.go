package apalache

import (
	"regexp"
	"strings"
)

var annotationCommentTokens = regexp.MustCompile(`\n|\\\*|\(\*|\*\)|@[a-zA-Z_][a-zA-Z0-9_]*|\)|;|"`)

type annotationCommentState struct {
	oneLine bool
	level   int
	parens  bool
	colon   bool
	quoted  bool
}

// AnnotationCandidate is an annotation-shaped fragment extracted from a TLA+
// comment together with the location of its leading '@' in the original text.
// Offset and Position are byte-based, matching the ASCII-oriented TLA+ source
// coordinates produced by the frontend.
type AnnotationCandidate struct {
	Text     string
	Offset   int
	Position SourcePosition
}

// UnterminatedInline reports the legacy @name: value form whose terminating
// semicolon is absent. Scala Apalache intentionally ignores these fragments
// instead of treating them as malformed annotations (issues #954 and #1292).
func (candidate AnnotationCandidate) UnterminatedInline() bool {
	trimmed := strings.TrimSpace(candidate.Text)
	if len(trimmed) < 2 || trimmed[0] != '@' {
		return false
	}
	index := 1
	for index < len(trimmed) && (trimmed[index] == '_' || trimmed[index] >= 'a' && trimmed[index] <= 'z' ||
		trimmed[index] >= 'A' && trimmed[index] <= 'Z' || trimmed[index] >= '0' && trimmed[index] <= '9') {
		index++
	}
	for index < len(trimmed) && (trimmed[index] == ' ' || trimmed[index] == '\t') {
		index++
	}
	return index < len(trimmed) && trimmed[index] == ':' && !strings.HasSuffix(trimmed, ";")
}

func (s annotationCommentState) inAnnotation() bool { return s.parens || s.colon }
func (s annotationCommentState) excluding() bool    { return s.oneLine && s.level > 0 || s.level > 1 }

// PreprocessAnnotationComments removes TLA+ comment delimiters and extracts
// annotation candidates. It is intentionally total: malformed annotations are
// returned as candidates for ParseAnnotation to diagnose.
func PreprocessAnnotationComments(input string) (string, []string) {
	clean, annotations, _ := preprocessAnnotationComments(input)
	return clean, annotations
}

func preprocessAnnotationComments(input string) (string, []string, []int) {
	var clean strings.Builder
	var candidate strings.Builder
	annotations := make([]string, 0)
	offsets := make([]int, 0)
	candidateStart := -1
	last := 0
	state := annotationCommentState{}
	for _, match := range annotationCommentTokens.FindAllStringIndex(input, -1) {
		start, end := match[0], match[1]
		group := input[start:end]
		validStart := start == 0 || isAnnotationLeading(input[start-1])
		if start > last && !state.excluding() {
			fragment := input[last:start]
			if state.inAnnotation() {
				candidate.WriteString(fragment)
			} else {
				clean.WriteString(fragment)
			}
		}
		last = end
		if !state.inAnnotation() && (group == "\n" || group == ")" || group == ";" || group == "\"") {
			clean.WriteString(group)
		}
		var lookahead byte
		hasLookahead := last < len(input)
		if hasLookahead {
			lookahead = input[last]
		}
		if !state.inAnnotation() && !state.quoted && strings.HasPrefix(group, "@") {
			if !validStart {
				clean.WriteString(group)
			} else if hasLookahead && (lookahead == ':' || lookahead == '(') {
				last++
			} else if annotationParameterlessEnd(lookahead, hasLookahead) {
				annotations = append(annotations, strings.TrimSpace(group))
				offsets = append(offsets, start)
			} else {
				clean.WriteString(group)
			}
		}
		next := nextAnnotationCommentState(state, group, lookahead, hasLookahead, validStart)
		switch {
		case !state.inAnnotation() && !next.inAnnotation():
		case !state.inAnnotation() && next.inAnnotation():
			candidate.Reset()
			candidateStart = start
			candidate.WriteString(strings.TrimSpace(group))
			if hasLookahead {
				candidate.WriteByte(lookahead)
			}
		case state.inAnnotation() && next.inAnnotation():
			if group != "\\*" && group != "(*" && group != "*)" {
				candidate.WriteString(group)
			}
		case state.inAnnotation() && !next.inAnnotation():
			candidate.WriteString(group)
			annotations = append(annotations, candidate.String())
			offsets = append(offsets, candidateStart)
			candidate.Reset()
			candidateStart = -1
		}
		state = next
	}
	if last < len(input) {
		clean.WriteString(input[last:])
	}
	if candidate.Len() > 0 {
		annotations = append(annotations, candidate.String())
		offsets = append(offsets, candidateStart)
	}
	return clean.String(), annotations, offsets
}

// FindAnnotationCandidates is the position-preserving counterpart of
// PreprocessAnnotationComments. Candidates retain their extraction order.
func FindAnnotationCandidates(input string) []AnnotationCandidate {
	_, texts, offsets := preprocessAnnotationComments(input)
	result := make([]AnnotationCandidate, 0, len(texts))
	for index, candidate := range texts {
		if index >= len(offsets) || offsets[index] < 0 {
			continue
		}
		offset := offsets[index]
		result = append(result, AnnotationCandidate{
			Text: candidate, Offset: offset, Position: sourcePositionAtOffset(input, offset),
		})
	}
	return result
}

func sourcePositionAtOffset(input string, offset int) SourcePosition {
	if offset < 0 {
		offset = 0
	}
	if offset > len(input) {
		offset = len(input)
	}
	line, column := 1, 1
	for index := 0; index < offset; index++ {
		if input[index] == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return SourcePosition{Line: line, Column: column}
}

func isAnnotationLeading(ch byte) bool {
	return ch == '*' || ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == '\f'
}
func annotationParameterlessEnd(ch byte, exists bool) bool {
	return !exists || strings.ContainsRune(" \n\t\r*", rune(ch))
}
func nextAnnotationCommentState(state annotationCommentState, group string, lookahead byte, hasLookahead, validStart bool) annotationCommentState {
	switch group {
	case "\\*":
		state.oneLine = true
	case "\n":
		if state.oneLine {
			state.oneLine = false
		}
	case "\"":
		if !state.colon {
			state.quoted = !state.quoted
		}
	case "(*":
		if !state.oneLine {
			state.level++
		}
	case "*)":
		if state.level > 0 {
			state.oneLine = false
			state.level--
		} else if state.oneLine {
			state.oneLine = false
			state.level = 0
		}
	case ")":
		if !state.quoted && !state.colon {
			state.parens = false
			state.colon = false
			state.quoted = false
		}
	case ";":
		if !state.quoted && state.inAnnotation() {
			state.parens = false
			state.colon = false
			state.quoted = false
		}
	default:
		if strings.HasPrefix(group, "@") && validStart {
			if hasLookahead && lookahead == '(' && !state.inAnnotation() {
				state.parens = true
				state.colon = false
				state.quoted = false
			} else if hasLookahead && lookahead == ':' && !state.inAnnotation() {
				state.parens = false
				state.colon = true
				state.quoted = false
			} else if !state.inAnnotation() && annotationParameterlessEnd(lookahead, hasLookahead) {
			} else if !state.quoted {
				state.parens = false
				state.colon = false
				state.quoted = false
			}
		}
	}
	return state
}
