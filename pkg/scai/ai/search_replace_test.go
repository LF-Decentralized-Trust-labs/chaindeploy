package ai

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

// Test data for search/replace blocks
const testSearchReplaceBlocks = `<<<<< ORIGINAL
// SimpleChaincode implements the fabric-contract-api-go programming model
type SimpleChaincode struct {
	contractapi.Contract
}

=======
// SimpleChaincode implements the fabric-contract-api-go programming model
type SimpleChaincode struct {
	contractapi.Contract
}

// GetClientIdentity returns the client identity string from the transaction context
func (s *SimpleChaincode) GetClientIdentity(ctx contractapi.TransactionContextInterface) (string, error) {
	clientIdentity, err := ctx.GetClientIdentity().GetID()
	if err != nil {
			return \"\", fmt.Errorf(\"failed to get client identity: %w\", err)
	}
	return clientIdentity, nil
}

>>>>>>> UPDATED`

// Test data for multiple search/replace blocks
const testMultipleSearchReplaceBlocks = `<<<<< ORIGINAL
// SimpleChaincode implements the fabric-contract-api-go programming model
type SimpleChaincode struct {
	contractapi.Contract
}

=======
// SimpleChaincode implements the fabric-contract-api-go programming model
type SimpleChaincode struct {
	contractapi.Contract
}

// GetClientIdentity returns the client identity string from the transaction context
func (s *SimpleChaincode) GetClientIdentity(ctx contractapi.TransactionContextInterface) (string, error) {
		clientIdentity, err := ctx.GetClientIdentity().GetID()
	if err != nil {
			return \"\", fmt.Errorf(\"failed to get client identity: %w\", err)
	}
	return clientIdentity, nil
}

>>>>>>> UPDATED
`

// TestInstantlyApplySearchReplaceBlocks tests the logic similar to instantlyApplySearchReplaceBlocks
func TestInstantlyApplySearchReplaceBlocks(t *testing.T) {
	// Read the original file
	fullFile, err := os.ReadFile("resources/contract.txt")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	originalCode := string(fullFile)
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	client := openai.NewClient(apiKey)
	// Test single search/replace block
	t.Run("SingleBlock", func(t *testing.T) {

		result, err := InitializeSearchAndReplaceStream(SearchReplaceOptions{
			ApplyStr:     testSearchReplaceBlocks,
			OriginalCode: originalCode,
			AIClient:     client,
			Model:        "gpt-4.1-mini",
			MaxRetries:   1,
		})
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if result.Error != nil {
			t.Fatalf("Expected no error, got: %v", result.Error)
		}
		fmt.Printf("Final code:\n%s\n", result.FinalCode)

		// Verify the original function is still there
		if !strings.Contains(result.FinalCode, "GetClientIdentity") {
			t.Error("Expected result to contain GetClientIdentity function")
		}

		fmt.Printf("Single block test passed. Result length: %d\n", len(result.FinalCode))
	})

	// // Test multiple search/replace blocks
	// t.Run("MultipleBlocks", func(t *testing.T) {
	// 	result, err := applySearchReplaceBlocks(originalCode, testMultipleSearchReplaceBlocks)
	// 	if err != nil {
	// 		t.Fatalf("Expected no error, got: %v", err)
	// 	}

	// 	// Verify error handling was improved
	// 	if !strings.Contains(result, "failed to marshal asset") {
	// 		t.Error("Expected result to contain improved error handling")
	// 	}

	// 	// Verify the original function is still there
	// 	if !strings.Contains(result, "GetClientIdentity") {
	// 		t.Error("Expected result to contain GetClientIdentity function")
	// 	}

	// 	fmt.Printf("Multiple blocks test passed. Result length: %d\n", len(result))
	// })

	// 	// Test invalid search/replace blocks
	// 	t.Run("InvalidBlocks", func(t *testing.T) {
	// 		invalidBlocks := `<<<<<<< ORIGINAL
	// This text does not exist in the original file
	// =======
	// Some replacement text
	// >>>>>>> UPDATED`

	// 		_, err := applySearchReplaceBlocks(originalCode, invalidBlocks)
	// 		if err == nil {
	// 			t.Error("Expected error for non-existent text, got nil")
	// 		}
	// 		fmt.Printf("Invalid blocks test passed. Error: %v\n", err)
	// 	})

	// 	// Test overlapping blocks
	// 	t.Run("OverlappingBlocks", func(t *testing.T) {
	// 		overlappingBlocks := `<<<<<<< ORIGINAL
	// 	exists, err := t.AssetExists(ctx, assetID)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to get asset: %v", err)
	// 	}
	// =======
	// 	exists, err := t.AssetExists(ctx, assetID)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to get asset: %v", err)
	// 	}
	// >>>>>>> UPDATED

	// <<<<<<< ORIGINAL
	// 	exists, err := t.AssetExists(ctx, assetID)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to get asset: %v", err)
	// 	}
	// 	if exists {
	// 		return fmt.Errorf("asset already exists: %s", assetID)
	// 	}
	// =======
	// 	exists, err := t.AssetExists(ctx, assetID)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to get asset: %v", err)
	// 	}
	// 	if exists {
	// 		return fmt.Errorf("asset already exists: %s", assetID)
	// 	}
	// >>>>>>> UPDATED`

	//		_, err := applySearchReplaceBlocks(originalCode, overlappingBlocks)
	//		if err == nil {
	//			t.Error("Expected error for overlapping blocks, got nil")
	//		}
	//		fmt.Printf("Overlapping blocks test passed. Error: %v\n", err)
	//	})
}

