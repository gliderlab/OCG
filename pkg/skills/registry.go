// Skills registry - manages loaded skills
package skills

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Registry holds all loaded skills
type Registry struct {
	skills    map[string]*Skill
	skillsDir string
}

// NewRegistry creates a new skills registry
func NewRegistry(dir string) *Registry {
	return &Registry{
		skills:    make(map[string]*Skill),
		skillsDir: dir,
	}
}

// Load loads all skills from the configured directory
func (r *Registry) Load() error {
	if r.skillsDir == "" {
		return fmt.Errorf("skills dir not set")
	}

	// Check if directory exists
	if _, err := os.Stat(r.skillsDir); os.IsNotExist(err) {
		log.Printf("[skills] Directory does not exist: %s", r.skillsDir)
		return nil
	}

	skills, err := LoadSkillsFromDir(r.skillsDir)
	if err != nil {
		return fmt.Errorf("load skills: %w", err)
	}

	for _, s := range skills {
		r.skills[s.Name] = s
		log.Printf("[skills] Loaded: %s (%s)", s.Name, s.Description)
	}

	log.Printf("[skills] Total loaded: %d from %s", len(r.skills), r.skillsDir)
	return nil
}

// Get returns a skill by name
func (r *Registry) Get(name string) *Skill {
	return r.skills[name]
}

// List returns all skills
func (r *Registry) List() []*Skill {
	var result []*Skill
	for _, s := range r.skills {
		result = append(result, s)
	}
	return result
}

// FilterAvailable returns only skills that meet requirements
func (r *Registry) FilterAvailable() []*Skill {
	var result []*Skill
	for _, s := range r.skills {
		ok, _ := CheckRequirements(s)
		if ok {
			result = append(result, s)
		}
	}
	return result
}

// CheckRequirements checks if a skill's requirements are met
func CheckRequirements(s *Skill) (bool, []string) {
	var missing []string

	// Check OS requirement
	if len(s.OS) > 0 {
		currentOS := runtime.GOOS
		supported := false
		for _, os := range s.OS {
			if os == currentOS {
				supported = true
				break
			}
		}
		if !supported {
			return false, []string{fmt.Sprintf("OS %s not supported", currentOS)}
		}
	}

	// Check binary requirements
	for _, bin := range s.BinRequires {
		if !isBinaryAvailable(bin) {
			missing = append(missing, fmt.Sprintf("bin:%s", bin))
		}
	}

	// Check anyBins (at least one must exist)
	if len(s.Metadata.Requires.AnyBins) > 0 {
		anyAvailable := false
		for _, bin := range s.Metadata.Requires.AnyBins {
			if isBinaryAvailable(bin) {
				anyAvailable = true
				break
			}
		}
		if !anyAvailable {
			missing = append(missing, fmt.Sprintf("anyBins:%v", s.Metadata.Requires.AnyBins))
		}
	}

	// Check env requirements
	for _, env := range s.EnvRequires {
		if os.Getenv(env) == "" {
			missing = append(missing, fmt.Sprintf("env:%s", env))
		}
	}

	return len(missing) == 0, missing
}

// isBinaryAvailable checks if a binary exists on PATH
func isBinaryAvailable(name string) bool {
	path, err := execLookPath(name)
	if err != nil {
		return false
	}
	if path == "" {
		return false
	}
	// Check if it's executable
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	if !fi.Mode().IsRegular() {
		return false
	}
	
	// On Unix, check execute bits
	// On Windows, check file extension or try to run
	if runtime.GOOS == "windows" {
		// Windows: check if it has an executable extension
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".exe" || ext == ".bat" || ext == ".cmd" || ext == ".com"
	}
	
	// Unix: check execute bits
	return fi.Mode()&0111 != 0
}

// execLookPath is a wrapper for testing
var execLookPath = func(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(path)
}

// DefaultSkillsDirs returns default skills directories to search
func DefaultSkillsDirs() []string {
	var dirs []string

	// Workspace skills: ./skills
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(cwd, "skills"))
	}

	// Home directory skills: ~/.openclaw/skills
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".openclaw", "skills"))
	}

	// Built-in skills (for development)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		// Check ../skills relative to binary
		dirs = append(dirs, filepath.Join(exeDir, "..", "skills"))
		// Check ../../skills
		dirs = append(dirs, filepath.Join(exeDir, "..", "..", "skills"))
	}

	return dirs
}
