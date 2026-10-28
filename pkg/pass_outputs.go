package apalache

import (
	"fmt"
	"io"
)

// StandardTLAModules returns the modules supported directly by Apalache IR.
// The returned slice is fresh so callers cannot alter later pass output.
func StandardTLAModules() []string {
	return []string{"Integers", "Sequences", "FiniteSets", "TLC", "Apalache", "Variants"}
}

type PassOutputOptions struct {
	Annotations *AnnotationStore
	SourceStore *SourceStore
}

// WritePassOutputs emits the TLA+ and Apalache-IR JSON representations used
// for Scala PassWithOutputs intermediate snapshots. Formatting is lazy: when
// intermediate output is disabled, the module is not inspected.
func WritePassOutputs(
	manager *OutputManager,
	passNumber int,
	passName string,
	module *Module,
	options PassOutputOptions,
) (bool, error) {
	outputName := fmt.Sprintf("%02d_Out%s", passNumber, passName)
	rename := func() (*Module, error) {
		if module == nil {
			return nil, fmt.Errorf("pass %s cannot write a nil module", passName)
		}
		output := NewModule(outputName, module.Declarations())
		output.copyMetadataFrom(module)
		return output, nil
	}

	tlaWritten, err := manager.WithWriterInIntermediateDir([]string{outputName + ".tla"}, func(writer io.Writer) error {
		output, renameErr := rename()
		if renameErr != nil {
			return renameErr
		}
		formatted, formatErr := FormatTLAModule(output, TLAFormatOptions{
			ExtendedModules: StandardTLAModules(),
			Annotations:     options.Annotations,
		})
		if formatErr != nil {
			return fmt.Errorf("format pass %s TLA+ output: %w", passName, formatErr)
		}
		_, writeErr := io.WriteString(writer, formatted)
		return writeErr
	})
	if err != nil {
		return tlaWritten, err
	}

	jsonWritten, err := manager.WithWriterInIntermediateDir([]string{outputName + ".json"}, func(writer io.Writer) error {
		output, renameErr := rename()
		if renameErr != nil {
			return renameErr
		}
		encoded, encodeErr := MarshalSingleModuleIRJSON(output, IRJSONOptions{SourceStore: options.SourceStore})
		if encodeErr != nil {
			return fmt.Errorf("format pass %s JSON output: %w", passName, encodeErr)
		}
		_, writeErr := writer.Write(encoded)
		return writeErr
	})
	return tlaWritten || jsonWritten, err
}
