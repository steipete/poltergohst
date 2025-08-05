package types_test

import (
	"encoding/json"
	"testing"

	"github.com/poltergeist/poltergeist/pkg/types"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		check   func(t *testing.T, target types.Target)
	}{
		{
			name: "executable target",
			json: `{
				"name": "test-exe",
				"type": "executable",
				"buildCommand": "go build",
				"watchPaths": ["*.go"],
				"outputPath": "bin/test"
			}`,
			wantErr: false,
			check: func(t *testing.T, target types.Target) {
				if target.GetName() != "test-exe" {
					t.Errorf("expected name 'test-exe', got %s", target.GetName())
				}
				if target.GetType() != types.TargetTypeExecutable {
					t.Errorf("expected type executable, got %s", target.GetType())
				}
				if !target.IsEnabled() {
					t.Error("expected target to be enabled")
				}
			},
		},
		{
			name: "app bundle target",
			json: `{
				"name": "MyApp",
				"type": "app-bundle",
				"buildCommand": "xcodebuild",
				"watchPaths": ["**/*.swift"],
				"platform": "macos",
				"bundleId": "com.example.app"
			}`,
			wantErr: false,
			check: func(t *testing.T, target types.Target) {
				if target.GetType() != types.TargetTypeAppBundle {
					t.Errorf("expected type app-bundle, got %s", target.GetType())
				}
			},
		},
		{
			name: "library target",
			json: `{
				"name": "mylib",
				"type": "library",
				"buildCommand": "make lib",
				"watchPaths": ["src/**/*.c"],
				"outputPath": "lib/mylib.a",
				"libraryType": "static"
			}`,
			wantErr: false,
			check: func(t *testing.T, target types.Target) {
				if target.GetType() != types.TargetTypeLibrary {
					t.Errorf("expected type library, got %s", target.GetType())
				}
			},
		},
		{
			name: "docker target",
			json: `{
				"name": "app-image",
				"type": "docker",
				"buildCommand": "docker build",
				"watchPaths": ["Dockerfile", "src/**"],
				"imageName": "myapp:latest"
			}`,
			wantErr: false,
			check: func(t *testing.T, target types.Target) {
				if target.GetType() != types.TargetTypeDocker {
					t.Errorf("expected type docker, got %s", target.GetType())
				}
			},
		},
		{
			name: "disabled target",
			json: `{
				"name": "disabled",
				"type": "custom",
				"buildCommand": "echo test",
				"watchPaths": ["*"],
				"enabled": false
			}`,
			wantErr: false,
			check: func(t *testing.T, target types.Target) {
				if target.IsEnabled() {
					t.Error("expected target to be disabled")
				}
			},
		},
		{
			name:    "invalid json",
			json:    `{"invalid": json}`,
			wantErr: true,
		},
		{
			name:    "unknown type",
			json:    `{"name": "test", "type": "unknown", "watchPaths": []}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := types.ParseTarget([]byte(tt.json))
			
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTarget() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr && tt.check != nil {
				tt.check(t, target)
			}
		})
	}
}

func TestTargetDefaults(t *testing.T) {
	json := `{
		"name": "test",
		"type": "executable",
		"buildCommand": "build",
		"watchPaths": ["src"],
		"outputPath": "out"
	}`
	
	target, err := types.ParseTarget([]byte(json))
	if err != nil {
		t.Fatalf("failed to parse target: %v", err)
	}
	
	// Test default values
	if target.GetSettlingDelay() != 1000 {
		t.Errorf("expected default settling delay 1000, got %d", target.GetSettlingDelay())
	}
	
	if target.GetMaxRetries() != 3 {
		t.Errorf("expected default max retries 3, got %d", target.GetMaxRetries())
	}
	
	if target.GetBackoffMultiplier() != 2.0 {
		t.Errorf("expected default backoff multiplier 2.0, got %f", target.GetBackoffMultiplier())
	}
	
	if target.GetDebounceInterval() != 100 {
		t.Errorf("expected default debounce interval 100, got %d", target.GetDebounceInterval())
	}
}

func TestPoltergeistConfig(t *testing.T) {
	configJSON := `{
		"version": "1.0",
		"projectType": "go",
		"targets": [
			{
				"name": "main",
				"type": "executable",
				"buildCommand": "go build",
				"watchPaths": ["*.go"],
				"outputPath": "main"
			}
		],
		"watchman": {
			"useDefaultExclusions": true,
			"excludeDirs": ["vendor"],
			"settlingDelay": 500
		}
	}`
	
	var config types.PoltergeistConfig
	err := json.Unmarshal([]byte(configJSON), &config)
	if err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}
	
	if config.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", config.Version)
	}
	
	if config.ProjectType != types.ProjectType("go") {
		t.Errorf("expected project type go, got %s", config.ProjectType)
	}
	
	if len(config.Targets) != 1 {
		t.Errorf("expected 1 target, got %d", len(config.Targets))
	}
	
	if config.Watchman == nil {
		t.Error("expected watchman config to be set")
	} else {
		if config.Watchman.SettlingDelay != 500 {
			t.Errorf("expected settling delay 500, got %d", config.Watchman.SettlingDelay)
		}
	}
}

func TestBuildStatus(t *testing.T) {
	statuses := []types.BuildStatus{
		types.BuildStatusIdle,
		types.BuildStatusQueued,
		types.BuildStatusBuilding,
		types.BuildStatusSucceeded,
		types.BuildStatusFailed,
		types.BuildStatusCancelled,
	}
	
	for _, status := range statuses {
		// Ensure status can be marshaled/unmarshaled
		data, err := json.Marshal(status)
		if err != nil {
			t.Errorf("failed to marshal status %s: %v", status, err)
		}
		
		var unmarshaled types.BuildStatus
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Errorf("failed to unmarshal status: %v", err)
		}
		
		if unmarshaled != status {
			t.Errorf("status mismatch: expected %s, got %s", status, unmarshaled)
		}
	}
}

func BenchmarkParseTarget(b *testing.B) {
	json := []byte(`{
		"name": "bench",
		"type": "executable",
		"buildCommand": "go build",
		"watchPaths": ["*.go"],
		"outputPath": "bench"
	}`)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := types.ParseTarget(json)
		if err != nil {
			b.Fatal(err)
		}
	}
}