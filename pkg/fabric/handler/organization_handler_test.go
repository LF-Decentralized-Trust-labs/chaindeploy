package handler

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

// TestDurationValidation tests the duration format validation logic
func TestDurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		duration    string
		wantErr     bool
		description string
	}{
		{
			name:        "valid - 10 years in hours",
			duration:    "87600h",
			wantErr:     false,
			description: "87600h = 10 years",
		},
		{
			name:        "valid - 1 year in hours",
			duration:    "8760h",
			wantErr:     false,
			description: "8760h = 1 year",
		},
		{
			name:        "valid - 24 hours",
			duration:    "24h",
			wantErr:     false,
			description: "24 hours",
		},
		{
			name:        "valid - 1 week in hours",
			duration:    "168h",
			wantErr:     false,
			description: "168h = 1 week",
		},
		{
			name:        "valid - mixed duration",
			duration:    "2h30m",
			wantErr:     false,
			description: "2 hours 30 minutes",
		},
		{
			name:        "valid - 2 years in hours",
			duration:    "17520h",
			wantErr:     false,
			description: "17520h = 2 years",
		},
		{
			name:        "valid - minutes only",
			duration:    "30m",
			wantErr:     false,
			description: "30 minutes",
		},
		{
			name:        "valid - seconds only",
			duration:    "60s",
			wantErr:     false,
			description: "60 seconds",
		},
		{
			name:        "invalid - no unit",
			duration:    "87600",
			wantErr:     true,
			description: "Missing time unit",
		},
		{
			name:        "invalid - years unit",
			duration:    "10years",
			wantErr:     true,
			description: "years is not a valid Go duration unit",
		},
		{
			name:        "invalid - y unit",
			duration:    "1y",
			wantErr:     true,
			description: "y is not a valid Go duration unit",
		},
		{
			name:        "invalid - d unit (days)",
			duration:    "7d",
			wantErr:     true,
			description: "d (days) is not a valid Go duration unit",
		},
		{
			name:        "invalid - random string",
			duration:    "invalid",
			wantErr:     true,
			description: "Random string is not valid",
		},
		{
			name:        "invalid - empty string",
			duration:    "",
			wantErr:     true,
			description: "Empty string is not valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := time.ParseDuration(tt.duration)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for duration '%s' (%s), but got none", tt.duration, tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for duration '%s' (%s): %v", tt.duration, tt.description, err)
				}
			}
		})
	}
}

// TestCreateOrganizationRequest_DurationFields tests that the request struct properly handles duration fields
func TestCreateOrganizationRequest_DurationFields(t *testing.T) {
	tests := []struct {
		name           string
		jsonBody       string
		wantCaDuration *string
		wantDuration   *string
	}{
		{
			name:           "with CA cert duration",
			jsonBody:       `{"mspId":"TestMSP","name":"TestOrg","caCertValidFor":"87600h"}`,
			wantCaDuration: strPtr("87600h"),
			wantDuration:   nil,
		},
		{
			name:           "with cert duration",
			jsonBody:       `{"mspId":"TestMSP","name":"TestOrg","certValidFor":"8760h"}`,
			wantCaDuration: nil,
			wantDuration:   strPtr("8760h"),
		},
		{
			name:           "with both durations",
			jsonBody:       `{"mspId":"TestMSP","name":"TestOrg","caCertValidFor":"87600h","certValidFor":"8760h"}`,
			wantCaDuration: strPtr("87600h"),
			wantDuration:   strPtr("8760h"),
		},
		{
			name:           "without durations",
			jsonBody:       `{"mspId":"TestMSP","name":"TestOrg"}`,
			wantCaDuration: nil,
			wantDuration:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req CreateOrganizationRequest
			err := json.NewDecoder(bytes.NewBufferString(tt.jsonBody)).Decode(&req)
			if err != nil {
				t.Fatalf("Failed to decode JSON: %v", err)
			}

			// Check CaCertValidFor
			if tt.wantCaDuration == nil {
				if req.CaCertValidFor != nil {
					t.Errorf("Expected CaCertValidFor to be nil, got %s", *req.CaCertValidFor)
				}
			} else {
				if req.CaCertValidFor == nil {
					t.Errorf("Expected CaCertValidFor to be %s, got nil", *tt.wantCaDuration)
				} else if *req.CaCertValidFor != *tt.wantCaDuration {
					t.Errorf("Expected CaCertValidFor to be %s, got %s", *tt.wantCaDuration, *req.CaCertValidFor)
				}
			}

			// Check CertValidFor
			if tt.wantDuration == nil {
				if req.CertValidFor != nil {
					t.Errorf("Expected CertValidFor to be nil, got %s", *req.CertValidFor)
				}
			} else {
				if req.CertValidFor == nil {
					t.Errorf("Expected CertValidFor to be %s, got nil", *tt.wantDuration)
				} else if *req.CertValidFor != *tt.wantDuration {
					t.Errorf("Expected CertValidFor to be %s, got %s", *tt.wantDuration, *req.CertValidFor)
				}
			}
		})
	}
}

// strPtr is a helper to create a pointer to a string
func strPtr(s string) *string {
	return &s
}
