package tools

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ReadFiles reads the contents of specified files.
func ReadFiles(args json.RawMessage) (string, error) {
	var input struct {
		Files []struct {
			Name        string `json:"name"`
			LineRanges  []struct {
				Start int `json:"start"`
				End   int `json:"end"`
			} `json:"line_ranges,omitempty"`
		} `json:"files"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("parse read_files args: %w", err)
	}

	var sb strings.Builder
	for _, f := range input.Files {
		data, err := os.ReadFile(f.Name)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Error reading %s: %v\n", f.Name, err))
			continue
		}
		lines := strings.Split(string(data), "\n")
		if len(f.LineRanges) == 0 {
			sb.WriteString(fmt.Sprintf("--- %s ---\n%s\n", f.Name, string(data)))
			continue
		}
		for _, lr := range f.LineRanges {
			start := lr.Start
			if start < 1 {
				start = 1
			}
			end := lr.End
			if end > len(lines) {
				end = len(lines)
			}
			sb.WriteString(fmt.Sprintf("--- %s (lines %d-%d) ---\n", f.Name, start, end))
			for i := start - 1; i < end && i < len(lines); i++ {
				sb.WriteString(fmt.Sprintf("%6d: %s\n", i+1, lines[i]))
			}
		}
	}
	return sb.String(), nil
}

// Grep searches for patterns in files.
func Grep(args json.RawMessage) (string, error) {
	var input struct {
		Queries []string `json:"queries"`
		Path    string   `json:"path,omitempty"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("parse grep args: %w", err)
	}

	searchDir := input.Path
	if searchDir == "" {
		searchDir = "."
	}

	var sb strings.Builder
	for _, query := range input.Queries {
		re, err := regexp.Compile(query)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Invalid regex %q: %v\n", query, err))
			continue
		}

		matches := 0
		err = filepath.WalkDir(searchDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "target" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if re.MatchString(line) {
					sb.WriteString(fmt.Sprintf("%s:%d: %s\n", path, i+1, strings.TrimSpace(line)))
					matches++
					if matches >= 50 {
						sb.WriteString("(truncated, 50+ matches)\n")
						return fmt.Errorf("limit reached")
					}
				}
			}
			return nil
		})
		if matches == 0 && err == nil {
			sb.WriteString(fmt.Sprintf("No matches for %q in %s\n", query, searchDir))
		}
	}
	return sb.String(), nil
}

// FileGlob finds files matching glob patterns.
func FileGlob(args json.RawMessage) (string, error) {
	var input struct {
		Patterns  []string `json:"patterns"`
		SearchDir string   `json:"search_dir,omitempty"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("parse file_glob args: %w", err)
	}

	searchDir := input.SearchDir
	if searchDir == "" {
		searchDir = "."
	}

	var sb strings.Builder
	for _, pattern := range input.Patterns {
		fullPattern := filepath.Join(searchDir, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Invalid pattern %q: %v\n", pattern, err))
			continue
		}
		if len(matches) == 0 {
			sb.WriteString(fmt.Sprintf("No files matching %q in %s\n", pattern, searchDir))
			continue
		}
		for _, m := range matches {
			sb.WriteString(m + "\n")
		}
	}
	return sb.String(), nil
}
