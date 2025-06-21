package ai

import (
	"fmt"
	"strings"
)

// Triple backtick wrapper used throughout the prompts for code blocks
var tripleTick = []string{"```", "```"}

// Maximum limits for directory structure information
const MAX_DIRSTR_CHARS_TOTAL_BEGINNING = 20_000
const MAX_DIRSTR_CHARS_TOTAL_TOOL = 20_000
const MAX_DIRSTR_RESULTS_TOTAL_BEGINNING = 100
const MAX_DIRSTR_RESULTS_TOTAL_TOOL = 100

// tool info
const MAX_FILE_CHARS_PAGE = 500_000
const MAX_CHILDREN_URIs_PAGE = 500

// terminal tool info
const MAX_TERMINAL_CHARS = 100_000
const MAX_TERMINAL_INACTIVE_TIME = 8 // seconds
const MAX_TERMINAL_BG_COMMAND_TIME = 5

// Maximum character limits for prefix and suffix context
const MAX_PREFIX_SUFFIX_CHARS = 20_000

const ORIGINAL = "<<<<<<< ORIGINAL"
const DIVIDER = "======="
const FINAL = ">>>>>>> UPDATED"

var searchReplaceBlockTemplate = `\
` + ORIGINAL + `
// ... original code goes here
` + DIVIDER + `
// ... final code goes here
` + FINAL + `

` + ORIGINAL + `
// ... original code goes here
` + DIVIDER + `
// ... final code goes here
` + FINAL

var replaceTool_description = `\
A string of SEARCH/REPLACE block(s) which will be applied to the given file.
Your SEARCH/REPLACE blocks string must be formatted as follows:
` + searchReplaceBlockTemplate + `

## Guidelines:

1. You may output multiple search replace blocks if needed.

2. The ORIGINAL code in each SEARCH/REPLACE block must EXACTLY match lines in the original file. Do not add or remove any whitespace or comments from the original code.

3. Each ORIGINAL text must be large enough to uniquely identify the change. However, bias towards writing as little as possible.

4. Each ORIGINAL text must be DISJOINT from all other ORIGINAL text.

5. This field is a STRING (not an array).

`

// SearchReplaceOptions contains the options for search and replace operations
type SearchReplaceOptions struct {
	From         string
	ApplyStr     string
	OriginalCode string
	URI          string
	AIClient     interface{} // OpenAI client or other AI client
	Model        string      // AI model to use
	MaxRetries   int         // Maximum number of retries
}

// SearchReplaceBlock represents a single search/replace block
type SearchReplaceBlock struct {
	Original string
	Final    string
	State    string // "writingOriginal", "writingFinal", "done"
}

// SearchReplaceResult contains the result of a search/replace operation
type SearchReplaceResult struct {
	Success   bool
	Error     error
	Blocks    []SearchReplaceBlock
	FinalCode string
}

// InitializeSearchAndReplaceStream processes search and replace blocks synchronously without AI
// This is the simplified Go equivalent that directly parses and applies search/replace blocks
func InitializeSearchAndReplaceStream(opts SearchReplaceOptions) (*SearchReplaceResult, error) {
	// Validate input
	if opts.ApplyStr == "" {
		return nil, fmt.Errorf("apply string cannot be empty")
	}

	if opts.OriginalCode == "" {
		return nil, fmt.Errorf("original code cannot be empty")
	}

	// Extract search/replace blocks from the apply string
	blocks, err := extractSearchReplaceBlocks(opts.ApplyStr)
	if err != nil {
		return &SearchReplaceResult{
			Success: false,
			Error:   fmt.Errorf("failed to extract search/replace blocks: %w", err),
		}, nil
	}

	if len(blocks) == 0 {
		return &SearchReplaceResult{
			Success: false,
			Error:   fmt.Errorf("no valid search/replace blocks found"),
		}, nil
	}

	// Apply the blocks to the original code
	finalCode, err := applyBlocksToCode(opts.OriginalCode, blocks)
	if err != nil {
		return &SearchReplaceResult{
			Success: false,
			Error:   fmt.Errorf("failed to apply blocks to code: %w", err),
		}, nil
	}

	// Success!
	return &SearchReplaceResult{
		Success:   true,
		Blocks:    blocks,
		FinalCode: finalCode,
	}, nil
}

// TextRange represents a range of text in the original code
type TextRange struct {
	Start int
	End   int
}

// extractSearchReplaceBlocks extracts search/replace blocks from the apply string
func extractSearchReplaceBlocks(applyStr string) ([]SearchReplaceBlock, error) {
	var blocks []SearchReplaceBlock

	// Split by the search/replace block template
	parts := strings.Split(applyStr, ORIGINAL)

	for _, part := range parts[1:] { // Skip the first part (before first ORIGINAL)
		// Split by DIVIDER to separate original and final
		dividerParts := strings.Split(part, DIVIDER)
		if len(dividerParts) != 2 {
			continue // Skip malformed blocks
		}

		// Split by FINAL to get the final part
		finalParts := strings.Split(dividerParts[1], FINAL)
		if len(finalParts) < 1 {
			continue // Skip malformed blocks
		}

		original := strings.TrimSpace(dividerParts[0])
		final := strings.TrimSpace(finalParts[0])

		if original != "" && final != "" {
			blocks = append(blocks, SearchReplaceBlock{
				Original: original,
				Final:    final,
				State:    "done",
			})
		}
	}

	return blocks, nil
}

// findTextInCode finds the range of text in the original code
func findTextInCode(searchText, originalCode string) (TextRange, error) {
	searchIndex := strings.Index(originalCode, searchText)
	if searchIndex == -1 {
		return TextRange{Start: -1, End: -1}, fmt.Errorf("text not found in original code")
	}

	return TextRange{
		Start: searchIndex,
		End:   searchIndex + len(searchText),
	}, nil
}

// applyReplacement applies a replacement to the original code
func applyReplacement(originalCode string, textRange TextRange, replacement string) (string, error) {
	if textRange.Start < 0 || textRange.End > len(originalCode) || textRange.Start >= textRange.End {
		return "", fmt.Errorf("invalid text range")
	}

	before := originalCode[:textRange.Start]
	after := originalCode[textRange.End:]

	return before + replacement + after, nil
}

// applyBlocksToCode applies multiple search/replace blocks to the original code
func applyBlocksToCode(originalCode string, blocks []SearchReplaceBlock) (string, error) {
	currentCode := originalCode

	for i, block := range blocks {
		// Find the original text in the current code
		originalRange, err := findTextInCode(block.Original, currentCode)
		if err != nil {
			return "", fmt.Errorf("failed to find original text in block %d: %w", i+1, err)
		}

		// Validate that the original text was found
		if originalRange.Start == -1 || originalRange.End == -1 {
			return "", fmt.Errorf("original text not found in block %d", i+1)
		}

		// Apply the replacement
		newCode, err := applyReplacement(currentCode, originalRange, block.Final)
		if err != nil {
			return "", fmt.Errorf("failed to apply replacement for block %d: %w", i+1, err)
		}

		// Update the current code for subsequent blocks
		currentCode = newCode
	}

	return currentCode, nil
}