// // applySearchReplaceBlocks implements the logic from instantlyApplySearchReplaceBlocks
// func applySearchReplaceBlocks(originalCode, blocksStr string) (string, error) {
// 	blocks, err := extractSearchReplaceBlocks(blocksStr)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to extract search/replace blocks: %w", err)
// 	}
// 	if len(blocks) == 0 {
// 		return "", fmt.Errorf("no search/replace blocks were received")
// 	}

// 	// Convert original code to lines for easier manipulation
// 	originalLines := strings.Split(originalCode, "\n")

// 	// Find all replacements
// 	type replacement struct {
// 		startLine int
// 		endLine   int
// 		newText   string
// 	}

// 	var replacements []replacement

// 	for _, block := range blocks {
// 		// Find the original text in the code
// 		originalText := strings.TrimSpace(block.Original)
// 		if originalText == "" {
// 			continue
// 		}

// 		// Search for the original text in the code
// 		found := false
// 		var startLine, endLine int

// 		// Simple line-by-line search (can be improved with more sophisticated matching)
// 		for i := 0; i < len(originalLines); i++ {
// 			// Check if this line starts the original text
// 			if strings.Contains(strings.Join(originalLines[i:], "\n"), originalText) {
// 				// Find the end of this block
// 				originalLinesText := strings.Join(originalLines[i:], "\n")
// 				startIdx := strings.Index(originalLinesText, originalText)
// 				if startIdx == 0 {
// 					// Found the start
// 					startLine = i
// 					// Count lines in the original text
// 					linesInOriginal := strings.Count(originalText, "\n") + 1
// 					endLine = i + linesInOriginal - 1
// 					found = true
// 					break
// 				}
// 			}
// 		}

// 		if !found {
// 			return "", fmt.Errorf("original text not found: %s", originalText)
// 		}

// 		// Check for uniqueness
// 		for _, existing := range replacements {
// 			if (startLine >= existing.startLine && startLine <= existing.endLine) ||
// 				(endLine >= existing.startLine && endLine <= existing.endLine) ||
// 				(existing.startLine >= startLine && existing.endLine <= endLine) {
// 				return "", fmt.Errorf("overlapping replacements detected")
// 			}
// 		}

// 		replacements = append(replacements, replacement{
// 			startLine: startLine,
// 			endLine:   endLine,
// 			newText:   block.Final,
// 		})
// 	}

// 	// Sort replacements by start line (descending) to apply from end to beginning
// 	// This prevents index shifting issues
// 	for i := 0; i < len(replacements)-1; i++ {
// 		for j := i + 1; j < len(replacements); j++ {
// 			if replacements[i].startLine < replacements[j].startLine {
// 				replacements[i], replacements[j] = replacements[j], replacements[i]
// 			}
// 		}
// 	}

// 	// Apply replacements
// 	resultLines := make([]string, len(originalLines))
// 	copy(resultLines, originalLines)

// 	for _, repl := range replacements {
// 		// Split the new text into lines
// 		newLines := strings.Split(repl.newText, "\n")

// 		// Replace the lines
// 		newResultLines := make([]string, 0, len(resultLines)-(repl.endLine-repl.startLine+1)+len(newLines))
// 		newResultLines = append(newResultLines, resultLines[:repl.startLine]...)
// 		newResultLines = append(newResultLines, newLines...)
// 		newResultLines = append(newResultLines, resultLines[repl.endLine+1:]...)

