package smtcore

// Source: src/smt/theory_str.cpp nfa::convert_re, epsilon_closure and matches.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// This is the concrete byte NFA used by theory_str's !RegexAutomata
// equality-consistency path, distinct from theory_seq's automata machinery.
type smtStringNFA struct {
	valid       bool
	start, end  uint32
	transitions []map[byte]uint32
	epsilon     [][]uint32
}

func newSMTStringNFA(m *ast.ASTManager, regex ast.ASTHandle) (*smtStringNFA, error) {
	n := &smtStringNFA{valid: true}
	var err error
	n.start, n.end, err = n.convert(m, regex)
	if err != nil {
		n.valid = false
	}
	return n, err
}

func (n *smtStringNFA) state() uint32 {
	id := uint32(len(n.transitions))
	n.transitions = append(n.transitions, nil)
	n.epsilon = append(n.epsilon, nil)
	return id
}

func (n *smtStringNFA) transition(from uint32, symbol byte, to uint32) {
	if n.transitions[from] == nil {
		n.transitions[from] = make(map[byte]uint32)
	}
	n.transitions[from][symbol] = to
}

func (n *smtStringNFA) epsilonMove(from, to uint32) {
	for _, existing := range n.epsilon[from] {
		if existing == to {
			return
		}
	}
	n.epsilon[from] = append(n.epsilon[from], to)
}

func (n *smtStringNFA) convert(m *ast.ASTManager, regex ast.ASTHandle) (start, end uint32, result error) {
	start, end = n.state(), n.state()
	family, err := m.SequenceFamilyID()
	if err != nil {
		return start, end, err
	}
	actual, err := m.GetFamilyID(regex)
	if err != nil {
		return start, end, err
	}
	if actual != family {
		n.valid = false
		return start, end, nil
	}
	kind, err := m.GetDeclKind(regex)
	if err != nil {
		return start, end, err
	}
	switch kind {
	case ast.ASTSequenceOpToRegex:
		argument, err := m.GetArg(regex, 0)
		if err != nil {
			return start, end, err
		}
		text, constant, err := m.SequenceStringValue(argument)
		if err != nil {
			return start, end, err
		}
		if !constant {
			n.valid = false
			return start, end, errors.New("invalid term in str.to.re, argument must be a string constant")
		}
		if len(text) == 0 {
			n.epsilonMove(start, end)
			break
		}
		last := start
		for i := 0; i < len(text)-1; i++ {
			intermediate := n.state()
			n.transition(last, text[i], intermediate)
			last = intermediate
		}
		n.transition(last, text[len(text)-1], end)
	case 17, 18: // concat, union
		left, err := m.GetArg(regex, 0)
		if err != nil {
			return start, end, err
		}
		right, err := m.GetArg(regex, 1)
		if err != nil {
			return start, end, err
		}
		leftStart, leftEnd, err := n.convert(m, left)
		if err != nil {
			return start, end, err
		}
		rightStart, rightEnd, err := n.convert(m, right)
		if err != nil {
			return start, end, err
		}
		n.epsilonMove(start, leftStart)
		if kind == 17 {
			n.epsilonMove(leftEnd, rightStart)
		} else {
			n.epsilonMove(start, rightStart)
			n.epsilonMove(leftEnd, end)
		}
		n.epsilonMove(rightEnd, end)
	case 14: // star
		argument, err := m.GetArg(regex, 0)
		if err != nil {
			return start, end, err
		}
		subStart, subEnd, err := n.convert(m, argument)
		if err != nil {
			return start, end, err
		}
		n.epsilonMove(start, subStart)
		n.epsilonMove(start, end)
		n.epsilonMove(subEnd, subStart)
		n.epsilonMove(subEnd, end)
	case 16: // range
		var bounds [2]byte
		for i := range bounds {
			argument, err := m.GetArg(regex, uint32(i))
			if err != nil {
				return start, end, err
			}
			text, constant, err := m.SequenceStringValue(argument)
			if err != nil {
				return start, end, err
			}
			if !constant || len(text) == 0 {
				return start, end, ast.ErrASTDomain
			}
			bounds[i] = text[0]
		}
		if bounds[0] > bounds[1] {
			bounds[0], bounds[1] = bounds[1], bounds[0]
		}
		for symbol := int(bounds[0]); symbol <= int(bounds[1]); symbol++ {
			n.transition(start, byte(symbol), end)
		}
	case 23: // full sequence
		middle := n.state()
		n.epsilonMove(start, middle)
		n.epsilonMove(middle, end)
		for symbol := 0; symbol < 256; symbol++ {
			n.transition(middle, byte(symbol), middle)
		}
	case 24: // full character
		for symbol := 0; symbol < 256; symbol++ {
			n.transition(start, byte(symbol), end)
		}
	default:
		n.valid = false
	}
	return start, end, nil
}

func (n *smtStringNFA) epsilonClosure(start uint32, closure map[uint32]struct{}) {
	closure[start] = struct{}{}
	work := []uint32{start}
	for head := 0; head < len(work); head++ {
		for _, next := range n.epsilon[work[head]] {
			if _, seen := closure[next]; !seen {
				closure[next] = struct{}{}
				work = append(work, next)
			}
		}
	}
}

func (n *smtStringNFA) matches(input string) bool {
	if n == nil || !n.valid {
		return false
	}
	current := make(map[uint32]struct{})
	n.epsilonClosure(n.start, current)
	for i := 0; i < len(input); i++ {
		next := make(map[uint32]struct{})
		for state := range current {
			if target, found := n.transitions[state][input[i]]; found {
				n.epsilonClosure(target, next)
			}
		}
		current = next
	}
	_, accepted := current[n.end]
	return accepted
}
