package apalache

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const BugReportFilename = "BugReport.md"

type BugReportOptions struct {
	Command   string
	Version   string
	OS        string
	GoVersion string
}

func QuoteBugReportLog(log string) string {
	return strings.NewReplacer(`\`, `\\`, `$`, `\$`).Replace(log)
}

func PrepareBugReport(manager *OutputManager, options BugReportOptions) (string, error) {
	if manager == nil {
		return "", fmt.Errorf("bug report requires an output manager")
	}
	source := "<!-- TLA+ specification not found. -->"
	if text, found := manager.AllSource(); found {
		source = "```\n" + text + "\n````"
	}
	log := ""
	if text, found, err := manager.ReadFileInRunDir("detailed.log"); err == nil && found {
		log = QuoteBugReportLog(text)
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read detailed log: %w", err)
	}
	osName := options.OS
	if osName == "" {
		osName = runtime.GOOS
	}
	goVersion := options.GoVersion
	if goVersion == "" {
		goVersion = runtime.Version()
	}
	report := formatBugReport(source, options.Command, log, options.Version, osName, goVersion)
	written, err := manager.WithWriterInRunDir([]string{BugReportFilename}, func(writer io.Writer) error {
		_, writeErr := fmt.Fprintln(writer, report)
		return writeErr
	})
	if err != nil {
		return "", fmt.Errorf("write bug report: %w", err)
	}
	if !written {
		return "", fmt.Errorf("bug report requires a configured run directory")
	}
	runDir, err := manager.RunDir()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(runDir, BugReportFilename))
}

func formatBugReport(specification, command, log, version, osName, goVersion string) string {
	return `<!-- Thank you for filing a report! Please ensure you have filled out all -->
<!-- sections, as it help us to address the problem effectively. -->

<!-- NOTE: Please try to ensure the bug can be produced on the latest release of -->
<!-- Apalache. See https://github.com/apalache-mc/apalache/releases -->

## Impact

<!-- Whether this is blocking your work or whether you are able to proceed using -->
<!-- workarounds or alternative approaches. -->

## Input specification

` + specification + `

## The command line parameters used to run the tool

` + "```\n" + command + "\n```" + `

## Expected behavior

<!-- What did you expect to see? -->

## Log files

<details>

` + "```\n" + log + "\n```" + `
</details>

## System information

- Apalache version: ` + "`" + version + "`" + `
- OS: ` + "`" + osName + "`" + `
- Go version: ` + "`" + goVersion + "`" + `

## Triage checklist (for maintainers)

<!-- This section is for maintainers -->

- [ ] Reproduce the bug on the main development branch.
- [ ] Add the issue to the apalache GitHub project.
- [ ] If the bug is high impact, ensure someone available is assigned to fix it.
`
}
