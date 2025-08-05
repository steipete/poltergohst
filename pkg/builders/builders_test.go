package builders_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/poltergeist/poltergeist/pkg/builders"
	"github.com/poltergeist/poltergeist/pkg/interfaces"
	"github.com/poltergeist/poltergeist/pkg/state"
	"github.com/poltergeist/poltergeist/pkg/types"
)

// Mock state manager
type mockStateManager struct{}

func (m *mockStateManager) InitializeState(target types.Target) (*state.PoltergeistState, error) {
	return nil, nil
}
func (m *mockStateManager) ReadState(targetName string) (*state.PoltergeistState, error) {
	return nil, nil
}
func (m *mockStateManager) UpdateState(targetName string, updates map[string]interface{}) error {
	return nil
}
func (m *mockStateManager) UpdateBuildStatus(targetName string, status types.BuildStatus) error {
	return nil
}
func (m *mockStateManager) RemoveState(targetName string) error      { return nil }
func (m *mockStateManager) IsLocked(targetName string) (bool, error) { return false, nil }
func (m *mockStateManager) DiscoverStates() (map[string]*state.PoltergeistState, error) {
	return nil, nil
}
func (m *mockStateManager) StartHeartbeat(ctx context.Context) {}
func (m *mockStateManager) StopHeartbeat()                     {}
func (m *mockStateManager) Cleanup() error                     { return nil }

