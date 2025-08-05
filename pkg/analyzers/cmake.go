// Package analyzers provides build system analysis functionality
package analyzers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/poltergeist/poltergeist/pkg/types"
)

// CMakeAnalyzer analyzes CMake projects and configurations
type CMakeAnalyzer struct {
	projectRoot string
}

// NewCMakeAnalyzer creates a new CMake analyzer
func NewCMakeAnalyzer(projectRoot string) *CMakeAnalyzer {
	return &CMakeAnalyzer{
		projectRoot: projectRoot,
	}
}

// CMakeTarget represents a discovered CMake target
type CMakeTarget struct {
	Name         string
	Type         string
	Sources      []string
	Dependencies []string
	Properties   map[string]string
	Directory    string
}

// CMakeProject represents a CMake project analysis
type CMakeProject struct {
	Name         string
	Version      string
	Targets      []CMakeTarget
	Dependencies []string
	BuildDir     string
	Generator    string
	Variables    map[string]string
}

// AnalysisOptions configures CMake analysis
type AnalysisOptions struct {
	IncludeTests    bool
	AnalyzeDeps     bool
	BuildDir        string
	Generator       string
	RecursiveSearch bool
}

// DefaultAnalysisOptions returns default analysis options
func DefaultAnalysisOptions() *AnalysisOptions {
	return &AnalysisOptions{
		IncludeTests:    true,
		AnalyzeDeps:     true,
		BuildDir:        "build",
		Generator:       "",
		RecursiveSearch: true,
	}
}

// AnalyzeProject analyzes a CMake project
func (a *CMakeAnalyzer) AnalyzeProject(options *AnalysisOptions) (*CMakeProject, error) {
	if options == nil {
		options = DefaultAnalysisOptions()
	}

	project := &CMakeProject{
		Variables: make(map[string]string),
	}

	// Find CMakeLists.txt files
	cmakeFiles, err := a.findCMakeFiles(options.RecursiveSearch)
	if err != nil {
		return nil, fmt.Errorf("failed to find CMake files: %w", err)
	}

	if len(cmakeFiles) == 0 {
		return nil, fmt.Errorf("no CMakeLists.txt files found in project")
	}

	// Analyze main CMakeLists.txt
	mainCMakeFile := filepath.Join(a.projectRoot, "CMakeLists.txt")
	if err := a.analyzeMainCMakeFile(mainCMakeFile, project); err != nil {
		return nil, fmt.Errorf("failed to analyze main CMakeLists.txt: %w", err)
	}

	// Analyze all CMake files for targets
	for _, cmakeFile := range cmakeFiles {
		if err := a.analyzeCMakeFile(cmakeFile, project, options); err != nil {
			// Log error but continue with other files
			continue
		}
	}

	// Set build configuration
	project.BuildDir = options.BuildDir
	project.Generator = options.Generator

	return project, nil
}

// FindTargets discovers CMake targets in the project
func (a *CMakeAnalyzer) FindTargets(options *AnalysisOptions) ([]CMakeTarget, error) {
	project, err := a.AnalyzeProject(options)
	if err != nil {
		return nil, err
	}

	return project.Targets, nil
}

// ValidateTarget validates a CMake target configuration
func (a *CMakeAnalyzer) ValidateTarget(target *types.CMakeExecutableTarget) error {
	if target.TargetName == "" {
		return fmt.Errorf("target name is required")
	}

	// Check if CMakeLists.txt exists
	cmakeFile := filepath.Join(a.projectRoot, "CMakeLists.txt")
	if _, err := os.Stat(cmakeFile); os.IsNotExist(err) {
		return fmt.Errorf("CMakeLists.txt not found in project root")
	}

	// Validate generator if specified
	if target.Generator != "" {
		if err := a.validateGenerator(target.Generator); err != nil {
			return fmt.Errorf("invalid generator: %w", err)
		}
	}

	return nil
}

// GetRecommendedConfig returns recommended configuration for CMake projects
func (a *CMakeAnalyzer) GetRecommendedConfig() (*types.PoltergeistConfig, error) {
	project, err := a.AnalyzeProject(DefaultAnalysisOptions())
	if err != nil {
		return nil, err
	}

	config := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectTypeCMake,
		Targets:     []json.RawMessage{},
	}

	// Create targets based on discovered CMake targets
	for _, cmakeTarget := range project.Targets {
		var target interface{}

		switch cmakeTarget.Type {
		case "EXECUTABLE":
			target = &types.CMakeExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         cmakeTarget.Name,
					Type:         types.TargetTypeCMakeExecutable,
					WatchPaths:   []string{"src/**/*.cpp", "src/**/*.h", "CMakeLists.txt"},
					BuildCommand: fmt.Sprintf("cmake --build build --target %s", cmakeTarget.Name),
				},
				TargetName: cmakeTarget.Name,
				BuildType:  types.CMakeBuildTypeDebug,
			}
		case "STATIC_LIBRARY", "SHARED_LIBRARY":
			libType := types.LibraryTypeStatic
			if cmakeTarget.Type == "SHARED_LIBRARY" {
				libType = types.LibraryTypeDynamic
			}
			target = &types.CMakeLibraryTarget{
				BaseTarget: types.BaseTarget{
					Name:         cmakeTarget.Name,
					Type:         types.TargetTypeCMakeLibrary,
					WatchPaths:   []string{"src/**/*.cpp", "src/**/*.h", "CMakeLists.txt"},
					BuildCommand: fmt.Sprintf("cmake --build build --target %s", cmakeTarget.Name),
				},
				TargetName:  cmakeTarget.Name,
				LibraryType: libType,
				BuildType:   types.CMakeBuildTypeDebug,
			}
		default:
			target = &types.CMakeCustomTarget{
				BaseTarget: types.BaseTarget{
					Name:         cmakeTarget.Name,
					Type:         types.TargetTypeCMakeCustom,
					WatchPaths:   []string{"**/*.cmake", "CMakeLists.txt"},
					BuildCommand: fmt.Sprintf("cmake --build build --target %s", cmakeTarget.Name),
				},
				TargetName: cmakeTarget.Name,
				BuildType:  types.CMakeBuildTypeDebug,
			}
		}

		// Serialize target to JSON
		targetJSON, err := json.Marshal(target)
		if err != nil {
			continue
		}
		config.Targets = append(config.Targets, targetJSON)
	}

	return config, nil
}

