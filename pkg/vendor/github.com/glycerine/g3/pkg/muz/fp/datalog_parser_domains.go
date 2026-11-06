package fp

// Source: dparser::parse_domains, parse_domain and extract_domain_name.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (p *datalogParser) parseDomain() error {
	name, ok := domainBaseName(p.token.text)
	if !ok {
		return p.fail("expected domain name")
	}
	p.advance()
	if p.token.kind == datalogIdentifier && p.token.text == "int" {
		family, err := p.manager.RegisterArithmeticPlugin()
		if err != nil {
			return err
		}
		sort, err := p.manager.MkFamilySort(family, 1, nil)
		if err != nil {
			return err
		}
		if _, exists := p.sorts[name]; exists {
			return p.fail("domain is already declared")
		}
		p.sorts[name] = sort
		p.advance()
	} else {
		if p.token.kind != datalogNumber {
			return p.fail("expected domain size or int")
		}
		size, err := strconv.ParseUint(p.token.text, 10, 64)
		if err != nil || size == 0 {
			return p.fail("invalid finite domain size")
		}
		nameSymbol, err := p.symbol(name)
		if err != nil {
			return err
		}
		sort, err := p.manager.MkDatalogFiniteSort(nameSymbol, size)
		if err != nil {
			return err
		}
		if _, exists := p.sorts[name]; exists {
			return p.fail("domain is already declared")
		}
		if err = p.context.RegisterFiniteSort(sort, base.DatalogSortSymbol); err != nil {
			return err
		}
		p.sorts[name] = sort
		p.advance()
		if p.token.kind == datalogIdentifier {
			if err = p.loadMap(sort, p.token.text); err != nil {
				return err
			}
			p.advance()
		}
	}
	if p.token.kind != datalogNewline && p.token.kind != datalogEOF {
		return p.fail("expected end of domain line")
	}
	if p.token.kind == datalogNewline {
		p.advance()
	}
	return nil
}
