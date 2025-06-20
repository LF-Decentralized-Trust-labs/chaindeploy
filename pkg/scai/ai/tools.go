package ai

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/scai/sessionchanges"
	"github.com/sahilm/fuzzy"
)

// GetExtendedToolSchemas returns all registered tools including the extended set with their schemas and handlers.
func (s *OpenAIChatService) GetExtendedToolSchemas(projectRoot string) []ToolSchema {
	allTools := []ToolSchema{
		{
			Name:        "read_file",
			Description: "Read the contents of a file. the output of this tool call will be the 1-indexed file contents from start_line_one_indexed to end_line_one_indexed_inclusive, together with a summary of the lines outside start_line_one_indexed and end_line_one_indexed_inclusive.\nNote that this call can view at most 250 lines at a time.\n\nWhen using this tool to gather information, it's your responsibility to ensure you have the COMPLETE context. Specifically, each time you call this command you should:\n1) Assess if the contents you viewed are sufficient to proceed with your task.\n2) Take note of where there are lines not shown.\n3) If the file contents you have viewed are insufficient, and you suspect they may be in lines not shown, proactively call the tool again to view those lines.\n4) When in doubt, call this tool again to gather more information. Remember that partial file views may miss critical dependencies, imports, or functionality.\n\nIn some cases, if reading a range of lines is not enough, you may choose to read the entire file.\nReading entire files is often wasteful and slow, especially for large files (i.e. more than a few hundred lines). So you should use this option sparingly.\nReading the entire file is not allowed in most cases. You are only allowed to read the entire file if it has been edited or manually attached to the conversation by the user.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_file": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to read (relative to project root).",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used, and how it contributes to the goal.",
					},
					"should_read_entire_file": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to read the entire file or just a portion",
					},
					"start_line_one_indexed": map[string]interface{}{
						"type":        "number",
						"description": "The line number to start reading from (1-indexed)",
					},
					"end_line_one_indexed": map[string]interface{}{
						"type":        "number",
						"description": "The line number to end reading at (inclusive, 1-indexed)",
					},
				},
				"required": []string{
					"target_file",
					"should_read_entire_file",
					"start_line_one_indexed",
					"end_line_one_indexed_inclusive",
				},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				targetFile, _ := args["target_file"].(string)
				shouldReadEntireFile, _ := args["should_read_entire_file"].(bool)
				startLine, _ := args["start_line_one_indexed"].(float64)
				endLine, _ := args["end_line_one_indexed"].(float64)

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
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				path, _ := args["path"].(string)
				content, _ := args["content"].(string)

				// Check if content is empty and return early
				if strings.TrimSpace(content) == "" {
					return map[string]interface{}{
						"result":    "No changes made - content is empty",
						"file_path": path,
						"skipped":   true,
					}, nil
				}

				absPath := filepath.Join(projectRoot, path)

				// Ensure directory exists
				dir := filepath.Dir(absPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return nil, err
				}

				if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
					return nil, err
				}
				// Register the change with the global tracker for backward compatibility
				sessionchanges.RegisterChange(absPath)
				return map[string]interface{}{"result": "file written successfully"}, nil
			},
		},
		// {
		// 	Name:        "codebase_search",
		// 	Description: "Find snippets of code from the codebase most relevant to the search query.",
		// 	Parameters: map[string]interface{}{
		// 		"type": "object",
		// 		"properties": map[string]interface{}{
		// 			"query": map[string]interface{}{
		// 				"type":        "string",
		// 				"description": "The search query to find relevant code.",
		// 			},
		// 			"target_directories": map[string]interface{}{
		// 				"type": "array",
		// 				"items": map[string]interface{}{
		// 					"type": "string",
		// 				},
		// 				"description": "Glob patterns for directories to search over",
		// 			},
		// 			"explanation": map[string]interface{}{
		// 				"type":        "string",
		// 				"description": "One sentence explanation as to why this tool is being used.",
		// 			},
		// 		},
		// 		"required": []string{"query"},
		// 	},
		// 	Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
		// 		query, _ := args["query"].(string)
		// 		return map[string]interface{}{
		// 			"results": []map[string]interface{}{
		// 				{
		// 					"file":    "placeholder.go",
		// 					"content": "Semantic search not yet implemented",
		// 					"score":   0.0,
		// 				},
		// 			},
		// 			"query": query,
		// 		}, nil
		// 	},
		// },
		// {
		// 	Name:        "run_terminal_cmd",
		// 	Description: "Run a terminal command on behalf of the user.",
		// 	Parameters: map[string]interface{}{
		// 		"type": "object",
		// 		"properties": map[string]interface{}{
		// 			"command": map[string]interface{}{
		// 				"type":        "string",
		// 				"description": "The terminal command to execute",
		// 			},
		// 			"is_background": map[string]interface{}{
		// 				"type":        "boolean",
		// 				"description": "Whether the command should be run in the background",
		// 			},
		// 			"explanation": map[string]interface{}{
		// 				"type":        "string",
		// 				"description": "One sentence explanation as to why this command needs to be run.",
		// 			},
		// 		},
		// 		"required": []string{"command", "is_background"},
		// 	},
		// 	Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
		// 		command, _ := args["command"].(string)
		// 		isBackground, _ := args["is_background"].(bool)

		// 		parts := strings.Fields(command)
		// 		if len(parts) == 0 {
		// 			return nil, fmt.Errorf("empty command")
		// 		}

		// 		cmd := exec.CommandContext(context.Background(), parts[0], parts[1:]...)
		// 		cmd.Dir = projectRoot

		// 		if isBackground {
		// 			err := cmd.Start()
		// 			if err != nil {
		// 				return nil, err
		// 			}
		// 			return map[string]interface{}{
		// 				"result":     "Command started in background",
		// 				"pid":        cmd.Process.Pid,
		// 				"command":    command,
		// 				"background": true,
		// 			}, nil
		// 		} else {
		// 			output, err := cmd.CombinedOutput()
		// 			if err != nil {
		// 				return map[string]interface{}{
		// 					"result":  string(output),
		// 					"error":   err.Error(),
		// 					"command": command,
		// 				}, nil
		// 			}
		// 			return map[string]interface{}{
		// 				"result":  string(output),
		// 				"command": command,
		// 			}, nil
		// 		}
		// 	},
		// },
		{
			Name:        "list_dir",
			Description: "List the contents of a directory. The quick tool to use for discovery, before using more targeted tools like semantic search or file reading. Useful to try to understand the file structure before diving deeper into specific files. Can be used to explore the codebase.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"relative_workspace_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to list contents of, relative to the workspace root.",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used, and how it contributes to the goal.",
					},
				},
				"required": []string{"relative_workspace_path"},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
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
			Description: "Fast text-based regex search that finds exact pattern matches within files or directories, utilizing the ripgrep command for efficient searching.\nResults will be formatted in the style of ripgrep and can be configured to include line numbers and content.\nTo avoid overwhelming output, the results are capped at 50 matches.\nUse the include or exclude patterns to filter the search scope by file type or specific paths.\n\nThis is best for finding exact text matches or regex patterns.\nMore precise than semantic search for finding specific strings or patterns.\nThis is preferred over semantic search when we know the exact symbol/function name/etc. to search in some set of directories/file types.",
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
						"description": "One sentence explanation as to why this tool is being used, and how it contributes to the goal.",
					},
				},
				"required": []string{"query"},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				query, _ := args["query"].(string)
				includePattern, _ := args["include_pattern"].(string)
				excludePattern, _ := args["exclude_pattern"].(string)
				caseSensitive, _ := args["case_sensitive"].(bool)

				cmdArgs := []string{"rg", "--line-number", "--with-filename"}
				if !caseSensitive {
					cmdArgs = append(cmdArgs, "-i")
				}
				if includePattern != "" {
					cmdArgs = append(cmdArgs, "-g", fmt.Sprintf("**/%s", includePattern))
				}
				if excludePattern != "" {
					cmdArgs = append(cmdArgs, "-g", "!"+excludePattern)
				}
				cmdArgs = append(cmdArgs, query, projectRoot)
				s.Logger.Infof("grep_search cmd: %v", cmdArgs)
				cmd := exec.CommandContext(context.Background(), cmdArgs[0], cmdArgs[1:]...)
				output, err := cmd.CombinedOutput()
				if err != nil {
					if strings.Contains(err.Error(), "exit status 1") {
						return map[string]interface{}{
							"results": "No matches found",
							"query":   query,
						}, nil
					}
					s.Logger.Infof("grep_search error: %v, output: %s", err, string(output))
					return nil, err
				}

				// Parse the output and format it nicely
				lines := strings.Split(string(output), "\n")
				var formattedResults []string

				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}

					// Parse ripgrep output format: filepath:line:content
					parts := strings.SplitN(line, ":", 3)
					if len(parts) >= 3 {
						filePath := parts[0]
						lineNum := parts[1]
						content := parts[2]

						// Convert to relative path
						relPath, err := filepath.Rel(projectRoot, filePath)
						if err != nil {
							relPath = filePath
						}

						formattedResults = append(formattedResults, fmt.Sprintf("```%s:%s:%s\n%s\n```", lineNum, lineNum, relPath, content))
					}
				}

				if len(formattedResults) > 50 {
					formattedResults = formattedResults[:50]
					formattedResults = append(formattedResults, "... (truncated to 50 results)")
				}

				return map[string]interface{}{
					"results": strings.Join(formattedResults, "\n"),
					"query":   query,
					"count":   len(formattedResults),
				}, nil
			},
		},
		{
			Name:        "edit_file",
			Description: "Use this tool to propose an edit to an existing file.\n\nThis will be read by a less intelligent model, which will quickly apply the edit. You should make it clear what the edit is, while also minimizing the unchanged code you write.\nWhen writing the edit, you should specify each edit in sequence, with the special comment `// ... existing code ...` to represent unchanged code in between edited lines.\n\nFor example:\n\n```\n// ... existing code ...\nFIRST_EDIT\n// ... existing code ...\nSECOND_EDIT\n// ... existing code ...\nTHIRD_EDIT\n// ... existing code ...\n```\n\nYou should still bias towards repeating as few lines of the original file as possible to convey the change.\nBut, each edit should contain sufficient context of unchanged lines around the code you're editing to resolve ambiguity.\nDO NOT omit spans of pre-existing code (or comments) without using the `// ... existing code ...` comment to indicate its absence. If you omit the existing code comment, the model may inadvertently delete these lines.\nMake sure it is clear what the edit should be, and where it should be applied.\n\nYou should specify the following arguments before the others: [target_file]",
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
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				targetFile, _ := args["target_file"].(string)
				instructions, _ := args["instructions"].(string)
				codeEdit, _ := args["code_edit"].(string)

				// Check if content is empty and return early
				if strings.TrimSpace(codeEdit) == "" {
					return map[string]interface{}{
						"result":    "No changes made - content is empty",
						"file_path": targetFile,
						"skipped":   true,
					}, nil
				}

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
			Name:        "file_search",
			Description: "Fast file search based on fuzzy matching against file path. Use if you know part of the file path but don't know where it's located exactly. Response will be capped to 10 results. Make your query more specific if need to filter results further.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Fuzzy filename to search for",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used, and how it contributes to the goal.",
					},
				},
				"required": []string{"query", "explanation"},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				query, _ := args["query"].(string)
				query = strings.TrimSpace(query)

				if query == "" {
					return map[string]interface{}{
						"results": []map[string]interface{}{},
						"query":   query,
						"error":   "Empty query provided",
					}, nil
				}

				// Collect all files
				var files []string
				err := filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil // Skip files we can't access
					}

					// Skip directories and hidden files
					if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
						return nil
					}

					// Skip common build and cache directories
					relPath, _ := filepath.Rel(projectRoot, path)
					if shouldSkipPath(relPath) {
						return nil
					}

					files = append(files, relPath)
					return nil
				})

				if err != nil {
					return nil, err
				}

				// Use fuzzy search
				matches := fuzzy.Find(query, files)

				// Convert to results format
				var results []map[string]interface{}
				for i, match := range matches {
					if i >= 10 { // Limit to 10 results
						break
					}

					// Get file info
					absPath := filepath.Join(projectRoot, match.Str)
					info, err := os.Stat(absPath)
					if err != nil {
						continue
					}

					results = append(results, map[string]interface{}{
						"path":  match.Str,
						"size":  info.Size(),
						"score": match.Score,
					})
				}

				return map[string]interface{}{
					"results": results,
					"query":   query,
					"count":   len(results),
				}, nil
			},
		},
		{
			Name:        "delete_file",
			Description: "Deletes a file at the specified path. The operation will fail gracefully if:\n    - The file doesn't exist\n    - The operation is rejected for security reasons\n    - The file cannot be deleted",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_file": map[string]interface{}{
						"type":        "string",
						"description": "The path of the file to delete, relative to the workspace root.",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used, and how it contributes to the goal.",
					},
				},
				"required": []string{"target_file"},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
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
		// {
		// 	Name:        "reapply",
		// 	Description: "Calls a smarter model to apply the last edit to the specified file.\nUse this tool immediately after the result of an edit_file tool call ONLY IF the diff is not what you expected, indicating the model applying the changes was not smart enough to follow your instructions.",
		// 	Parameters: map[string]interface{}{
		// 		"type": "object",
		// 		"properties": map[string]interface{}{
		// 			"target_file": map[string]interface{}{
		// 				"type":        "string",
		// 				"description": "The relative path to the file to reapply the last edit to.",
		// 			},
		// 		},
		// 		"required": []string{"target_file"},
		// 	},
		// 	Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
		// 		targetFile, _ := args["target_file"].(string)

		// 		return map[string]interface{}{
		// 			"result":      "Reapply functionality not yet implemented",
		// 			"target_file": targetFile,
		// 		}, nil
		// 	},
		// },
	}

	return allTools
}

// shouldSkipPath checks if a path should be skipped during file search
func shouldSkipPath(path string) bool {
	// Common directories to skip
	skipDirs := []string{
		"node_modules", ".git", ".vscode", ".idea", "dist", "build", "target",
		"vendor", ".cache", ".tmp", "coverage", ".nyc_output", "logs",
		"bin", "obj", ".vs", "__pycache__", ".pytest_cache", ".mypy_cache",
	}

	pathLower := strings.ToLower(path)
	for _, skipDir := range skipDirs {
		if strings.Contains(pathLower, skipDir) {
			return true
		}
	}

	// Skip files with certain extensions
	skipExts := []string{".log", ".tmp", ".temp", ".cache", ".lock"}
	for _, ext := range skipExts {
		if strings.HasSuffix(pathLower, ext) {
			return true
		}
	}

	return false
}
