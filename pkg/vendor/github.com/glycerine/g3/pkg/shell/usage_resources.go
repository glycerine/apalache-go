package shell

import "io"

func writeUsageResources(out io.Writer) error {
	return writeShellText(out, `
Resources:
  -T:timeout  set the timeout (in seconds).
  -t:timeout  set the soft timeout (in milli seconds). It only kills the current query.
  -memory:Megabytes  set a limit for virtual memory consumption.
`)
}

func writeUsageOutput(out io.Writer) error {
	if err := writeShellText(out, "\nOutput:\n  -st         display statistics.\n"); err != nil {
		return err
	}
	if !shellTraceOptions && !shellDebugOptions {
		return nil
	}
	if err := writeShellText(out, "\nDebugging support:\n"); err != nil {
		return err
	}
	if shellTraceOptions {
		if err := writeShellText(out, "  -tr:tag     enable trace messages tagged with <tag>.\n"); err != nil {
			return err
		}
	}
	if shellDebugOptions {
		return writeShellText(out, "  -dbg:tag    enable assertions tagged with <tag>.\n")
	}
	return nil
}
