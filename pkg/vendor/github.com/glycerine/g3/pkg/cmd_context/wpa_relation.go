package cmd_context

// Source: wpa_parser_impl::parse_rel_file and inp_num_to_element.

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *wpaParser) readRelations() error {
	files, err := p.files("rel")
	if err != nil {
		return err
	}
	for _, path := range files {
		name := wpaBase(path)
		if strings.Contains(name, "DirectCall") || strings.Contains(name, "FunctionFormals") || strings.Contains(name, "IndirectCall") {
			continue
		}
		declaration := p.declarations[name]
		if declaration.IsNull() {
			return fmt.Errorf("tuple file %s has undeclared predicate %s", path, name)
		}
		if err = p.readRelation(path, declaration); err != nil {
			return err
		}
	}
	return nil
}

func (p *wpaParser) readRelation(path string, declaration ast.ASTHandle) error {
	arity, err := p.manager.GetArity(declaration)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), int(^uint(0)>>1))
	line := 0
	for scanner.Scan() {
		line++
		values, present, parseErr := parseWPARelationLine(scanner.Text())
		if parseErr != nil {
			file.Close()
			return fmt.Errorf("%s:%d: %w", path, line, parseErr)
		}
		if !present {
			continue
		}
		if len(values) != int(arity) {
			file.Close()
			return fmt.Errorf("%s:%d: got %d elements, want %d", path, line, len(values), arity)
		}
		fact := make([]uint32, len(values))
		valid := true
		for i, value := range values {
			sortHandle, getErr := p.manager.GetDomain(declaration, uint32(i))
			if getErr != nil {
				file.Close()
				return getErr
			}
			converted, found, getErr := p.relationElement(sortHandle, value)
			if getErr != nil {
				file.Close()
				return getErr
			}
			if !found {
				valid = false
				break
			}
			fact[i] = converted
		}
		if valid {
			if err = p.context.AddFixedpointTableFact(declaration, fact); err != nil {
				file.Close()
				return err
			}
		}
	}
	err = scanner.Err()
	return joinClose(err, file.Close())
}

func (p *wpaParser) relationElement(sortHandle ast.ASTHandle, number uint64) (uint32, bool, error) {
	name, err := p.manager.GetASTName(sortHandle)
	if err != nil {
		return 0, false, err
	}
	text, err := p.manager.Symbols.String(name)
	if err != nil {
		return 0, false, err
	}
	entry := p.sorts[text]
	if entry == nil {
		return 0, false, fmt.Errorf("unknown WPA sort %s", text)
	}
	builtin := text == "BOOL" || text == "SHORT"
	if !builtin && number != 0 && !entry.numbers[number] {
		return 0, false, nil
	}
	if builtin || !p.useMapNames {
		table := p.rules.numericConstants[sortHandle]
		if index, found := table[number]; found {
			return uint32(index), true, nil
		}
		index := uint64(len(table))
		if index >= entry.size {
			return 0, false, fmt.Errorf("WPA sort %s exceeds finite domain size %d", text, entry.size)
		}
		table[number] = index
		return uint32(index), true, nil
	}
	constant := "<zero element>"
	if number != 0 {
		var found bool
		constant, found = p.mapNames[number]
		if !found {
			return 0, false, fmt.Errorf("unknown WPA symbol number %d", number)
		}
	}
	index, err := p.rules.symbolicConstantNumber(constant, sortHandle)
	return uint32(index), err == nil, err
}
