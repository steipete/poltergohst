// Package xcode provides Xcode-specific helper functionality
package xcode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/poltergeist/poltergeist/pkg/types"
)

// XcodeHelper provides Xcode-specific functionality
type XcodeHelper struct {
	projectRoot string
}

// NewXcodeHelper creates a new Xcode helper
func NewXcodeHelper(projectRoot string) *XcodeHelper {
	return &XcodeHelper{
		projectRoot: projectRoot,
	}
}

// XcodeProject represents an Xcode project
type XcodeProject struct {
	Path    string
	Name    string
	Targets []XcodeTarget
	Schemes []XcodeScheme
	WorkDir string
}

// XcodeTarget represents an Xcode target
type XcodeTarget struct {
	Name         string
	Type         string
	Platform     types.Platform
	BundleID     string
	OutputPath   string
	Dependencies []string
}

// XcodeScheme represents an Xcode scheme
type XcodeScheme struct {
	Name        string
	Target      string
	BuildConfig string
	IsShared    bool
}

// BuildSettings represents Xcode build settings
type BuildSettings struct {
	Configuration string
	Platform      string
	Arch          string
	SDK           string
	Settings      map[string]string
}

// FindXcodeProjects discovers Xcode projects in the directory
func (h *XcodeHelper) FindXcodeProjects() ([]XcodeProject, error) {
	var projects []XcodeProject

	err := filepath.Walk(h.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories and common build/cache directories
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") ||
			info.Name() == "build" ||
			info.Name() == "DerivedData" ||
			info.Name() == "Pods") {
			return filepath.SkipDir
		}

		// Look for .xcodeproj and .xcworkspace files
		if strings.HasSuffix(path, ".xcodeproj") || strings.HasSuffix(path, ".xcworkspace") {
			project, err := h.analyzeProject(path)
			if err != nil {
				// Log error but continue
				return nil
			}
			projects = append(projects, *project)
		}

		return nil
	})

	return projects, err
}

// GetProjectInfo returns information about an Xcode project
func (h *XcodeHelper) GetProjectInfo(projectPath string) (*XcodeProject, error) {
	return h.analyzeProject(projectPath)
}

// ListTargets lists all targets in an Xcode project
func (h *XcodeHelper) ListTargets(projectPath string) ([]XcodeTarget, error) {
	project, err := h.analyzeProject(projectPath)
	if err != nil {
		return nil, err
	}
	return project.Targets, nil
}

// ListSchemes lists all schemes in an Xcode project
func (h *XcodeHelper) ListSchemes(projectPath string) ([]XcodeScheme, error) {
	project, err := h.analyzeProject(projectPath)
	if err != nil {
		return nil, err
	}
	return project.Schemes, nil
}

// BuildTarget builds a specific target
func (h *XcodeHelper) BuildTarget(projectPath, target, configuration, platform string) error {
	args := []string{
		"-project", projectPath,
		"-target", target,
		"-configuration", configuration,
	}

	if platform != "" {
		args = append(args, "-destination", fmt.Sprintf("platform=%s", platform))
	}

	cmd := exec.Command("xcodebuild", args...)
	cmd.Dir = h.projectRoot

	return cmd.Run()
}

// BuildScheme builds using a specific scheme
func (h *XcodeHelper) BuildScheme(projectPath, scheme, configuration string) error {
	args := []string{
		"-project", projectPath,
		"-scheme", scheme,
		"-configuration", configuration,
		"build",
	}

	cmd := exec.Command("xcodebuild", args...)
	cmd.Dir = h.projectRoot

	return cmd.Run()
}

// CleanTarget cleans a specific target
func (h *XcodeHelper) CleanTarget(projectPath, target string) error {
	args := []string{
		"-project", projectPath,
		"-target", target,
		"clean",
	}

	cmd := exec.Command("xcodebuild", args...)
	cmd.Dir = h.projectRoot

	return cmd.Run()
}

// GetBuildSettings retrieves build settings for a target
func (h *XcodeHelper) GetBuildSettings(projectPath, target, configuration string) (*BuildSettings, error) {
	args := []string{
		"-project", projectPath,
		"-target", target,
		"-configuration", configuration,
		"-showBuildSettings",
		"-json",
	}

	cmd := exec.Command("xcodebuild", args...)
	cmd.Dir = h.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get build settings: %w", err)
	}

	// Parse xcodebuild JSON output
	var buildSettingsOutput []map[string]interface{}
	if err := json.Unmarshal(output, &buildSettingsOutput); err != nil {
		return nil, fmt.Errorf("failed to parse build settings: %w", err)
	}

	if len(buildSettingsOutput) == 0 {
		return nil, fmt.Errorf("no build settings found")
	}

	settings := &BuildSettings{
		Configuration: configuration,
		Settings:      make(map[string]string),
	}

	// Extract build settings from the first result
	if buildSettings, ok := buildSettingsOutput[0]["buildSettings"].(map[string]interface{}); ok {
		for key, value := range buildSettings {
			if strValue, ok := value.(string); ok {
				settings.Settings[key] = strValue
			}
		}
	}

	// Extract common settings
	if platform, ok := settings.Settings["PLATFORM_NAME"]; ok {
		settings.Platform = platform
	}
	if arch, ok := settings.Settings["ARCHS"]; ok {
		settings.Arch = arch
	}
	if sdk, ok := settings.Settings["SDKROOT"]; ok {
		settings.SDK = sdk
	}

	return settings, nil
}

