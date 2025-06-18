package projectrunner

import (
	"fmt"

	"github.com/chainlaunch/chainlaunch/pkg/scai/boilerplates"
)

// BoilerplateRunnerConfig defines how to run a project for a given boilerplate type.
type BoilerplateRunnerConfig struct {
	Command string
	Args    []string
	Image   string // Docker image to use for this boilerplate
	Prompt  string // Boilerplate-specific prompt to add to system prompt
}

var boilerplateRunners = map[string]BoilerplateRunnerConfig{
	"chaincode-fabric-ts": {
		Args:  []string{"npm", "run", "start:dev"},
		Image: "chaincode-ts:1.0",
		Prompt: `This is a Hyperledger Fabric TypeScript chaincode project.

Key Technologies and Patterns:
- TypeScript for type-safe chaincode development
- Hyperledger Fabric SDK for Node.js
- Chaincode lifecycle management
- Asset-based data modeling
- Smart contract patterns for blockchain

Development Guidelines:
- Use TypeScript interfaces for asset definitions
- Implement proper error handling with Fabric errors
- Follow Fabric's transaction-based programming model
- Use the fabric-contract-api for contract development
- Structure code with clear separation of concerns
- Implement proper validation for all inputs
- Use Fabric's built-in access control mechanisms

Common Patterns:
- Asset CRUD operations (Create, Read, Update, Delete)
- Query operations with rich filtering
- Cross-asset transactions
- Event emission for external systems
- Access control and authorization checks

Important Contract Design Principles:
- Contracts must be completely stateless - never store data in memory or class variables
- All data must be persisted to the ledger using putState/getState operations
- Avoid any in-memory caching or temporary storage
- Each transaction should be independent and not rely on previous transaction state
- Use the ledger as the single source of truth for all data
- Implement proper state management through Fabric's key-value store
- Ensure all operations read from and write to the ledger directly

State Management Best Practices:
- Always use ctx.stub.putState() to persist data
- Always use ctx.stub.getState() to retrieve data
- Never use class properties or variables to store transaction data
- Implement proper key management for state storage
- Use composite keys for complex data relationships
- Validate state existence before operations
- Handle state deletion with ctx.stub.deleteState()

Transaction Isolation:
- Each transaction should be self-contained
- Don't rely on data from previous transactions in memory
- Always query the ledger for current state
- Implement proper error handling for missing states
- Use transactions to maintain data consistency
`,
	},
}

// GetBoilerplateRunner returns the command, args, and image for a given boilerplate type
func GetBoilerplateRunner(boilerplateService *boilerplates.BoilerplateService, boilerplateType string) (string, []string, string, error) {
	config, err := boilerplateService.GetBoilerplateConfig(boilerplateType)
	if err != nil {
		return "", nil, "", fmt.Errorf("unknown boilerplate type: %s", boilerplateType)
	}

	return config.Command, config.Args, config.Image, nil
}

// GetBoilerplatePrompt returns the boilerplate-specific prompt for a given boilerplate type
func GetBoilerplatePrompt(boilerplateType string) string {
	if config, exists := boilerplateRunners[boilerplateType]; exists {
		return config.Prompt
	}
	return "" // Return empty string if boilerplate type not found
}
