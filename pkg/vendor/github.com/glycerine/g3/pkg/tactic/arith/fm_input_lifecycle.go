package arith

// Source: src/tactic/arith/fm_tactic.cpp:804-846.
import "github.com/glycerine/g3/pkg/ast"

func (p *FMInputParser) Close() error {
	if p == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if err := p.Expressions.Close(); err != nil {
		return err
	}
	p.closed = true
	p.variableIDs = nil
	p.booleanIDs = nil
	p.forbidden = nil
	return nil
}
