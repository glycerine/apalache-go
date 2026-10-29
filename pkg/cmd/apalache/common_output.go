package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	pal "github.com/glycerine/apalache-go"
)

type commonOutputFlags struct {
	debug             optionalBoolFlag
	writeIntermediate optionalBoolFlag
}

type commonOutputOptions struct {
	debug             bool
	writeIntermediate bool
}

func addCommonOutputFlags(flags *flag.FlagSet) *commonOutputFlags {
	options := &commonOutputFlags{}
	flags.Var(&options.debug, "debug", "write the resolved application configuration to the run directory")
	flags.Var(&options.writeIntermediate, "write-intermediate", "write intermediate representations to the run directory")
	return options
}

func (flags *commonOutputFlags) resolve(config jsonCommandConfig, visited map[string]bool) (commonOutputOptions, error) {
	options := commonOutputOptions{}
	if visited["debug"] {
		options.debug = flags.debug.value
	} else if config.Debug != nil {
		options.debug = *config.Debug
	}
	if visited["write-intermediate"] {
		options.writeIntermediate = flags.writeIntermediate.value
	} else if config.WriteIntermediate != nil {
		options.writeIntermediate = *config.WriteIntermediate
	} else if value, found := os.LookupEnv("WRITE_INTERMEDIATE"); found {
		var parsed optionalBoolFlag
		if err := parsed.Set(value); err != nil {
			return commonOutputOptions{}, fmt.Errorf("invalid WRITE_INTERMEDIATE environment value %q: %w", value, err)
		}
		options.writeIntermediate = parsed.value
	}
	return options, nil
}

func writeDebugApplicationConfig(
	paths commandOutputPaths,
	config jsonCommandConfig,
	command, selectedConfig, sourcePath, outDir, runDir string,
	options commonOutputOptions,
) error {
	if !options.debug {
		return nil
	}
	config.Command = stringPointer(command)
	if selectedConfig != "" {
		config.ConfigFile = stringPointer(selectedConfig)
	}
	config.OutDir = stringPointer(outDir)
	if runDir != "" {
		config.RunDir = stringPointer(runDir)
	}
	config.Debug = boolPointer(true)
	config.WriteIntermediate = boolPointer(options.writeIntermediate)
	if config.SMTProfile == nil {
		config.SMTProfile = boolPointer(false)
	}
	if config.Profiling == nil {
		config.Profiling = boolPointer(false)
	}
	if config.Features == nil {
		empty := []string{}
		config.Features = &empty
	}
	if config.Source == nil && sourcePath != "" {
		config.Source = &jsonInputSource{Kind: stringPointer("file"), Path: stringPointer(sourcePath)}
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode application configuration: %w", err)
	}
	var snapshot map[string]any
	if err = json.Unmarshal(encoded, &snapshot); err != nil {
		return fmt.Errorf("normalize application configuration: %w", err)
	}
	pruneNullConfigurationValues(snapshot)
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("format application configuration: %w", err)
	}
	data = append(data, '\n')
	for _, path := range paths.implicitFiles("application-config.json") {
		if err = os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write application configuration to %s: %w", path, err)
		}
	}
	return nil
}

func pruneNullConfigurationValues(object map[string]any) {
	for key, value := range object {
		switch nested := value.(type) {
		case nil:
			delete(object, key)
		case map[string]any:
			pruneNullConfigurationValues(nested)
			if len(nested) == 0 {
				delete(object, key)
			}
		}
	}
}

func boolPointer(value bool) *bool { return &value }

func writeFrontendIntermediate(
	paths commandOutputPaths,
	module *pal.Module,
	extends []string,
	sources *pal.SourceStore,
	baseName string,
) error {
	formatted, err := pal.FormatTLAModule(module, pal.TLAFormatOptions{ExtendedModules: extends})
	if err != nil {
		return fmt.Errorf("format frontend intermediate TLA+: %w", err)
	}
	encoded, err := pal.MarshalSingleModuleIRJSON(module, pal.IRJSONOptions{SourceStore: sources})
	if err != nil {
		return fmt.Errorf("format frontend intermediate JSON: %w", err)
	}
	for extension, data := range map[string][]byte{".tla": []byte(formatted), ".json": encoded} {
		files, pathErr := paths.intermediateFiles(baseName + extension)
		if pathErr != nil {
			return pathErr
		}
		for _, path := range files {
			if err = os.WriteFile(path, data, 0o644); err != nil {
				return fmt.Errorf("write frontend intermediate to %s: %w", path, err)
			}
		}
	}
	return nil
}
