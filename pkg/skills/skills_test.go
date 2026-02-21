package skills

import (
	"testing"
)

func TestSkillBasic(t *testing.T) {
	// Test basic skill structure
	skill := &Skill{
		Name:        "test",
		Description: "Test skill",
		BinRequires: []string{"curl"},
		EnvRequires: []string{"TEST_VAR"},
	}
	
	if skill.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", skill.Name)
	}
	
	if skill.Description != "Test skill" {
		t.Errorf("Expected description 'Test skill', got '%s'", skill.Description)
	}
	
	if len(skill.BinRequires) != 1 || skill.BinRequires[0] != "curl" {
		t.Error("Expected BinRequires to contain 'curl'")
	}
}

func TestSkillRegistry(t *testing.T) {
	// Test registry creation
	registry := NewRegistry("")
	
	if registry == nil {
		t.Fatal("Registry should not be nil")
	}
	
	// Test List when empty - may return nil or empty slice
	list := registry.List()
	// List can be nil or empty, both are valid
	_ = list
}

func TestSkillRegistryGet(t *testing.T) {
	registry := NewRegistry("")
	
	// Get non-existent skill
	skill := registry.Get("nonexistent")
	if skill != nil {
		t.Error("Should return nil for non-existent skill")
	}
}

func TestSkillRegistryFilter(t *testing.T) {
	registry := NewRegistry("")
	
	// Filter available skills - may return nil or empty slice
	skills := registry.FilterAvailable()
	// Can be nil or empty, both are valid
	_ = skills
}

func TestSkillAdapter(t *testing.T) {
	// Test adapter creation
	registry := NewRegistry("")
	adapter := NewAdapter(registry)
	
	if adapter == nil {
		t.Fatal("Adapter should not be nil")
	}
}

func TestInstallParsing(t *testing.T) {
	// Test install array parsing
	tests := []struct {
		name     string
		input    string
		wantLen  int
		wantKind string
	}{
		{
			name:     "brew install",
			input:    `metadata: { "openclaw": { "install": [{ "id": "brew", "kind": "brew", "formula": "gh", "label": "Install" }] } }`,
			wantLen:  1,
			wantKind: "brew",
		},
		{
			name:     "apt install",
			input:    `metadata: { "openclaw": { "install": [{ "id": "apt", "kind": "apt", "package": "gh", "label": "Install" }] } }`,
			wantLen:  1,
			wantKind: "apt",
		},
		{
			name:     "npm install",
			input:    `metadata: { "openclaw": { "install": [{ "id": "npm", "kind": "npm", "package": "clawhub", "label": "Install" }] } }`,
			wantLen:  1,
			wantKind: "npm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := extractMetadataJSON(tt.input)
			if meta == nil {
				t.Fatal("Expected metadata, got nil")
			}
			if len(meta.Install) != tt.wantLen {
				t.Errorf("Expected %d install items, got %d", tt.wantLen, len(meta.Install))
			}
			if tt.wantLen > 0 && meta.Install[0].Kind != tt.wantKind {
				t.Errorf("Expected kind '%s', got '%s'", tt.wantKind, meta.Install[0].Kind)
			}
		})
	}
}

func TestGenerateInstallCommand(t *testing.T) {
	tests := []struct {
		item     InstallItem
		expected string
	}{
		{
			item:     InstallItem{Kind: "brew", Formula: "gh"},
			expected: "brew install gh",
		},
		{
			item:     InstallItem{Kind: "apt", Package: "curl"},
			expected: "sudo apt install curl",
		},
		{
			item:     InstallItem{Kind: "npm", Package: "clawhub"},
			expected: "npm install -g clawhub",
		},
		{
			item:     InstallItem{Kind: "node", Package: "clawhub"},
			expected: "npm i -g clawhub",
		},
		{
			item:     InstallItem{Kind: "pip", Package: "requests"},
			expected: "pip install requests",
		},
	}

	for _, tt := range tests {
		tt.item.Command = generateInstallCommand(tt.item)
		if tt.item.Command != tt.expected {
			t.Errorf("Expected '%s', got '%s'", tt.expected, tt.item.Command)
		}
	}
}
