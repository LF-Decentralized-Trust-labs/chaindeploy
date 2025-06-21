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
		{
			Name:        "run_terminal_cmd",
			Description: "Run a terminal command in the project's Docker container. This tool executes commands inside the running project container, not on the host system.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The terminal command to execute in the project container",
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
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				command, _ := args["command"].(string)
				isBackground, _ := args["is_background"].(bool)

				// Extract project slug from project root path
				projectSlug := filepath.Base(projectRoot)

				// Execute command in the project container
				result, err := s.runCommandInContainer(projectSlug, command, isBackground)
				if err != nil {
					return nil, fmt.Errorf("failed to execute command in container: %w", err)
				}

				return result, nil
			},
		},
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
			Description: "Fast text-based regex search that finds exact pattern matches within files or directories, utilizing the ripgrep command for efficient searching.\nTo avoid overwhelming output, the results are capped at 50 matches.\nUse the include or exclude patterns to filter the search scope by file type or specific paths.\nThis is best for finding exact text matches or regex patterns. This is preferred over semantic search when we know the exact symbol/function name/etc. to search in some set of directories/file types.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The regex pattern to search for",
					},
					"case_sensitive": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the search should be case sensitive",
					},
					"include_pattern": map[string]interface{}{
						"type":        "string",
						"description": "Glob pattern for files to include (e.g. '*.ts' for TypeScript files)",
					},
					"exclude_pattern": map[string]interface{}{
						"type":        "string",
						"description": "Glob pattern for files to exclude",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "One sentence explanation as to why this tool is being used.",
					},
				},
				"required": []string{"query", "include_pattern"},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				query, _ := args["query"].(string)
				caseSensitive, _ := args["case_sensitive"].(bool)
				includePattern, _ := args["include_pattern"].(string)
				excludePattern, _ := args["exclude_pattern"].(string)

				// Check if query contains regex patterns
				hasRegexPatterns := strings.ContainsAny(query, ".*+?^$|()[]{}")

				cmdArgs := []string{"/opt/homebrew/bin/rg", "--line-number", "--with-filename"}

				// If query doesn't contain regex patterns, use literal matching for better reliability
				if !hasRegexPatterns {
					cmdArgs = append(cmdArgs, "-F")
				}

				if !caseSensitive {
					cmdArgs = append(cmdArgs, "-i")
				}
				if includePattern != "" {
					cmdArgs = append(cmdArgs, "-g", fmt.Sprintf("**/%s", includePattern))
				}
				if excludePattern != "" {
					cmdArgs = append(cmdArgs, "-g", fmt.Sprintf("!**/%s", excludePattern))
				}

				// Use the query directly if using literal matching, otherwise escape it
				searchQuery := query
				if hasRegexPatterns {
					searchQuery = escapeRegexQuery(query)
				}

				cmdArgs = append(cmdArgs, searchQuery, projectRoot)
				s.Logger.Infof("grep_search cmd: %v", cmdArgs)
				s.Logger.Infof("grep_search original query: %s", query)
				s.Logger.Infof("grep_search search query: %s", searchQuery)
				s.Logger.Infof("grep_search using literal matching: %v", !hasRegexPatterns)

				cmd := exec.CommandContext(context.Background(), cmdArgs[0], cmdArgs[1:]...)

				// Capture both stdout and stderr
				output, err := cmd.CombinedOutput()
				if err != nil {
					// Log detailed error information
					s.Logger.Infof("grep_search error: %v", err)
					s.Logger.Infof("grep_search output: %s", string(output))
					s.Logger.Infof("grep_search command: %v", cmdArgs)
					s.Logger.Infof("grep_search original query: %s", query)
					s.Logger.Infof("grep_search search query: %s", searchQuery)

					// Check if it's an exit error (no matches found)
					if exitErr, ok := err.(*exec.ExitError); ok {
						s.Logger.Infof("grep_search exit code: %d", exitErr.ExitCode())
						if exitErr.ExitCode() == 1 {
							// No matches found, return empty results
							return map[string]interface{}{
								"results": []map[string]interface{}{},
								"query":   query,
							}, nil
						}
					}
					return nil, fmt.Errorf("grep_search failed: %w, output: %s", err, string(output))
				}

				lines := strings.Split(strings.TrimSpace(string(output)), "\n")
				var results []map[string]interface{}

				for i, line := range lines {
					if i >= 50 { // Cap at 50 results
						break
					}
					if line == "" {
						continue
					}

					// Parse ripgrep output format: file:line:content
					parts := strings.SplitN(line, ":", 3)
					if len(parts) >= 3 {
						results = append(results, map[string]interface{}{
							"file":    parts[0],
							"line":    parts[1],
							"content": parts[2],
						})
					}
				}

				return map[string]interface{}{
					"results": results,
					"query":   query,
				}, nil
			},
		},
		{
			Name:        "edit_file",
			Description: "Edit the contents of a file. You must provide the file's URI as well as a SINGLE string of SEARCH/REPLACE block(s) that will be used to apply the edit.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target_file": map[string]interface{}{
						"type":        "string",
						"description": "The target file to modify (relative to project root).",
					},
					"instructions": map[string]interface{}{
						"type":        "string",
						"description": "Instructions for the edit. This will be used to guide the edit.",
					},
					"search_replace_blocks": map[string]interface{}{
						"type":        "string",
						"description": replaceTool_description,
					},
				},
				"required": []string{"target_file", "search_replace_blocks"},
			},
			Handler: func(toolName string, args map[string]interface{}) (interface{}, error) {
				targetFile, _ := args["target_file"].(string)
				searchReplaceBlocks, _ := args["search_replace_blocks"].(string)

				// Check if content is empty and return early
				if strings.TrimSpace(searchReplaceBlocks) == "" {
					return map[string]interface{}{
						"result":    "No changes made - content is empty",
						"file_path": targetFile,
						"skipped":   true,
					}, nil
				}

				absPath := filepath.Join(projectRoot, targetFile)

				_, err := os.Stat(absPath)
				fileExists := err == nil

				// If file doesn't exist, create it with the new content
				if !fileExists {
					return nil, fmt.Errorf("file does not exist: %s", absPath)
				}

				// File exists, use search/replace functionality
				// Read the existing file content
				existingContent, err := os.ReadFile(absPath)
				if err != nil {
					return nil, fmt.Errorf("failed to read existing file: %w", err)
				}

				// Use the search/replace functionality to apply the edit
				opts := SearchReplaceOptions{
					From:         "edit_file_tool",
					ApplyStr:     searchReplaceBlocks,
					OriginalCode: string(existingContent),
					URI:          absPath,
				}

				result, err := InitializeSearchAndReplaceStream(opts)
				if err != nil {
					return nil, fmt.Errorf("search/replace operation failed: %w", err)
				}

				if !result.Success {
					return nil, fmt.Errorf("search/replace operation failed: %v", result.Error)
				}

				// Write the modified content back to the file
				if err := os.WriteFile(absPath, []byte(result.FinalCode), 0644); err != nil {
					return nil, fmt.Errorf("failed to write modified file: %w", err)
				}
				sessionchanges.RegisterChange(absPath)

				return map[string]interface{}{
					"result":         "File edited successfully using search/replace",
					"file_path":      targetFile,
					"blocks_applied": len(result.Blocks),
					"ai_used":        false,
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

// escapeRegexQuery escapes special regex characters in the query
func escapeRegexQuery(query string) string {
	// For simple text searches, we want to escape regex special characters
	// but we need to be careful not to over-escape

	// First, handle backslashes (must be first)
	escapedQuery := strings.ReplaceAll(query, "\\", "\\\\")

	// Escape regex special characters
	specialChars := []string{".", "*", "+", "?", "^", "$", "|", "(", ")", "[", "]", "{", "}", " "}
	for _, char := range specialChars {
		escapedQuery = strings.ReplaceAll(escapedQuery, char, "\\"+char)
	}

	return escapedQuery
}