// ValidateProject validates an Xcode project
func (h *XcodeHelper) ValidateProject(projectPath string) error {
	if !strings.HasSuffix(projectPath, ".xcodeproj") && !strings.HasSuffix(projectPath, ".xcworkspace") {
		return fmt.Errorf("invalid Xcode project path: %s", projectPath)
	}

	fullPath := filepath.Join(h.projectRoot, projectPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("Xcode project not found: %s", fullPath)
	}

	// Check if xcodebuild is available
	if _, err := exec.LookPath("xcodebuild"); err != nil {
		return fmt.Errorf("xcodebuild not found in PATH")
	}

	return nil
}

// GetRecommendedConfig generates recommended Poltergeist configuration for Xcode projects
func (h *XcodeHelper) GetRecommendedConfig() (*types.PoltergeistConfig, error) {
	projects, err := h.FindXcodeProjects()
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("no Xcode projects found")
	}

	config := &types.PoltergeistConfig{
		Version:     "1.0",
		ProjectType: types.ProjectTypeSwift,
		Targets:     []json.RawMessage{},
	}

	// Create targets for each Xcode target
	for _, project := range projects {
		for _, xcodeTarget := range project.Targets {
			var target interface{}

			switch xcodeTarget.Type {
			case "Application":
				target = &types.AppBundleTarget{
					BaseTarget: types.BaseTarget{
						Name: xcodeTarget.Name,
						Type: types.TargetTypeAppBundle,
						WatchPaths: []string{
							"**/*.swift",
							"**/*.m",
							"**/*.h",
							"**/*.xib",
							"**/*.storyboard",
						},
						BuildCommand: h.getBuildCommand(project.Path, xcodeTarget.Name, "Debug"),
					},
					Platform:      xcodeTarget.Platform,
					BundleID:      xcodeTarget.BundleID,
					AutoRelaunch:  &[]bool{true}[0],
					LaunchCommand: h.getLaunchCommand(xcodeTarget),
				}

			case "Framework":
				target = &types.FrameworkTarget{
					BaseTarget: types.BaseTarget{
						Name: xcodeTarget.Name,
						Type: types.TargetTypeFramework,
						WatchPaths: []string{
							"**/*.swift",
							"**/*.m",
							"**/*.h",
						},
						BuildCommand: h.getBuildCommand(project.Path, xcodeTarget.Name, "Debug"),
					},
					Platform:   xcodeTarget.Platform,
					OutputPath: xcodeTarget.OutputPath,
				}

			case "Static Library", "Dynamic Library":
				libType := types.LibraryTypeStatic
				if xcodeTarget.Type == "Dynamic Library" {
					libType = types.LibraryTypeDynamic
				}

				target = &types.LibraryTarget{
					BaseTarget: types.BaseTarget{
						Name: xcodeTarget.Name,
						Type: types.TargetTypeLibrary,
						WatchPaths: []string{
							"**/*.swift",
							"**/*.m",
							"**/*.h",
						},
						BuildCommand: h.getBuildCommand(project.Path, xcodeTarget.Name, "Debug"),
					},
					LibraryType: libType,
					OutputPath:  xcodeTarget.OutputPath,
				}

			case "Unit Test Bundle":
				target = &types.TestTarget{
					BaseTarget: types.BaseTarget{
						Name: xcodeTarget.Name,
						Type: types.TargetTypeTest,
						WatchPaths: []string{
							"**/*Test*.swift",
							"**/*Spec*.swift",
							"**/*.swift",
						},
						BuildCommand: h.getTestCommand(project.Path, xcodeTarget.Name),
					},
					TestCommand: h.getTestCommand(project.Path, xcodeTarget.Name),
				}

			default:
				// Custom target
				target = &types.CustomTarget{
					BaseTarget: types.BaseTarget{
						Name: xcodeTarget.Name,
						Type: types.TargetTypeCustom,
						WatchPaths: []string{
							"**/*.swift",
							"**/*.m",
							"**/*.h",
						},
						BuildCommand: h.getBuildCommand(project.Path, xcodeTarget.Name, "Debug"),
					},
				}
			}

			// Serialize target to JSON
			targetJSON, err := json.Marshal(target)
			if err != nil {
				continue
			}
			config.Targets = append(config.Targets, targetJSON)
		}
	}

	return config, nil
}

