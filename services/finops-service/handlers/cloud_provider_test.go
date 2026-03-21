package handlers

import (
	"testing"
	"time"
)

// TestNewCloudProvider_ValidProviders verifies that valid provider names return a non-nil provider.
func TestNewCloudProvider_ValidProviders(t *testing.T) {
	for _, provider := range []string{"aws", "azure", "gcp"} {
		cp, err := NewCloudProvider(provider)
		if err != nil {
			t.Errorf("expected no error for provider %q, got %v", provider, err)
		}
		if cp == nil {
			t.Errorf("expected non-nil provider for %q", provider)
		}
	}
}

// TestNewCloudProvider_InvalidProvider verifies that an unsupported provider returns an error.
func TestNewCloudProvider_InvalidProvider(t *testing.T) {
	_, err := NewCloudProvider("digitalocean")
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

// TestAWSCredentialValidation_Valid verifies that valid AWS credentials pass validation.
func TestAWSCredentialValidation_Valid(t *testing.T) {
	cp, _ := NewCloudProvider("aws")
	err := cp.ValidateCredentials(map[string]string{
		"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
		"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	})
	if err != nil {
		t.Errorf("expected valid AWS credentials to pass, got: %v", err)
	}
}

// TestAWSCredentialValidation_MissingKey verifies that missing access_key_id returns an error.
func TestAWSCredentialValidation_MissingKey(t *testing.T) {
	cp, _ := NewCloudProvider("aws")
	err := cp.ValidateCredentials(map[string]string{
		"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	})
	if err == nil {
		t.Error("expected error for missing access_key_id")
	}
}

// TestAWSCredentialValidation_MissingSecret verifies that missing secret_access_key returns an error.
func TestAWSCredentialValidation_MissingSecret(t *testing.T) {
	cp, _ := NewCloudProvider("aws")
	err := cp.ValidateCredentials(map[string]string{
		"access_key_id": "AKIAIOSFODNN7EXAMPLE",
	})
	if err == nil {
		t.Error("expected error for missing secret_access_key")
	}
}

// TestAzureCredentialValidation_Valid verifies that valid Azure credentials pass validation.
func TestAzureCredentialValidation_Valid(t *testing.T) {
	cp, _ := NewCloudProvider("azure")
	err := cp.ValidateCredentials(map[string]string{
		"subscription_id": "sub-123",
		"tenant_id":       "tenant-456",
		"client_id":       "client-789",
		"client_secret":   "secret-abc",
	})
	if err != nil {
		t.Errorf("expected valid Azure credentials to pass, got: %v", err)
	}
}

// TestAzureCredentialValidation_MissingField verifies that missing any required Azure field returns an error.
func TestAzureCredentialValidation_MissingField(t *testing.T) {
	cp, _ := NewCloudProvider("azure")
	// Missing client_secret
	err := cp.ValidateCredentials(map[string]string{
		"subscription_id": "sub-123",
		"tenant_id":       "tenant-456",
		"client_id":       "client-789",
	})
	if err == nil {
		t.Error("expected error for missing client_secret")
	}
}

// TestGCPCredentialValidation_Valid verifies that valid GCP credentials pass validation.
func TestGCPCredentialValidation_Valid(t *testing.T) {
	cp, _ := NewCloudProvider("gcp")
	err := cp.ValidateCredentials(map[string]string{
		"project_id":          "my-project",
		"service_account_key": `{"type":"service_account"}`,
	})
	if err != nil {
		t.Errorf("expected valid GCP credentials to pass, got: %v", err)
	}
}

// TestGCPCredentialValidation_MissingProjectID verifies that missing project_id returns an error.
func TestGCPCredentialValidation_MissingProjectID(t *testing.T) {
	cp, _ := NewCloudProvider("gcp")
	err := cp.ValidateCredentials(map[string]string{
		"service_account_key": `{"type":"service_account"}`,
	})
	if err == nil {
		t.Error("expected error for missing project_id")
	}
}

// TestFetchCosts_ReturnsEntries verifies that FetchCosts returns cost entries for a date range.
func TestFetchCosts_ReturnsEntries(t *testing.T) {
	for _, provider := range []string{"aws", "azure", "gcp"} {
		cp, _ := NewCloudProvider(provider)
		start := time.Now().AddDate(0, 0, -3)
		end := time.Now()
		entries, err := cp.FetchCosts(map[string]string{}, start, end)
		if err != nil {
			t.Errorf("FetchCosts for %s returned error: %v", provider, err)
		}
		if len(entries) == 0 {
			t.Errorf("expected cost entries for %s, got none", provider)
		}
		// Verify all entries have required fields
		for _, e := range entries {
			if e.Service == "" {
				t.Errorf("%s: entry has empty service name", provider)
			}
			if e.Currency == "" {
				t.Errorf("%s: entry has empty currency", provider)
			}
			if e.Amount < 0 {
				t.Errorf("%s: entry has negative amount: %f", provider, e.Amount)
			}
		}
	}
}
