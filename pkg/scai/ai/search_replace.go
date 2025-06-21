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
You are a coding assistant that takes in a diff and automatically identifies the exact chunks of code in the original file that need to be replaced to implement the changes.

The diff will be labeled ` + "`" + "DIFF" + "`" + ` and the original file will be labeled ` + "`" + "ORIGINAL_FILE" + "`" + `.

Your task is to:
1. Analyze the diff to understand what changes need to be made
2. Find the EXACT corresponding code chunks in the original file
3. Generate SEARCH/REPLACE blocks that will apply the changes

Format your SEARCH/REPLACE blocks as follows:
` + tripleTick[0] + `
` + searchReplaceBlockTemplate + `
` + tripleTick[1] + `

## CRITICAL INSTRUCTIONS FOR AUTOMATIC CHUNK IDENTIFICATION:

1. **ANALYZE THE DIFF FIRST**: Look at the diff to understand what function, method, or code section needs to be added, modified, or removed.

2. **FIND THE RIGHT LOCATION**: In the original file, identify where this change should be applied:
   - For new functions: Find the end of the struct/class or the last function in the file
   - For modifications: Find the exact function/method that needs to be changed
   - For additions: Find the appropriate location (end of file, after similar functions, etc.)

3. **INCLUDE SUFFICIENT CONTEXT**: The ORIGINAL text must include enough context to uniquely identify the location:
   - Include function signatures, struct definitions, or surrounding code
   - Include enough lines to make the location unambiguous
   - For new functions: Include the closing brace of the last function or the end of the struct
   - **CRITICAL**: The ORIGINAL text must EXACTLY match what exists in the original file

4. **EXACT MATCHING**: The ORIGINAL code must EXACTLY match lines in the original file (including whitespace, comments, etc.)

5. **MULTIPLE BLOCKS**: You may need multiple blocks if the diff contains multiple changes in different locations.

6. **FOR NEW FUNCTIONS**: When adding a new function that doesn't exist in the original file:
   - Find the last function in the file or the end of the struct
   - Include the closing brace and any trailing whitespace/newlines
   - The FINAL text should include the original closing brace PLUS the new function

## EXAMPLE FOR ADDING A NEW FUNCTION:
DIFF:
` + tripleTick[0] + `
// GetClientIdentity returns the client identity's ID string
func (t *SimpleChaincode) GetClientIdentity(ctx contractapi.TransactionContextInterface) (string, error) {
	clientID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return "", fmt.Errorf("failed to get client identity: %v", err)
	}
	return clientID, nil
}
` + tripleTick[1] + `

ORIGINAL_FILE:
` + tripleTick[0] + `
func (t *SimpleChaincode) CreateAsset(ctx contractapi.TransactionContextInterface, assetID, color string, size int) error {
	// ... function body ...
	return nil
}
` + tripleTick[1] + `

CORRECT OUTPUT:
` + tripleTick[0] + `
` + ORIGINAL + `
	return nil
}
` + DIVIDER + `
	return nil
}

// GetClientIdentity returns the client identity's ID string
func (t *SimpleChaincode) GetClientIdentity(ctx contractapi.TransactionContextInterface) (string, error) {
	clientID, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return "", fmt.Errorf("failed to get client identity: %v", err)
	}
	return clientID, nil
}
` + FINAL + `
` + tripleTick[1] + `

## IMPORTANT RULES:
- The ORIGINAL text must EXIST in the original file
- For new functions, find the insertion point (usually the end of the last function)
- Include the closing brace and any trailing whitespace in the ORIGINAL text
- The FINAL text should preserve the original closing brace and add the new function after it

## OUTPUT FORMAT:
Your output should consist ONLY of SEARCH/REPLACE blocks. Do NOT output any text or explanations before or after this.

Each ORIGINAL text must be large enough to uniquely identify the change in the file, but bias towards including sufficient context to ensure accurate placement.
`

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
		fmt.Printf("Response: %+v\n", response)
		// Extract the generated search/replace blocks
		generatedContent := response.Choices[0].Message.Content

		// Extract search/replace blocks from the generated content
		blocks, err := extractSearchReplaceBlocks(generatedContent)
		if err != nil {
			lastError = fmt.Errorf("failed to extract search/replace blocks (attempt %d): %w", attempt+1, err)
			continue
		}
		fmt.Printf("Blocks: %+v\n", blocks)
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