// 		resultLines = newResultLines
// 	}

// 	return strings.Join(resultLines, "\n"), nil
// }

// // TestEditFileToolOutput tests the output format expected from the edit_file tool
// func TestEditFileToolOutput(t *testing.T) {
// 	// Simulate the output from edit_file tool
// 	editFileOutput := map[string]interface{}{
// 		"result":         "File edited successfully using AI search/replace",
// 		"file_path":      "test_file.go",
// 		"blocks_applied": 2,
// 		"ai_used":        true,
// 	}

// 	// Test the output structure
// 	if result, ok := editFileOutput["result"].(string); !ok || result == "" {
// 		t.Error("Expected non-empty result string")
// 	}

// 	if filePath, ok := editFileOutput["file_path"].(string); !ok || filePath == "" {
// 		t.Error("Expected non-empty file_path string")
// 	}

// 	if blocksApplied, ok := editFileOutput["blocks_applied"].(int); !ok || blocksApplied <= 0 {
// 		t.Error("Expected positive blocks_applied integer")
// 	}

// 	if aiUsed, ok := editFileOutput["ai_used"].(bool); !ok || !aiUsed {
// 		t.Error("Expected ai_used to be true")
// 	}

// 	fmt.Printf("Edit file tool output test passed. Applied %d blocks to %s\n",
// 		editFileOutput["blocks_applied"], editFileOutput["file_path"])
// }

// // TestSearchReplaceBlockExtraction tests the extraction of search/replace blocks
// func TestSearchReplaceBlockExtraction(t *testing.T) {
// 	// Test valid blocks
// 	blocks, err := extractSearchReplaceBlocks(testSearchReplaceBlocks)
// 	if err != nil {
// 		t.Fatalf("Failed to extract blocks: %v", err)
// 	}
// 	if len(blocks) != 1 {
// 		t.Errorf("Expected 1 block, got %d", len(blocks))
// 	}

// 	if blocks[0].Original == "" {
// 		t.Error("Expected non-empty original text")
// 	}

// 	if blocks[0].Final == "" {
// 		t.Error("Expected non-empty final text")
// 	}

// 	// Test multiple blocks
// 	multipleBlocks, err := extractSearchReplaceBlocks(testMultipleSearchReplaceBlocks)
// 	if err != nil {
// 		t.Fatalf("Failed to extract multiple blocks: %v", err)
// 	}
// 	if len(multipleBlocks) != 2 {
// 		t.Errorf("Expected 2 blocks, got %d", len(multipleBlocks))
// 	}

// 	fmt.Printf("Block extraction test passed. Extracted %d blocks from multiple block test\n", len(multipleBlocks))
// }

// // TestInitializeSearchAndReplaceStream_Success is the original test with improvements
// func TestInitializeSearchAndReplaceStream_Success(t *testing.T) {
// 	apiKey := os.Getenv("OPENAI_API_KEY")
// 	if apiKey == "" {
// 		t.Skip("OPENAI_API_KEY not set; skipping real OpenAI API test.")
// 	}

// 	client := openai.NewClient(apiKey)
// 	fullFile, err := os.ReadFile("resources/contract.txt")
// 	if err != nil {
// 		t.Fatalf("Expected no error, got: %v", err)
// 	}

// 	opts := SearchReplaceOptions{
// 		ApplyStr:     testSearchReplaceBlocks,
// 		OriginalCode: string(fullFile),
// 		AIClient:     client,
// 		Model:        "gpt-4o-mini",
// 		MaxRetries:   2,
// 	}

// 	result, err := InitializeSearchAndReplaceStream(opts)
// 	if err != nil {
// 		t.Fatalf("Expected no error, got: %v", err)
// 	}
// 	if !result.Success {
// 		t.Fatalf("Expected success, got failure: %v", result.Error)
// 	}

// 	fmt.Printf("AI Search/Replace test passed:\n")
// 	fmt.Printf("- Success: %v\n", result.Success)
// 	fmt.Printf("- Number of blocks: %d\n", len(result.Blocks))
// 	fmt.Printf("- Final code length: %d characters\n", len(result.FinalCode))

// 	for i, block := range result.Blocks {
// 		fmt.Printf("- Block %d: Original length=%d, Final length=%d\n",
// 			i, len(block.Original), len(block.Final))
// 	}
// }
