package smt2

// Source: src/muz/fp/dl_cmds.cpp collect_query and scope trail.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// ParseAssertionsWithFixedpointQueries parses without executing checks and
// returns owned references to the final scoped fixedpoint query expressions.
// The caller must release each reference with the context's AST manager.
func ParseAssertionsWithFixedpointQueries(context *cmd_context.Context, input io.Reader) (queries []ast.ASTRef, resultErr error) {
	program, err := ParseFixedpointProgram(context, input)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			for i := range program.Queries {
				resultErr = errors.Join(resultErr, program.manager.DelRef(&program.Queries[i]))
			}
			queries = nil
		}
	}()
	for i := range program.Rules {
		resultErr = errors.Join(resultErr, program.manager.DelRef(&program.Rules[i]))
	}
	for i := range program.Relations {
		resultErr = errors.Join(resultErr, program.manager.DelRef(&program.Relations[i]))
	}
	return program.Queries, resultErr
}

// FixedpointProgram holds owned query and bound rule references. Callers must
// release all references through the parser context's manager.
type FixedpointProgram struct {
	Queries   []ast.ASTRef
	Rules     []ast.ASTRef
	Relations []ast.ASTRef
	Names     []string
	// NameSymbols preserves exact source labels; Names is the legacy text view.
	NameSymbols []symbol.Symbol
	manager     *ast.ASTManager
}

// ParseFixedpointProgram collects fixedpoint commands without executing checks
// or ingesting rules into the parser's fixedpoint engine.
func ParseFixedpointProgram(context *cmd_context.Context, input io.Reader) (program FixedpointProgram, resultErr error) {
	c := &fixedpointQueryCollector{}
	defer func() {
		if resultErr != nil {
			cleanupErr := c.clear()
			if c.hasRoots() {
				resultErr = &fixedpointCollectorCleanupFailure{collector: c, cause: resultErr, cleanupErr: cleanupErr}
			} else {
				resultErr = errors.Join(resultErr, cleanupErr)
			}
		}
	}()
	if err := parseCommands(context, input, io.Discard, true, c); err != nil {
		return program, err
	}
	return FixedpointProgram{Queries: c.queries, Rules: c.rules, Relations: c.relations, Names: c.names, NameSymbols: c.nameSymbols, manager: c.manager}, nil
}

type fixedpointCollectorScope struct{ queries, rules, relations int }
type fixedpointQueryCollector struct {
	manager     *ast.ASTManager
	queries     []ast.ASTRef
	rules       []ast.ASTRef
	relations   []ast.ASTRef
	names       []string
	nameSymbols []symbol.Symbol
	scopes      []fixedpointCollectorScope
}

func (c *fixedpointQueryCollector) addRelation(p *commandParser, declaration ast.ASTHandle) error {
	m, err := p.astManager()
	if err != nil {
		return err
	}
	c.manager = m
	ref, err := m.NewRef(declaration)
	if err != nil {
		return err
	}
	c.relations = append(c.relations, ref)
	return nil
}

func (c *fixedpointQueryCollector) addRule(p *commandParser, formula ast.ASTHandle, name symbol.Symbol) error {
	m, err := p.astManager()
	if err != nil {
		return err
	}
	c.manager = m
	text := ""
	if name != (symbol.Symbol{}) {
		text, err = m.Symbols.String(name)
		if err != nil {
			return err
		}
	}
	ref, err := p.context.BindFixedpointRule(formula)
	if err != nil {
		return err
	}
	c.rules = append(c.rules, ref)
	c.names = append(c.names, text)
	c.nameSymbols = append(c.nameSymbols, name)
	return nil
}

func (c *fixedpointQueryCollector) add(p *commandParser, declaration ast.ASTHandle) error {
	m, err := p.astManager()
	if err != nil {
		return err
	}
	c.manager = m
	count, err := m.GetArity(declaration)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		sort, e := m.GetDomain(declaration, i)
		if e != nil {
			return e
		}
		args[i], e = p.pin(m.MkVar(i, sort))
		if e != nil {
			return e
		}
	}
	app, err := m.MkApp(declaration, args)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(app)
	if err != nil {
		return err
	}
	c.queries = append(c.queries, ref)
	return nil
}
func (c *fixedpointQueryCollector) push(n uint32) {
	for i := uint32(0); i < n; i++ {
		c.scopes = append(c.scopes, fixedpointCollectorScope{len(c.queries), len(c.rules), len(c.relations)})
	}
}
func (c *fixedpointQueryCollector) truncate(scope fixedpointCollectorScope) (result error) {
	result = errors.Join(result, c.releaseSuffix(c.queries, scope.queries))
	result = errors.Join(result, c.releaseSuffix(c.rules, scope.rules))
	result = errors.Join(result, c.releaseSuffix(c.relations, scope.relations))
	if result != nil {
		return result
	}
	c.relations = c.relations[:scope.relations]
	c.queries = c.queries[:scope.queries]
	c.rules = c.rules[:scope.rules]
	c.names = c.names[:scope.rules]
	c.nameSymbols = c.nameSymbols[:scope.rules]
	return result
}

func (c *fixedpointQueryCollector) releaseSuffix(references []ast.ASTRef, first int) error {
	var result error
	for i := len(references) - 1; i >= first; i-- {
		if references[i] == (ast.ASTRef{}) {
			continue
		}
		result = errors.Join(result, c.manager.DelRef(&references[i]))
	}
	return result
}

func (c *fixedpointQueryCollector) pop(n uint32) error {
	if n == 0 {
		return nil
	}
	if int(n) > len(c.scopes) {
		if err := c.truncate(fixedpointCollectorScope{}); err != nil {
			return err
		}
		c.scopes = nil
		return nil
	}
	first := len(c.scopes) - int(n)
	err := c.truncate(c.scopes[first])
	if err == nil {
		c.scopes = c.scopes[:first]
	}
	return err
}
func (c *fixedpointQueryCollector) clear() error {
	err := c.truncate(fixedpointCollectorScope{})
	if err == nil {
		c.manager = nil
		c.scopes = nil
	}
	return err
}

func (c *fixedpointQueryCollector) hasRoots() bool {
	return c != nil && (len(c.queries) != 0 || len(c.rules) != 0 || len(c.relations) != 0)
}

type fixedpointCollectorCleanupFailure struct {
	collector  *fixedpointQueryCollector
	cause      error
	cleanupErr error
}

var _ memory.ConstructionFailure = (*fixedpointCollectorCleanupFailure)(nil)

func (f *fixedpointCollectorCleanupFailure) Error() string {
	return errors.Join(f.cause, f.cleanupErr).Error()
}
func (f *fixedpointCollectorCleanupFailure) Unwrap() error {
	return errors.Join(f.cause, f.cleanupErr)
}
func (f *fixedpointCollectorCleanupFailure) Cleanup() error {
	if f == nil || f.collector == nil {
		return nil
	}
	f.cleanupErr = f.collector.clear()
	if !f.collector.hasRoots() {
		f.collector = nil
	}
	return f.cleanupErr
}
func (f *fixedpointCollectorCleanupFailure) RuntimeOwner() *memory.Runtime {
	if f == nil || f.collector == nil || f.collector.manager == nil || f.collector.manager.NumeralService() == nil {
		return nil
	}
	return f.collector.manager.NumeralService().RuntimeOwner()
}
func (f *fixedpointCollectorCleanupFailure) OriginalCause() error { return f.cause }
func (f *fixedpointCollectorCleanupFailure) CleanupCause() error  { return f.cleanupErr }
