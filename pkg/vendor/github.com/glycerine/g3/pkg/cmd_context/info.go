package cmd_context

// Source: src/cmd_context/basic_cmds.cpp get_info_cmd, set_info_cmd and
// cmd_context::validate_check_sat_result.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/version"
)

func infoStatusText(status lbool.LBool) string {
	if status == lbool.True {
		return "sat"
	}
	if status == lbool.False {
		return "unsat"
	}
	return "unknown"
}

// SetInfoStatus records the benchmark's expected result annotation. It is
// session metadata and intentionally survives reset.
func (c *Context) SetInfoStatus(value string) error {
	if err := c.valid(); err != nil {
		return err
	}
	switch value {
	case "sat":
		c.infoStatus = lbool.True
	case "unsat":
		c.infoStatus = lbool.False
	case "unknown":
		c.infoStatus = lbool.Undef
	default:
		return fmt.Errorf("invalid ':status' attribute")
	}
	return nil
}

// ValidateInfoStatus checks the latest result against the benchmark status.
func (c *Context) ValidateInfoStatus(actual lbool.LBool) error {
	if err := c.valid(); err != nil {
		return err
	}
	if actual == lbool.True && c.infoStatus == lbool.False {
		return fmt.Errorf("check annotation that says unsat")
	}
	if actual == lbool.False && c.infoStatus == lbool.True {
		return fmt.Errorf("check annotation that says sat")
	}
	return nil
}

// GetInfo returns complete SMT-LIB output for fixed information attributes.
// Streaming reason/statistics attributes are handled by their dedicated
// writers.
func (c *Context) GetInfo(name string) (string, bool, error) {
	if err := c.valid(); err != nil {
		return "", false, err
	}
	name = strings.TrimPrefix(name, ":")
	switch name {
	case "error-behavior":
		behavior := "continued-execution"
		if c.ExitOnError {
			behavior = "immediate-exit"
		}
		return "(:error-behavior " + behavior + ")", true, nil
	case "name":
		return `(:name "Z3")`, true, nil
	case "authors":
		return `(:authors "Leonardo de Moura, Nikolaj Bjorner and Christoph Wintersteiger")`, true, nil
	case "version":
		value := strconv.FormatUint(uint64(version.Major), 10) + "." + strconv.FormatUint(uint64(version.Minor), 10) + "." + strconv.FormatUint(uint64(version.Build), 10)
		return `(:version "` + value + `")`, true, nil
	case "status":
		return "(:status " + infoStatusText(c.infoStatus) + ")", true, nil
	case "assertion-stack-levels":
		return "(:assertion-stack-levels " + strconv.Itoa(len(c.scopes)) + ")", true, nil
	default:
		return "", false, nil
	}
}