func (h *XcodeHelper) analyzeProject(projectPath string) (*XcodeProject, error) {
	project := &XcodeProject{
		Path:    projectPath,
		Name:    h.getProjectName(projectPath),
		WorkDir: filepath.Dir(projectPath),
	}

	// Get targets using xcodebuild
	targets, err := h.getTargetsFromXcodebuild(projectPath)
	if err != nil {
		return nil, err
	}
	project.Targets = targets

	// Get schemes using xcodebuild
	schemes, err := h.getSchemesFromXcodebuild(projectPath)
	if err != nil {
		return nil, err
	}
	project.Schemes = schemes

	return project, nil
}

func (h *XcodeHelper) getProjectName(projectPath string) string {
	name := filepath.Base(projectPath)
	if strings.HasSuffix(name, ".xcodeproj") {
		return name[:len(name)-10]
	}
	if strings.HasSuffix(name, ".xcworkspace") {
		return name[:len(name)-12]
	}
	return name
}

func (h *XcodeHelper) getTargetsFromXcodebuild(projectPath string) ([]XcodeTarget, error) {
	args := []string{"-list", "-json"}

	if strings.HasSuffix(projectPath, ".xcworkspace") {
		args = append(args, "-workspace", projectPath)
	} else {
		args = append(args, "-project", projectPath)
	}

	cmd := exec.Command("xcodebuild", args...)
	cmd.Dir = h.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list targets: %w", err)
	}

	var listOutput struct {
		Project struct {
			Targets []string `json:"targets"`
		} `json:"project"`
	}

	if err := json.Unmarshal(output, &listOutput); err != nil {
		return nil, fmt.Errorf("failed to parse target list: %w", err)
	}

	var targets []XcodeTarget
	for _, targetName := range listOutput.Project.Targets {
		target := XcodeTarget{
			Name:     targetName,
			Type:     "Application", // Default, should be determined more accurately
			Platform: types.PlatformMacOS,
		}
		targets = append(targets, target)
	}

	return targets, nil
}

func (h *XcodeHelper) getSchemesFromXcodebuild(projectPath string) ([]XcodeScheme, error) {
	args := []string{"-list", "-json"}

	if strings.HasSuffix(projectPath, ".xcworkspace") {
		args = append(args, "-workspace", projectPath)
	} else {
		args = append(args, "-project", projectPath)
	}

	cmd := exec.Command("xcodebuild", args...)
	cmd.Dir = h.projectRoot

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list schemes: %w", err)
	}

	var listOutput struct {
		Project struct {
			Schemes []string `json:"schemes"`
		} `json:"project"`
	}

	if err := json.Unmarshal(output, &listOutput); err != nil {
		return nil, fmt.Errorf("failed to parse scheme list: %w", err)
	}

	var schemes []XcodeScheme
	for _, schemeName := range listOutput.Project.Schemes {
		scheme := XcodeScheme{
			Name:        schemeName,
			Target:      schemeName, // Often the same
			BuildConfig: "Debug",
			IsShared:    true,
		}
		schemes = append(schemes, scheme)
	}

	return schemes, nil
}

func (h *XcodeHelper) getBuildCommand(projectPath, target, configuration string) string {
	if strings.HasSuffix(projectPath, ".xcworkspace") {
		return fmt.Sprintf("xcodebuild -workspace %s -scheme %s -configuration %s build",
			projectPath, target, configuration)
	}
	return fmt.Sprintf("xcodebuild -project %s -target %s -configuration %s build",
		projectPath, target, configuration)
}

func (h *XcodeHelper) getTestCommand(projectPath, target string) string {
	if strings.HasSuffix(projectPath, ".xcworkspace") {
		return fmt.Sprintf("xcodebuild -workspace %s -scheme %s test", projectPath, target)
	}
	return fmt.Sprintf("xcodebuild -project %s -target %s test", projectPath, target)
}

func (h *XcodeHelper) getLaunchCommand(target XcodeTarget) string {
	if target.Platform == types.PlatformIOS {
		return fmt.Sprintf("xcrun simctl launch booted %s", target.BundleID)
	}
	if target.OutputPath != "" {
		return fmt.Sprintf("open %s", target.OutputPath)
	}
	return ""
}

// IsXcodeAvailable checks if Xcode tools are available
func IsXcodeAvailable() bool {
	_, err := exec.LookPath("xcodebuild")
	return err == nil
}

// GetXcodeVersion returns the installed Xcode version
func GetXcodeVersion() (string, error) {
	cmd := exec.Command("xcodebuild", "-version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse version from output like "Xcode 14.2\nBuild version 14C18"
	re := regexp.MustCompile(`Xcode\s+([0-9.]+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("could not parse Xcode version from output: %s", string(output))
}
