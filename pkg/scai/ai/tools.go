package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/scai/sessionchanges"
)

// GetExtendedToolSchemas returns all registered tools including the extended set with their schemas and handlers.
func GetExtendedToolSchemas(projectRoot string) []ToolSchema {
	allTools := []ToolSchema{
		{
			Name:        "read_file",
			Description: "Read the contents of a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_file": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to read (relative to project root).",
					},
					"should_read_entire_file": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to read the entire file or just a portion",
					},
					"start_line_one_indexed": map[string]interface{}{
						"type":        "number",
						"description": "The line number to start reading from (1-indexed)",
					},
					"end_line_one_indexed_inclusive": map[string]interface{}{
						"type":        "number",
						"description": "The line number to end reading at (inclusive, 1-indexed)",
					},
				},
				"required": []string{"target_file"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				targetFile, _ := args["target_file"].(string)
				shouldReadEntireFile, _ := args["should_read_entire_file"].(bool)
				startLine, _ := args["start_line_one_indexed"].(float64)
				endLine, _ := args["end_line_one_indexed_inclusive"].(float64)

				absPath := filepath.Join(projectRoot, targetFile)

				data, err := os.ReadFile(absPath)
				if err != nil {
					return nil, err
				}

				lines := strings.Split(string(data), "\n")
				totalLines := len(lines)

				if shouldReadEntireFile {
					return map[string]interface{}{
						"content":     string(data),
						"total_lines": totalLines,
						"file_path":   targetFile,
					}, nil
				}

				start := int(startLine) - 1
				end := int(endLine)
				if start < 0 {
					start = 0
				}
				if end > totalLines {
					end = totalLines
				}

				selectedLines := lines[start:end]
				content := strings.Join(selectedLines, "\n")

				return map[string]interface{}{
					"content":     content,
					"start_line":  int(startLine),
					"end_line":    int(endLine),
					"total_lines": totalLines,
					"file_path":   targetFile,
				}, nil
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string", "description": "Path to the file (relative to project root)"},
					"content": map[string]interface{}{"type": "string", "description": "Content to write"},
				},
				"required": []string{"path", "content"},
			},
			Handler: func(funcName string, args map[string]interface{}) (interface{}, error) {
				path, _ := args["path"].(string)
				content, _ := args["content"].(string)
				absPath := filepath.Join(projectRoot, path)
				if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
					return nil, err
				}
				// Register the change with the global tracker for backward compatibility
				sessionchanges.RegisterChange(absPath)
				return map[string]interface{}{"result": "file written successfully"}, nil
			},
		},
		{
			Name:        "codebase_search",
			Description: "Find snippets of code from the codebase most relevant to the search query.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query to find relevant code.",
					},
					"target_directories": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Glob patterns for directories to search over",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used.",
					},
				},
				"required": []string{"query"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				query, _ := args["query"].(string)
				return map[string]interface{}{
					"results": []map[string]interface{}{
						{
							"file":    "placeholder.go",
							"content": "Semantic search not yet implemented",
							"score":   0.0,
						},
					},
					"query": query,
				}, nil
			},
		},
		{
			Name:        "run_terminal_cmd",
			Description: "Run a terminal command on behalf of the user.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The terminal command to execute",
					},
					"is_background": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the command should be run in the background",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this command needs to be run.",
					},
				},
				"required": []string{"command", "is_background"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				command, _ := args["command"].(string)
				isBackground, _ := args["is_background"].(bool)

				parts := strings.Fields(command)
				if len(parts) == 0 {
					return nil, fmt.Errorf("empty command")
				}

				cmd := exec.CommandContext(context.Background(), parts[0], parts[1:]...)
				cmd.Dir = projectRoot

				if isBackground {
					err := cmd.Start()
					if err != nil {
						return nil, err
					}
					return map[string]interface{}{
						"result":     "Command started in background",
						"pid":        cmd.Process.Pid,
						"command":    command,
						"background": true,
					}, nil
				} else {
					output, err := cmd.CombinedOutput()
					if err != nil {
						return map[string]interface{}{
							"result":  string(output),
							"error":   err.Error(),
							"command": command,
						}, nil
					}
					return map[string]interface{}{
						"result":  string(output),
						"command": command,
					}, nil
				}
			},
		},
		{
			Name:        "list_dir",
			Description: "List the contents of a directory.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"relative_workspace_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to list contents of, relative to the workspace root.",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used.",
					},
				},
				"required": []string{"relative_workspace_path"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				relativePath, _ := args["relative_workspace_path"].(string)
				fullPath := filepath.Join(projectRoot, relativePath)

				entries, err := os.ReadDir(fullPath)
				if err != nil {
					return nil, err
				}

				var items []map[string]interface{}
				for _, entry := range entries {
					info, err := entry.Info()
					if err != nil {
						continue
					}
					items = append(items, map[string]interface{}{
						"name":   entry.Name(),
						"is_dir": entry.IsDir(),
						"size":   info.Size(),
					})
				}

				return map[string]interface{}{
					"path":  relativePath,
					"items": items,
				}, nil
			},
		},
		{
			Name:        "grep_search",
			Description: "Search for text patterns using ripgrep.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The regex pattern to search for",
					},
					"include_pattern": map[string]interface{}{
						"type":        "string",
						"description": "Glob pattern for files to include",
					},
					"exclude_pattern": map[string]interface{}{
						"type":        "string",
						"description": "Glob pattern for files to exclude",
					},
					"case_sensitive": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the search should be case sensitive",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used.",
					},
				},
				"required": []string{"query"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				query, _ := args["query"].(string)
				includePattern, _ := args["include_pattern"].(string)
				excludePattern, _ := args["exclude_pattern"].(string)
				caseSensitive, _ := args["case_sensitive"].(bool)

				cmdArgs := []string{"rg", "--json"}
				if !caseSensitive {
					cmdArgs = append(cmdArgs, "-i")
				}
				if includePattern != "" {
					cmdArgs = append(cmdArgs, "-g", includePattern)
				}
				if excludePattern != "" {
					cmdArgs = append(cmdArgs, "-g", "!"+excludePattern)
				}
				cmdArgs = append(cmdArgs, query, projectRoot)

				cmd := exec.CommandContext(context.Background(), cmdArgs[0], cmdArgs[1:]...)
				output, err := cmd.CombinedOutput()
				if err != nil {
					if strings.Contains(err.Error(), "exit status 1") {
						return map[string]interface{}{
							"results": []map[string]interface{}{},
							"query":   query,
						}, nil
					}
					return nil, err
				}

				var results []map[string]interface{}
				lines := strings.Split(string(output), "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}
					var result map[string]interface{}
					if err := json.Unmarshal([]byte(line), &result); err != nil {
						continue
					}
					results = append(results, result)
				}

				if len(results) > 50 {
					results = results[:50]
				}

				return map[string]interface{}{
					"results": results,
					"query":   query,
				}, nil
			},
		},
		{
			Name:        "edit_file",
			Description: "Edit an existing file or create a new file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_file": map[string]interface{}{
						"type":        "string",
						"description": "The target file to modify (relative to project root).",
					},
					"instructions": map[string]interface{}{
						"type":        "string",
						"description": "A single sentence instruction describing what you are going to do.",
					},
					"code_edit": map[string]interface{}{
						"type":        "string",
						"description": "The code to edit or create.",
					},
				},
				"required": []string{"target_file", "instructions", "code_edit"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				targetFile, _ := args["target_file"].(string)
				instructions, _ := args["instructions"].(string)
				codeEdit, _ := args["code_edit"].(string)

				absPath := filepath.Join(projectRoot, targetFile)

				_, err := os.Stat(absPath)
				fileExists := err == nil

				if !fileExists {
					dir := filepath.Dir(absPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						return nil, err
					}
					if err := os.WriteFile(absPath, []byte(codeEdit), 0644); err != nil {
						return nil, err
					}
					sessionchanges.RegisterChange(absPath)
					return map[string]interface{}{
						"result":    "File created successfully",
						"file_path": targetFile,
						"created":   true,
					}, nil
				}

				if err := os.WriteFile(absPath, []byte(codeEdit), 0644); err != nil {
					return nil, err
				}
				sessionchanges.RegisterChange(absPath)

				return map[string]interface{}{
					"result":       "File edited successfully",
					"file_path":    targetFile,
					"instructions": instructions,
				}, nil
			},
		},
		{
			Name:        "search_replace",
			Description: "Search and replace text in a file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file you want to search and replace in.",
					},
					"old_string": map[string]interface{}{
						"type":        "string",
						"description": "The text to replace.",
					},
					"new_string": map[string]interface{}{
						"type":        "string",
						"description": "The edited text to replace the old_string.",
					},
				},
				"required": []string{"file_path", "old_string", "new_string"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				filePath, _ := args["file_path"].(string)
				oldString, _ := args["old_string"].(string)
				newString, _ := args["new_string"].(string)

				absPath := filepath.Join(projectRoot, filePath)

				data, err := os.ReadFile(absPath)
				if err != nil {
					return nil, err
				}

				content := string(data)
				if !strings.Contains(content, oldString) {
					return nil, fmt.Errorf("old_string not found in file")
				}

				newContent := strings.Replace(content, oldString, newString, 1)

				if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
					return nil, err
				}
				sessionchanges.RegisterChange(absPath)

				return map[string]interface{}{
					"result":    "Search and replace completed successfully",
					"file_path": filePath,
				}, nil
			},
		},
		{
			Name:        "file_search",
			Description: "Fast file search based on fuzzy matching against file path.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Fuzzy filename to search for",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used.",
					},
				},
				"required": []string{"query", "explanation"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				query, _ := args["query"].(string)

				var results []map[string]interface{}
				err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					if info.IsDir() {
						return nil
					}

					relPath, _ := filepath.Rel(projectRoot, path)
					if strings.Contains(strings.ToLower(relPath), strings.ToLower(query)) {
						results = append(results, map[string]interface{}{
							"path": relPath,
							"size": info.Size(),
						})
					}

					if len(results) >= 10 {
						return filepath.SkipAll
					}
					return nil
				})

				if err != nil {
					return nil, err
				}

				return map[string]interface{}{
					"results": results,
					"query":   query,
				}, nil
			},
		},
		{
			Name:        "delete_file",
			Description: "Deletes a file at the specified path.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_file": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to delete, relative to the workspace root.",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used.",
					},
				},
				"required": []string{"target_file"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				targetFile, _ := args["target_file"].(string)
				absPath := filepath.Join(projectRoot, targetFile)

				if _, err := os.Stat(absPath); os.IsNotExist(err) {
					return map[string]interface{}{
						"result": "File does not exist",
						"path":   targetFile,
					}, nil
				}

				if err := os.Remove(absPath); err != nil {
					return nil, err
				}

				return map[string]interface{}{
					"result": "File deleted successfully",
					"path":   targetFile,
				}, nil
			},
		},
		{
			Name:        "reapply",
			Description: "Calls a smarter model to apply the last edit to the specified file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_file": map[string]interface{}{
						"type":        "string",
						"description": "The relative path to the file to reapply the last edit to.",
					},
				},
				"required": []string{"target_file"},
			},
			Handler: func(projectRoot string, args map[string]interface{}) (interface{}, error) {
				targetFile, _ := args["target_file"].(string)

				return map[string]interface{}{
					"result":      "Reapply functionality not yet implemented",
					"target_file": targetFile,
				}, nil
			},
		},
	}

	return allTools
}
