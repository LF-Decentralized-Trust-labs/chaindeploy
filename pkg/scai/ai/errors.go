package ai

import "fmt"

// MaxTokensExceededError is returned when the token count exceeds the model's maximum.
type MaxTokensExceededError struct {
	Model      string
	TokenCount int
	MaxTokens  int
}

func (e *MaxTokensExceededError) Error() string {
	return fmt.Sprintf("token count %d exceeds max tokens %d for model %s", e.TokenCount, e.MaxTokens, e.Model)
}
