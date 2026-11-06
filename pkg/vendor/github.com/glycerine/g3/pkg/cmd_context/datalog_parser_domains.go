package cmd_context

// Source: dparser::parse_domains, parse_domain and extract_domain_name.

import (
	"fmt"
	"strconv"
	"strings"
)

func datalogDomainName(name string) (string, error) {
	base := strings.TrimRight(name, "0123456789")
	if base == "" {
		return name, fmt.Errorf("invalid numeric domain name %q", name)
	}
	return base, nil
}

func (p *datalogParser) parseDomains() error {
	for p.current.kind != dlEOF {
		switch p.current.kind {
		case dlNewline:
			return p.advance()
		case dlInclude:
			if err := p.parseInclude(true); err != nil {
				return err
			}
		case dlIdentifier:
			if err := p.parseDomain(); err != nil {
				return err
			}
		default:
			return p.expected("domain declaration, include, or blank line")
		}
	}
	return nil
}

func (p *datalogParser) parseDomain() error {
	name, err := datalogDomainName(p.current.text)
	if err != nil {
		return err
	}
	if _, exists := p.sorts[name]; exists {
		return fmt.Errorf("datalog:%d: sort %s already declared", p.current.line, name)
	}
	if err = p.advance(); err != nil {
		return err
	}
	var sortName = name
	if p.current.kind == dlIdentifier && p.current.text == "int" {
		arithmetic, e := p.manager.ArithmeticFamilyID()
		if e != nil {
			return e
		}
		sort, e := p.manager.MkFamilySort(arithmetic, 1, nil)
		if e != nil {
			return e
		}
		p.sorts[sortName] = sort
		if err = p.advance(); err != nil {
			return err
		}
	} else {
		if err = p.require(dlNumber, "domain size or int"); err != nil {
			return err
		}
		size, e := strconv.ParseUint(p.current.text, 10, 64)
		if e != nil || size == 0 {
			return fmt.Errorf("datalog:%d: invalid domain size %q", p.current.line, p.current.text)
		}
		symbol, e := p.manager.Symbols.Intern(sortName)
		if e != nil {
			return e
		}
		sort, e := p.manager.MkDatalogFiniteSort(symbol, size)
		if e != nil {
			return e
		}
		p.sorts[sortName], p.constants[sort] = sort, make(map[string]uint64)
		if err = p.seedDatalogDomain(sort, false); err != nil {
			return err
		}
		if err = p.advance(); err != nil {
			return err
		}
		if p.current.kind == dlIdentifier { // optional map file
			if err = p.loadConstantMap(sort, p.current.text); err != nil {
				return err
			}
			if err = p.advance(); err != nil {
				return err
			}
		}
	}
	if p.current.kind == dlNewline {
		return p.advance()
	}
	if p.current.kind != dlEOF {
		return p.expected("end of domain line")
	}
	return nil
}
