package cmd_context

// Source: dparser::parse_mapfile. Lines assign finite-domain symbols in
// insertion order; their spelling, rather than a C string table, is retained.

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *datalogParser) loadConstantMap(sort ast.ASTHandle, filename string) error {
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(p.path, filename)
	}
	file, err := os.Open(filename)
	if err != nil {
		// Source parse_mapfile reports a warning and continues when an
		// optional map cannot be opened. This API has no parser diagnostic
		// stream, so preserve the functional behavior without global stderr.
		return nil
	}
	defer file.Close()
	table := p.constants[sort]
	if table == nil {
		table = make(map[string]uint64)
		p.constants[sort] = table
	}
	scanner := bufio.NewScanner(file)
	// Source line_reader grows to the complete symbol spelling.
	scanner.Buffer(make([]byte, 1024), int(^uint(0)>>1))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if _, found := table[name]; !found {
			table[name] = uint64(len(table))
		}
	}
	return scanner.Err()
}
