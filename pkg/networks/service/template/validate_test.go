package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTemplateStructure_MissingVersion(t *testing.T) {
	s := &TemplateService{}
	req := &ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "",
			Network: NetworkDefinition{Platform: "fabric", Fabric: &FabricNetworkTemplate{ChannelName: "ch"}},
		},
	}
	resp := &ValidateTemplateResponse{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationWarning{}}
	s.validateTemplateStructure(req, resp)
	assert.False(t, resp.Valid)
	assert.Equal(t, ErrCodeInvalidVersion, resp.Errors[0].Code)
}

func TestValidateTemplateStructure_NewerVersion(t *testing.T) {
	s := &TemplateService{}
	req := &ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "99.0.0",
			Network: NetworkDefinition{Platform: "fabric", Fabric: &FabricNetworkTemplate{ChannelName: "ch"}},
		},
	}
	resp := &ValidateTemplateResponse{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationWarning{}}
	s.validateTemplateStructure(req, resp)
	assert.True(t, resp.Valid)
	assert.Len(t, resp.Warnings, 1)
	assert.Equal(t, WarnCodeVersionNewer, resp.Warnings[0].Code)
}

func TestValidateTemplateStructure_UnsupportedPlatform(t *testing.T) {
	s := &TemplateService{}
	req := &ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{Platform: "corda"},
		},
	}
	resp := &ValidateTemplateResponse{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationWarning{}}
	s.validateTemplateStructure(req, resp)
	assert.False(t, resp.Valid)
}

func TestValidateTemplateStructure_FabricMissingConfig(t *testing.T) {
	s := &TemplateService{}
	req := &ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{Platform: "fabric"},
		},
	}
	resp := &ValidateTemplateResponse{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationWarning{}}
	s.validateTemplateStructure(req, resp)
	assert.False(t, resp.Valid)
	assert.Equal(t, ErrCodeMissingFabricConfig, resp.Errors[0].Code)
}

func TestValidateTemplateStructure_FabricMissingChannelName(t *testing.T) {
	s := &TemplateService{}
	req := &ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{
				Platform: "fabric",
				Fabric:   &FabricNetworkTemplate{ChannelName: ""},
			},
		},
	}
	resp := &ValidateTemplateResponse{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationWarning{}}
	s.validateTemplateStructure(req, resp)
	assert.False(t, resp.Valid)
	assert.Equal(t, ErrCodeMissingRequiredField, resp.Errors[0].Code)
}

func TestValidateTemplateStructure_FabricChannelNameOverride(t *testing.T) {
	s := &TemplateService{}
	ch := "overridden-channel"
	req := &ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{
				Platform: "fabric",
				Fabric:   &FabricNetworkTemplate{ChannelName: ""},
			},
		},
		Overrides: ImportOverrides{ChannelName: &ch},
	}
	resp := &ValidateTemplateResponse{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationWarning{}}
	s.validateTemplateStructure(req, resp)
	assert.True(t, resp.Valid)
}

func TestValidateTemplateStructure_BesuMissingConfig(t *testing.T) {
	s := &TemplateService{}
	req := &ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{Platform: "besu"},
		},
	}
	resp := &ValidateTemplateResponse{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationWarning{}}
	s.validateTemplateStructure(req, resp)
	assert.False(t, resp.Valid)
	assert.Equal(t, ErrCodeMissingBesuConfig, resp.Errors[0].Code)
}

func TestValidateTemplateStructure_BesuMissingChainID(t *testing.T) {
	s := &TemplateService{}
	req := &ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{
				Platform: "besu",
				Besu:     &BesuNetworkTemplate{ChainID: 0},
			},
		},
	}
	resp := &ValidateTemplateResponse{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationWarning{}}
	s.validateTemplateStructure(req, resp)
	assert.False(t, resp.Valid)
	assert.Equal(t, ErrCodeMissingRequiredField, resp.Errors[0].Code)
}

func TestValidateTemplateStructure_BesuValid(t *testing.T) {
	s := &TemplateService{}
	req := &ValidateTemplateRequest{
		Template: NetworkTemplate{
			Version: "2.0.0",
			Network: NetworkDefinition{
				Platform: "besu",
				Besu:     &BesuNetworkTemplate{ChainID: 1337, Consensus: "qbft"},
			},
		},
	}
	resp := &ValidateTemplateResponse{Valid: true, Errors: []ValidationError{}, Warnings: []ValidationWarning{}}
	s.validateTemplateStructure(req, resp)
	assert.True(t, resp.Valid)
	assert.Empty(t, resp.Errors)
}

func TestErrorCodesAreUnique(t *testing.T) {
	codes := []string{
		ErrCodeInvalidVersion,
		ErrCodeMissingFabricConfig,
		ErrCodeMissingBesuConfig,
		ErrCodeOrgNotFound,
		ErrCodeNodeNotFound,
		ErrCodeMissingRequiredField,
		ErrCodeMissingVariableBinding,
		ErrCodeInvalidVariableBinding,
		ErrCodeInvalidVariableType,
	}

	seen := make(map[string]bool)
	for _, code := range codes {
		assert.False(t, seen[code], "duplicate error code: %s", code)
		seen[code] = true
	}
}
