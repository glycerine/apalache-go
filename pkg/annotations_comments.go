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

func (s annotationCommentState) inAnnotation() bool { return s.parens || s.colon }
func (s annotationCommentState) excluding() bool    { return s.oneLine && s.level > 0 || s.level > 1 }

// PreprocessAnnotationComments removes TLA+ comment delimiters and extracts
// annotation candidates. It is intentionally total: malformed annotations are
// returned as candidates for ParseAnnotation to diagnose.
func PreprocessAnnotationComments(input string) (string, []string) {
	var clean strings.Builder
	var candidate strings.Builder
	annotations := make([]string, 0)
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
			} else {
				clean.WriteString(group)
			}
		}
		next := nextAnnotationCommentState(state, group, lookahead, hasLookahead, validStart)
		switch {
		case !state.inAnnotation() && !next.inAnnotation():
		case !state.inAnnotation() && next.inAnnotation():
			candidate.Reset()
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
			candidate.Reset()
		}
		state = next
	}
	if last < len(input) {
		clean.WriteString(input[last:])
	}
	if candidate.Len() > 0 {
		annotations = append(annotations, candidate.String())
	}
	return clean.String(), annotations
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
