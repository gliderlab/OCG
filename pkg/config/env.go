// Package config provides common configuration utilities

package config

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

// ReadEnvConfig reads env.config (KEY=VALUE)
func ReadEnvConfig(path string) map[string]string {
	config := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return config
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		config[key] = value
	}
	return config
}

// WriteEnvConfig writes env.config (KEY=VALUE)
func WriteEnvConfig(path string, config map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		_, _ = fmt.Fprintf(f, "%s=%s\n", k, config[k])
	}
	return nil
}

// MergeEnvConfig reads existing config, merges updates, and writes back
func MergeEnvConfig(path string, updates map[string]string) error {
	config := ReadEnvConfig(path)
	for k, v := range updates {
		config[k] = v
	}
	return WriteEnvConfig(path, config)
}