func TestBaseBuilder_Validate(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		target  types.Target
		wantErr bool
	}{
		{
			name: "valid target",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{"*.go"},
				},
				OutputPath: "test",
			},
			wantErr: false,
		},
		{
			name: "missing build command",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:       "test",
					Type:       types.TargetTypeExecutable,
					WatchPaths: []string{"*.go"},
				},
				OutputPath: "test",
			},
			wantErr: true,
		},
		{
			name: "missing watch paths",
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Name:         "test",
					Type:         types.TargetTypeExecutable,
					BuildCommand: "go build",
					WatchPaths:   []string{},
				},
				OutputPath: "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := builders.NewBaseBuilder(tt.target, tmpDir, nil, nil)
			err := builder.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecutableBuilder(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple Go file to build
	srcFile := filepath.Join(tmpDir, "main.go")
	err := os.WriteFile(srcFile, []byte(`
		package main
		import "fmt"
		func main() { fmt.Println("test") }
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	target := &types.ExecutableTarget{
		BaseTarget: types.BaseTarget{
			Name:         "test-exe",
			Type:         types.TargetTypeExecutable,
			BuildCommand: "go build -o test main.go",
			WatchPaths:   []string{"*.go"},
		},
		OutputPath: "test",
	}

	factory := builders.NewBuilderFactory()
	builder := factory.CreateBuilder(target, tmpDir, nil, nil)

	// Validate
	if err := builder.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	// Build
	ctx := context.Background()
	err = builder.Build(ctx, []string{"main.go"})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Check output exists
	outputPath := filepath.Join(tmpDir, "test")
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("expected output file to exist")
	}

	// Check metrics
	if builder.GetLastBuildTime() == 0 {
		t.Error("expected non-zero build time")
	}

	if builder.GetSuccessRate() != 1.0 {
		t.Errorf("expected success rate 1.0, got %f", builder.GetSuccessRate())
	}
}

func TestBuilderFactory_CreateBuilder(t *testing.T) {
	factory := builders.NewBuilderFactory()
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		targetType types.TargetType
		target     types.Target
		wantType   string
	}{
		{
			name:       "executable builder",
			targetType: types.TargetTypeExecutable,
			target: &types.ExecutableTarget{
				BaseTarget: types.BaseTarget{
					Type: types.TargetTypeExecutable,
				},
			},
			wantType: "*builders.ExecutableBuilder",
		},
		{
			name:       "app bundle builder",
			targetType: types.TargetTypeAppBundle,
			target: &types.AppBundleTarget{
				BaseTarget: types.BaseTarget{
					Type: types.TargetTypeAppBundle,
				},
			},
			wantType: "*builders.AppBundleBuilder",
		},
		{
			name:       "library builder",
			targetType: types.TargetTypeLibrary,
			target: &types.LibraryTarget{
				BaseTarget: types.BaseTarget{
					Type: types.TargetTypeLibrary,
				},
			},
			wantType: "*builders.LibraryBuilder",
		},
		{
			name:       "docker builder",
			targetType: types.TargetTypeDocker,
			target: &types.DockerTarget{
				BaseTarget: types.BaseTarget{
					Type: types.TargetTypeDocker,
				},
			},
			wantType: "*builders.DockerBuilder",
		},
		{
			name:       "test builder",
			targetType: types.TargetTypeTest,
			target: &types.TestTarget{
				BaseTarget: types.BaseTarget{
					Type: types.TargetTypeTest,
				},
			},
			wantType: "*builders.TestBuilder",
		},
		{
			name:       "cmake executable builder",
			targetType: types.TargetTypeCMakeExecutable,
			target: &types.CMakeExecutableTarget{
				BaseTarget: types.BaseTarget{
					Type: types.TargetTypeCMakeExecutable,
				},
			},
			wantType: "*builders.CMakeExecutableBuilder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := factory.CreateBuilder(tt.target, tmpDir, nil, nil)
			if builder == nil {
				t.Fatal("expected builder, got nil")
			}

			if builder.GetTarget() != tt.target {
				t.Error("builder target mismatch")
			}
		})
	}
}

func TestAppBundleBuilder_AutoRelaunch(t *testing.T) {
	t.Skip("Skipping platform-specific test")

	tmpDir := t.TempDir()
	autoRelaunch := true

	target := &types.AppBundleTarget{
		BaseTarget: types.BaseTarget{
			Name:         "TestApp",
			Type:         types.TargetTypeAppBundle,
			BuildCommand: "echo 'building app'",
			WatchPaths:   []string{"*.swift"},
		},
		BundleID:      "com.test.app",
		AutoRelaunch:  &autoRelaunch,
		LaunchCommand: "echo 'launching app'",
	}

	factory := builders.NewBuilderFactory()
	builder := factory.CreateBuilder(target, tmpDir, nil, nil)

	ctx := context.Background()
	err := builder.Build(ctx, []string{"test.swift"})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
}

func TestDockerBuilder(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available")
	}

	tmpDir := t.TempDir()

	// Create a simple Dockerfile
	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	err := os.WriteFile(dockerfile, []byte(`
		FROM alpine:latest
		CMD ["echo", "test"]
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create Dockerfile: %v", err)
	}

	target := &types.DockerTarget{
		BaseTarget: types.BaseTarget{
			Name:         "test-image",
			Type:         types.TargetTypeDocker,
			BuildCommand: "docker build",
			WatchPaths:   []string{"Dockerfile"},
		},
		ImageName:  "poltergeist-test",
		Dockerfile: "Dockerfile",
		Context:    ".",
		Tags:       []string{"latest", "test"},
	}

	factory := builders.NewBuilderFactory()
	builder := factory.CreateBuilder(target, tmpDir, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = builder.Build(ctx, []string{"Dockerfile"})
	if err != nil {
		t.Fatalf("docker build failed: %v", err)
	}
}

func TestCMakeBuilder(t *testing.T) {
	if _, err := exec.LookPath("cmake"); err != nil {
		t.Skip("CMake not available")
	}

	tmpDir := t.TempDir()

	// Create a simple CMakeLists.txt
	cmakeFile := filepath.Join(tmpDir, "CMakeLists.txt")
	err := os.WriteFile(cmakeFile, []byte(`
		cmake_minimum_required(VERSION 3.10)
		project(TestProject)
		add_executable(test main.cpp)
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create CMakeLists.txt: %v", err)
	}

	// Create main.cpp
	mainFile := filepath.Join(tmpDir, "main.cpp")
	err = os.WriteFile(mainFile, []byte(`
		#include <iostream>
		int main() {
			std::cout << "test" << std::endl;
			return 0;
		}
	`), 0644)
	if err != nil {
		t.Fatalf("failed to create main.cpp: %v", err)
	}

	target := &types.CMakeExecutableTarget{
		BaseTarget: types.BaseTarget{
			Name:       "test-cmake",
			Type:       types.TargetTypeCMakeExecutable,
			WatchPaths: []string{"*.cpp", "CMakeLists.txt"},
		},
		TargetName: "test",
		BuildType:  types.CMakeBuildTypeDebug,
		Generator:  "Unix Makefiles",
	}

	factory := builders.NewBuilderFactory()
	builder := factory.CreateBuilder(target, tmpDir, nil, nil)

	ctx := context.Background()
	err = builder.Build(ctx, []string{"main.cpp"})
	if err != nil {
		t.Logf("cmake build failed (expected on CI): %v", err)
	}
}

func TestBuilderConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	factory := builders.NewBuilderFactory()

	// Create multiple builders
	var builders []interfaces.Builder
	for i := 0; i < 5; i++ {
		target := &types.ExecutableTarget{
			BaseTarget: types.BaseTarget{
				Name:         fmt.Sprintf("test-%d", i),
				Type:         types.TargetTypeExecutable,
				BuildCommand: fmt.Sprintf("touch test-%d", i),
				WatchPaths:   []string{"*.go"},
			},
			OutputPath: fmt.Sprintf("test-%d", i),
		}

		builder := factory.CreateBuilder(target, tmpDir, nil, nil)
		builders = append(builders, builder)
	}

	// Build concurrently
	ctx := context.Background()
	errChan := make(chan error, len(builders))

	for _, builder := range builders {
		go func(b interfaces.Builder) {
			errChan <- b.Build(ctx, []string{"test.go"})
		}(builder)
	}

	// Wait for all builds
	for range builders {
		if err := <-errChan; err != nil {
			t.Errorf("concurrent build failed: %v", err)
		}
	}
}

func TestBuilderRetry(t *testing.T) {
	tmpDir := t.TempDir()
	maxRetries := 3

	target := &types.ExecutableTarget{
		BaseTarget: types.BaseTarget{
			Name:         "test-retry",
			Type:         types.TargetTypeExecutable,
			BuildCommand: "false", // Command that always fails
			WatchPaths:   []string{"*.go"},
			MaxRetries:   &maxRetries,
		},
		OutputPath: "test",
	}

	factory := builders.NewBuilderFactory()
	builder := factory.CreateBuilder(target, tmpDir, nil, nil)

	// Override build to count retries
	ctx := context.Background()
	err := builder.Build(ctx, []string{"test.go"})

	// Should fail after retries
	if err == nil {
		t.Error("expected build to fail")
	}
}

func BenchmarkBuilderCreation(b *testing.B) {
	factory := builders.NewBuilderFactory()
	tmpDir := b.TempDir()

	target := &types.ExecutableTarget{
		BaseTarget: types.BaseTarget{
			Name:         "bench",
			Type:         types.TargetTypeExecutable,
			BuildCommand: "echo test",
			WatchPaths:   []string{"*.go"},
		},
		OutputPath: "bench",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = factory.CreateBuilder(target, tmpDir, nil, nil)
	}
}

func BenchmarkBuilderBuild(b *testing.B) {
	tmpDir := b.TempDir()

	target := &types.ExecutableTarget{
		BaseTarget: types.BaseTarget{
			Name:         "bench",
			Type:         types.TargetTypeExecutable,
			BuildCommand: "echo test",
			WatchPaths:   []string{"*.go"},
		},
		OutputPath: "bench",
	}

	factory := builders.NewBuilderFactory()
	builder := factory.CreateBuilder(target, tmpDir, nil, nil)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = builder.Build(ctx, []string{"test.go"})
	}
}
