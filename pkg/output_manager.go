package apalache

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	IntermediateFolderName = "intermediate"
	RunMetadataFilename    = "run.txt"
)

type OutputManagerOptions struct {
	OutDir            string
	CustomRunDir      string
	Command           string
	Source            InputSource
	WriteIntermediate bool
	Profiling         bool
	Now               func() time.Time
}

// OutputManager owns mutable paths for one tool invocation. Separate manager
// instances are the explicit Go equivalent of Scala's dynamically scoped state.
type OutputManager struct {
	mutex           sync.RWMutex
	configured      bool
	outDir          string
	runDir          string
	customRunDir    string
	intermediateDir string
	profiling       bool
	sourceLines     []string
	sourceSet       bool
}

func NewOutputManager() *OutputManager { return &OutputManager{} }

func (manager *OutputManager) Configure(options OutputManagerOptions) error {
	if manager == nil {
		return fmt.Errorf("cannot configure a nil output manager")
	}
	namespace := options.Command
	switch file := options.Source.(type) {
	case FileInputSource:
		namespace = filepath.Base(file.Path)
	case *FileInputSource:
		if file != nil {
			namespace = filepath.Base(file.Path)
		}
	}
	if namespace == "" {
		return fmt.Errorf("output namespace cannot be empty")
	}
	outDir, err := filepath.Abs(filepath.Join(options.OutDir, namespace))
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	if err = ensureOutputDirectory(outDir); err != nil {
		return err
	}
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	prefix := now().Format("2006-01-02T15-04-05") + "_"
	runDir, err := os.MkdirTemp(outDir, prefix)
	if err != nil {
		return fmt.Errorf("create run directory in %s: %w", outDir, err)
	}
	custom := ""
	if options.CustomRunDir != "" {
		custom, err = filepath.Abs(options.CustomRunDir)
		if err != nil {
			return fmt.Errorf("resolve custom run directory: %w", err)
		}
		if err = ensureOutputDirectory(custom); err != nil {
			return err
		}
	}
	intermediate := ""
	if options.WriteIntermediate {
		intermediate = filepath.Join(runDir, IntermediateFolderName)
		if err = ensureOutputDirectory(intermediate); err != nil {
			return err
		}
		if custom != "" {
			if err = ensureOutputDirectory(filepath.Join(custom, IntermediateFolderName)); err != nil {
				return err
			}
		}
	}
	manager.mutex.Lock()
	manager.configured, manager.outDir, manager.runDir = true, outDir, runDir
	manager.customRunDir, manager.intermediateDir = custom, intermediate
	manager.profiling = options.Profiling
	manager.mutex.Unlock()
	return nil
}

func ensureOutputDirectory(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		absolute, _ := filepath.Abs(path)
		return fmt.Errorf("could not find or create directory: %s: %w", absolute, err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("could not find or create directory: %s", path)
	}
	return nil
}

func (manager *OutputManager) IsConfigured() bool {
	if manager == nil {
		return false
	}
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	return manager.configured
}
func (manager *OutputManager) OutDir() (string, error) {
	if manager == nil {
		return "", fmt.Errorf("out-dir is not configured")
	}
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	if !manager.configured {
		return "", fmt.Errorf("out-dir is not configured")
	}
	return manager.outDir, nil
}
func (manager *OutputManager) RunDir() (string, error) {
	if manager == nil {
		return "", fmt.Errorf("run directory does not exist")
	}
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	if manager.runDir == "" {
		return "", fmt.Errorf("run directory does not exist")
	}
	return manager.runDir, nil
}
func (manager *OutputManager) RunDirPath() (string, bool) {
	if manager == nil {
		return "", false
	}
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	if manager.runDir == "" {
		return "", false
	}
	return manager.runDir, true
}
func (manager *OutputManager) CustomRunDirPath() (string, bool) {
	if manager == nil {
		return "", false
	}
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	if manager.customRunDir == "" {
		return "", false
	}
	return manager.customRunDir, true
}

func (manager *OutputManager) InitSourceLines(source InputSource) error {
	if manager == nil || source == nil || !source.Exists() {
		return nil
	}
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if manager.sourceSet {
		return nil
	}
	content, err := source.ReadUTF8()
	if err != nil {
		return err
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.TrimSuffix(normalized, "\n")
	manager.sourceLines = strings.Split(normalized, "\n")
	manager.sourceSet = true
	return nil
}

func (manager *OutputManager) AllSource() (string, bool) {
	if manager == nil {
		return "", false
	}
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	if !manager.sourceSet {
		return "", false
	}
	return strings.TrimSpace(strings.Join(manager.sourceLines, "\n")), true
}

func (manager *OutputManager) withWriterIn(directories []string, parts []string, write func(io.Writer) error) (bool, error) {
	if len(directories) == 0 {
		return false, nil
	}
	for _, directory := range directories {
		path := filepath.Join(append([]string{directory}, parts...)...)
		if err := withOutputFile(path, write); err != nil {
			return true, err
		}
	}
	return true, nil
}

func withOutputFile(path string, write func(io.Writer) error) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	defer func() {
		flushErr := writer.Flush()
		closeErr := file.Close()
		if err == nil {
			err = flushErr
		}
		if err == nil {
			err = closeErr
		}
	}()
	return write(writer)
}

func (manager *OutputManager) WithWriterInRunDir(parts []string, write func(io.Writer) error) (bool, error) {
	if manager == nil {
		return false, nil
	}
	manager.mutex.RLock()
	runDir, customRunDir := manager.runDir, manager.customRunDir
	manager.mutex.RUnlock()
	if runDir == "" {
		return false, nil
	}
	directories := []string{runDir}
	if customRunDir != "" {
		directories = append(directories, customRunDir)
	}
	return manager.withWriterIn(directories, parts, write)
}

func (manager *OutputManager) WithWriterInIntermediateDir(parts []string, write func(io.Writer) error) (bool, error) {
	if manager == nil {
		return false, nil
	}
	manager.mutex.RLock()
	intermediateDir, customRunDir := manager.intermediateDir, manager.customRunDir
	manager.mutex.RUnlock()
	if intermediateDir == "" {
		return false, nil
	}
	directories := []string{intermediateDir}
	if customRunDir != "" {
		directories = append(directories, filepath.Join(customRunDir, IntermediateFolderName))
	}
	return manager.withWriterIn(directories, parts, write)
}

func (manager *OutputManager) WithProfilingWriter(write func(io.Writer) error) (bool, error) {
	if manager == nil {
		return false, nil
	}
	manager.mutex.RLock()
	profiling := manager.profiling
	manager.mutex.RUnlock()
	if !profiling {
		return false, nil
	}
	return manager.WithWriterInRunDir([]string{"profile-rules.txt"}, write)
}

func ReadOutputFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (manager *OutputManager) ReadFileInRunDir(filename string) (string, bool, error) {
	if manager == nil {
		return "", false, nil
	}
	manager.mutex.RLock()
	runDir := manager.runDir
	manager.mutex.RUnlock()
	if runDir == "" {
		return "", false, nil
	}
	content, err := ReadOutputFile(filepath.Join(runDir, filename))
	return content, true, err
}
