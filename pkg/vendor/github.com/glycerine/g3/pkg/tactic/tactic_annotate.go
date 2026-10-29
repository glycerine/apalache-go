package tactic

// Source: src/tactic/tactical.cpp:1190-1223.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type annotateTactic struct {
	*unaryTactic
	name string
}

func NewAnnotateTactic(name string, child Tactic) (Tactic, error) {
	base, err := newUnaryTactic(unaryAnnotate, child, 0)
	if err != nil {
		return nil, err
	}
	return &annotateTactic{unaryTactic: base, name: name}, nil
}

func writeTacticAnnotation(name, phase string) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(tacticVerbosityLevel)
	if err != nil || !enabled {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	return stream.AppendCString("(" + name + " " + phase + ")\n")
}

func (t *annotateTactic) Apply(g *Goal, out *TacticResult) (result error) {
	if err := t.open(); err != nil {
		return err
	}
	if g == nil || out == nil {
		return ast.ErrASTHandle
	}
	if err := writeTacticAnnotation(t.name, "start"); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, writeTacticAnnotation(t.name, "done")) }()
	return t.child.Apply(g, out)
}

func (t *annotateTactic) Translate(m *ast.ASTManager) (Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	child, err := t.child.Translate(m)
	if err != nil {
		return nil, err
	}
	return NewAnnotateTactic(t.name, child)
}