func (a *CMakeAnalyzer) findCMakeFiles(recursive bool) ([]string, error) {
	var files []string

	if recursive {
		err := filepath.Walk(a.projectRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.Name() == "CMakeLists.txt" {
				files = append(files, path)
			}

			// Skip build directories
			if info.IsDir() && (info.Name() == "build" || strings.HasPrefix(info.Name(), ".")) {
				return filepath.SkipDir
			}

			return nil
		})
		return files, err
	}

	// Only check root directory
	cmakeFile := filepath.Join(a.projectRoot, "CMakeLists.txt")
	if _, err := os.Stat(cmakeFile); err == nil {
		files = append(files, cmakeFile)
	}

	return files, nil
}

func (a *CMakeAnalyzer) analyzeMainCMakeFile(path string, project *CMakeProject) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	projectNameRegex := regexp.MustCompile(`^\s*project\s*\(\s*([^)\s]+)`)
	versionRegex := regexp.MustCompile(`VERSION\s+([0-9.]+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Extract project name
		if matches := projectNameRegex.FindStringSubmatch(line); len(matches) > 1 {
			project.Name = matches[1]

			// Look for version in the same line
			if versionMatches := versionRegex.FindStringSubmatch(line); len(versionMatches) > 1 {
				project.Version = versionMatches[1]
			}
		}
	}

	return scanner.Err()
}

func (a *CMakeAnalyzer) analyzeCMakeFile(path string, project *CMakeProject, options *AnalysisOptions) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Regex patterns for different CMake constructs
	// Updated to capture library type (STATIC, SHARED, MODULE, INTERFACE, OBJECT)
	targetRegex := regexp.MustCompile(`^\s*(add_executable|add_library)\s*\(\s*([^)\s]+)(?:\s+(STATIC|SHARED|MODULE|INTERFACE|OBJECT))?`)
	addTestRegex := regexp.MustCompile(`^\s*add_test\s*\(\s*([^)\s]+)`)

	dir := filepath.Dir(path)
	relDir, _ := filepath.Rel(a.projectRoot, dir)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Find targets
		if matches := targetRegex.FindStringSubmatch(line); len(matches) > 2 {
			cmdType := strings.ToUpper(matches[1])
			targetName := matches[2]

			// Determine target type
			cmakeTargetType := "EXECUTABLE"
			if cmdType == "ADD_LIBRARY" {
				// Check if library type is specified
				if len(matches) > 3 && matches[3] != "" {
					libType := strings.ToUpper(matches[3])
					switch libType {
					case "SHARED":
						cmakeTargetType = "SHARED_LIBRARY"
					case "MODULE":
						cmakeTargetType = "MODULE_LIBRARY"
					case "INTERFACE":
						cmakeTargetType = "INTERFACE_LIBRARY"
					case "OBJECT":
						cmakeTargetType = "OBJECT_LIBRARY"
					default:
						cmakeTargetType = "STATIC_LIBRARY"
					}
				} else {
					// Default to STATIC if not specified
					cmakeTargetType = "STATIC_LIBRARY"
				}
			}

			target := CMakeTarget{
				Name:       targetName,
				Type:       cmakeTargetType,
				Directory:  relDir,
				Properties: make(map[string]string),
			}

			project.Targets = append(project.Targets, target)
		}

		// Find tests
		if options.IncludeTests {
			if matches := addTestRegex.FindStringSubmatch(line); len(matches) > 1 {
				testName := matches[1]

				target := CMakeTarget{
					Name:       testName,
					Type:       "TEST",
					Directory:  relDir,
					Properties: make(map[string]string),
				}

				project.Targets = append(project.Targets, target)
			}
		}
	}

	return scanner.Err()
}

func (a *CMakeAnalyzer) validateGenerator(generator string) error {
	validGenerators := []string{
		"Unix Makefiles",
		"Ninja",
		"Xcode",
		"Visual Studio",
	}

	for _, valid := range validGenerators {
		if strings.Contains(generator, valid) {
			return nil
		}
	}

	return fmt.Errorf("unsupported generator: %s", generator)
}

// GetBuildCommands returns appropriate build commands for CMake targets
func (a *CMakeAnalyzer) GetBuildCommands(target CMakeTarget, buildType types.CMakeBuildType) []string {
	commands := []string{
		fmt.Sprintf("cmake -B build -DCMAKE_BUILD_TYPE=%s", buildType),
		fmt.Sprintf("cmake --build build --target %s", target.Name),
	}

	return commands
}
