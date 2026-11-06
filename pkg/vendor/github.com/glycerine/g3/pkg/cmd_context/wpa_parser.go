package cmd_context

// Source: src/muz/fp/datalog_parser.cpp wpa_parser_impl. The retained shell
// directory path loads map files, declarations/rules and relation tuples.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type wpaParser struct {
	context      *Context
	manager      *ast.ASTManager
	directory    string
	mapNames     map[uint64]string
	sorts        map[string]*wpaSort
	declarations map[string]ast.ASTHandle
	rules        *datalogParser
	useMapNames  bool
}

type wpaSort struct {
	name    string
	numbers map[uint64]bool
	size    uint64
}

func newWPAParser(context *Context, directory string) (*wpaParser, error) {
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	engine, err := context.fixedpointContext()
	if err != nil {
		return nil, err
	}
	useNames, err := engine.DatalogUseMapNames()
	if err != nil {
		return nil, err
	}
	rules, err := newDatalogParser(context)
	if err != nil {
		return nil, err
	}
	return &wpaParser{context: context, manager: manager, directory: directory, mapNames: make(map[uint64]string), sorts: make(map[string]*wpaSort), declarations: make(map[string]ast.ASTHandle), rules: rules, useMapNames: useNames}, nil
}

// ParseWPADirectory parses the legacy directory benchmark selected by the
// Datalog shell when its input path is a directory.
func (c *Context) ParseWPADirectory(directory string) (resultErr error) {
	p, err := newWPAParser(c, directory)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.rules.close()) }()
	if err = p.readMaps(); err != nil {
		return err
	}
	if err = p.finishSorts(); err != nil {
		return err
	}
	if err = p.readRules(); err != nil {
		return err
	}
	if err = p.indexDeclarations(); err != nil {
		return err
	}
	if err = p.readRelations(); err != nil {
		return err
	}
	return c.rememberDatalogDisplay(p.rules)
}

func (p *wpaParser) finishSorts() error {
	names := make([]string, 0, len(p.sorts))
	for name, entry := range p.sorts {
		if name == "BOOL" || name == "SHORT" {
			return fmt.Errorf("WPA sort %s is already declared", name)
		}
		// Source reserves capacity for zero, without interning it. An
		// explicitly mapped zero still contributes to the declared size.
		entry.size = uint64(len(entry.numbers)) + 1
		names = append(names, name)
	}
	sort.Strings(names)
	p.sorts["BOOL"] = &wpaSort{name: "BOOL", size: 2}
	p.sorts["SHORT"] = &wpaSort{name: "SHORT", size: 65536}
	names = append([]string{"BOOL", "SHORT"}, names...)
	p.rules.numericConstants = make(map[ast.ASTHandle]map[uint64]uint64)
	for _, name := range names {
		symbol, err := p.manager.Symbols.Intern(name)
		if err != nil {
			return err
		}
		root, err := p.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		p.rules.pins = append(p.rules.pins, root)
		handle, err := p.manager.MkDatalogFiniteSort(symbol, p.sorts[name].size)
		if err = errors.Join(err, p.manager.SetRef(root, handle)); err != nil {
			return err
		}
		p.rules.sorts[name] = handle
		if name == "BOOL" || name == "SHORT" || !p.useMapNames {
			p.rules.numericConstants[handle] = make(map[uint64]uint64)
		} else {
			p.rules.constants[handle] = make(map[string]uint64)
		}
		if err = p.rules.seedDatalogDomain(handle, name == "BOOL" || name == "SHORT" || !p.useMapNames); err != nil {
			return err
		}
	}
	return nil
}
