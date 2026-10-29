package lp

// Source: src/test/lp.cpp:2132-2264. Retained GLPK report comparison.

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// ErrDriverReport transports the source integer exception for an absent or
// malformed report field. Native file-read and writer errors remain distinct.
var ErrDriverReport = errors.New("lp: missing or malformed comparison report")

type DriverComparisonCounts struct{ Successes, Failures, Inconclusives uint32 }

func driverReportOutput(out io.Writer) io.Writer {
	if out == nil {
		return os.Stdout
	}
	return out
}

func readDriverReportLine(name, field string, last bool, out io.Writer) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		if e := writeDriverLine(out, "cannot open "+name); e != nil {
			return "", e
		}
		return "", ErrDriverReport
	}
	defer f.Close()
	r := bufio.NewReader(f)
	found := ""
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSuffix(line, "\n")
		if err != nil && err != io.EOF {
			return found, err
		}
		if strings.Contains(line, field) {
			found = line
			if !last {
				return found, nil
			}
		}
		if err == io.EOF {
			break
		}
	}
	if found == "" {
		label := field
		if field == "Status" {
			label = "the status"
		} else {
			label = "the " + field
		}
		if err = writeDriverLine(out, "cannot find "+label+" line in "+name); err != nil {
			return "", err
		}
		return "", ErrDriverReport
	}
	return found, nil
}

// ReadDriverReportStatus returns the second token of the first line containing
// case-sensitive "Status". It deliberately accepts unknown status vocabulary.
func ReadDriverReportStatus(name string, out io.Writer) (string, error) {
	out = driverReportOutput(out)
	line, err := readDriverReportLine(name, "Status", false, out)
	if err != nil {
		return "", err
	}
	words := mpsWords(line)
	if len(words) != 2 {
		if err = writeDriverLine(out, "unexpected Status string "+line); err != nil {
			return "", err
		}
		return "", ErrDriverReport
	}
	return words[1], nil
}

func ReadGLPKReportCost(name string, out io.Writer) (float64, error) {
	out = driverReportOutput(out)
	line, err := readDriverReportLine(name, "Objective", false, out)
	if err != nil {
		return 0, err
	}
	words := mpsWords(line)
	if len(words) != 5 {
		if err = writeDriverLine(out, "unexpected Objective std::string "+line); err != nil {
			return 0, err
		}
		return 0, ErrDriverReport
	}
	return mpsNumber(words[3])
}

// ReadLPReportCost uses the LAST line containing "cost", including a later
// "primal cost" report. It retains the source's exact three-token check.
func ReadLPReportCost(name string, out io.Writer) (float64, error) {
	out = driverReportOutput(out)
	line, err := readDriverReportLine(name, "cost", true, out)
	if err != nil {
		return 0, err
	}
	words := mpsWords(line)
	if len(words) != 3 {
		if err = writeDriverLine(out, "unexpected cost string "+line); err != nil {
			return 0, err
		}
		return 0, ErrDriverReport
	}
	return mpsNumber(words[2])
}

func compareDriverReportStatuses(glpkName, lpName string, counts *DriverComparisonCounts, out io.Writer) (bool, error) {
	glpk, err := ReadDriverReportStatus(glpkName, out)
	if err != nil {
		return false, err
	}
	local, err := ReadDriverReportStatus(lpName, out)
	if err != nil {
		return false, err
	}
	if glpk != local {
		if glpk == "UNDEFINED" && (local == "UNBOUNDED" || local == "INFEASIBLE") {
			counts.Successes++
			return false, nil
		}
		if _, err = io.WriteString(out, "glpsol and lp_tst disagree: glpsol status is "+glpk); err != nil {
			return false, err
		}
		if err = writeDriverLine(out, " but lp_tst status is "+local); err != nil {
			return false, err
		}
		counts.Failures++
		return false, nil
	}
	return local == "OPTIMAL", nil
}

// CompareGLPKDriverReports implements the ordinary retained comparison path.
// Equal nonoptimal statuses do not increment any counter. Status disagreement
// publishes failure AFTER reporting; cost disagreement publishes it BEFORE.
// Reports are reopened for cost reads, preserving changes during diagnostics.
func CompareGLPKDriverReports(glpkName, lpName string, counts *DriverComparisonCounts, out io.Writer) error {
	if counts == nil {
		return ErrBounds
	}
	out = driverReportOutput(out)
	compare, err := compareDriverReportStatuses(glpkName, lpName, counts, out)
	if err != nil || !compare {
		return err
	}
	a, err := ReadGLPKReportCost(glpkName, out)
	if err != nil {
		return err
	}
	b, err := ReadLPReportCost(lpName, out)
	if err != nil {
		return err
	}
	if DriverValuesOnePercentClose(a, b) {
		counts.Successes++
		return nil
	}
	counts.Failures++
	// Source is one insertion chain ending in endl; each operand is a snapshot
	// of the completed file reads. The default double presentation is shared.
	if _, err = io.WriteString(out, "glpsol cost is "); err != nil {
		return err
	}
	if _, err = io.WriteString(out, floatLPNumberText(a)); err != nil {
		return err
	}
	if _, err = io.WriteString(out, " lp_tst cost is "); err != nil {
		return err
	}
	return writeDriverLine(out, floatLPNumberText(b))
}
