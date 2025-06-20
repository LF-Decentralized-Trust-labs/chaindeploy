package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
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

var createSearchReplaceBlocks_systemMessage = `
You are a coding assistant that takes in a diff, and outputs SEARCH/REPLACE code blocks to implement the change(s) in the diff.
The diff will be labeled ` + "`" + "DIFF" + "`" + ` and the original file will be labeled ` + "`" + "ORIGINAL_FILE" + "`" + `.

Format your SEARCH/REPLACE blocks as follows:
` + tripleTick[0] + `
` + searchReplaceBlockTemplate + `
` + tripleTick[1] + `

1. Your SEARCH/REPLACE block(s) must implement the diff EXACTLY. Do NOT leave anything out.

2. You are allowed to output multiple SEARCH/REPLACE blocks to implement the change.

3. Assume any comments in the diff are PART OF THE CHANGE. Include them in the output.

4. Your output should consist ONLY of SEARCH/REPLACE blocks. Do NOT output any text or explanations before or after this.

5. The ORIGINAL code in each SEARCH/REPLACE block must EXACTLY match lines in the original file. Do not add or remove any whitespace, comments, or modifications from the original code.

6. Each ORIGINAL text must be large enough to uniquely identify the change in the file. However, bias towards writing as little as possible.

7. Each ORIGINAL text must be DISJOINT from all other ORIGINAL text.

## EXAMPLE 1
DIFF
` + tripleTick[0] + `
// ... existing code
let x = 6.5
// ... existing code
` + tripleTick[1] + `

ORIGINAL_FILE
` + tripleTick[0] + `
let w = 5
let x = 6
let y = 7
let z = 8
` + tripleTick[1] + `

ACCEPTED OUTPUT
` + tripleTick[0] + `
` + ORIGINAL + `
let x = 6
` + DIVIDER + `
let x = 6.5
` + FINAL + `
` + tripleTick[1] + ``

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
	From          string
	ApplyStr      string
	StartBehavior string
	OriginalCode  string
	URI           string
	AIClient      *openai.Client
	Model         string
	MaxRetries    int
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

// InitializeSearchAndReplaceStream initializes and executes a search and replace stream using AI
// This is the Go equivalent of the TypeScript _initializeSearchAndReplaceStream function
func InitializeSearchAndReplaceStream(opts SearchReplaceOptions) (*SearchReplaceResult, error) {
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 4
	}

	if opts.Model == "" {
		opts.Model = "gpt-4.1-mini"
	}

	// Validate input
	if opts.ApplyStr == "" {
		return nil, fmt.Errorf("apply string cannot be empty")
	}

	if opts.OriginalCode == "" {
		return nil, fmt.Errorf("original code cannot be empty")
	}

	if opts.AIClient == nil {
		return nil, fmt.Errorf("AI client is required")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Prepare the user message
	userMessage := fmt.Sprintf(`DIFF
%s
%s
%s

ORIGINAL_FILE
%s
%s
%s`,
		tripleTick[0],
		opts.ApplyStr,
		tripleTick[1],
		tripleTick[0],
		opts.OriginalCode,
		tripleTick[1])

	// Prepare messages for AI
	messages := []openai.ChatCompletionMessage{
		{
			Role:    "system",
			Content: createSearchReplaceBlocks_systemMessage,
		},
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	// Retry loop
	var lastError error
	for attempt := 0; attempt < opts.MaxRetries; attempt++ {
		// Create AI request
		request := openai.ChatCompletionRequest{
			Model:    opts.Model,
			Messages: messages,
			Stream:   false,
		}

		// Call AI
		response, err := opts.AIClient.CreateChatCompletion(ctx, request)
		if err != nil {
			lastError = fmt.Errorf("AI API call failed (attempt %d): %w", attempt+1, err)
			continue
		}

		if len(response.Choices) == 0 {
			lastError = fmt.Errorf("no response from AI (attempt %d)", attempt+1)
			continue
		}

		// Extract the generated search/replace blocks
		generatedContent := response.Choices[0].Message.Content

		// Extract search/replace blocks from the generated content
		blocks, err := extractSearchReplaceBlocks(generatedContent)
		if err != nil {
			lastError = fmt.Errorf("failed to extract search/replace blocks (attempt %d): %w", attempt+1, err)
			continue
		}

		if len(blocks) == 0 {
			lastError = fmt.Errorf("no valid search/replace blocks found (attempt %d)", attempt+1)
			continue
		}

		// Apply the blocks to the original code
		finalCode, err := applyBlocksToCode(opts.OriginalCode, blocks)
		if err != nil {
			lastError = fmt.Errorf("failed to apply blocks to code (attempt %d): %w", attempt+1, err)
			continue
		}

		// Success!
		return &SearchReplaceResult{
			Success:   true,
			Blocks:    blocks,
			FinalCode: finalCode,
		}, nil
	}

	// All attempts failed
	return &SearchReplaceResult{
		Success: false,
		Error:   fmt.Errorf("all %d attempts failed. Last error: %w", opts.MaxRetries, lastError),
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
