package smt2

// Source: src/cmd_context/parametric_cmd.cpp.

import (
	"fmt"

	uparams "github.com/glycerine/g3/pkg/util/params"
)

// parametricKeyword returns the normalized current command keyword.
// Each command keeps its own parameter set, as parametric_cmd::prepare does.
func (p *commandParser) parametricKeyword() (string, error) {
	if p.token != KeywordToken {
		return "", fmt.Errorf("invalid keyword argument")
	}
	raw := p.scanner.ID()
	key := uparams.NormParamName(&raw)
	return key, nil
}
