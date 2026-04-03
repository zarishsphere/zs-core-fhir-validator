package rules_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zarishsphere/zs-core-fhir-validator/internal/rules"
)

var v = &rules.StructuralValidator{}

// --------------------------------------------------------------------------
// Patient validation
// --------------------------------------------------------------------------

func TestValidatePatient_Valid(t *testing.T) {
	patient := map[string]any{
		"resourceType": "Patient",
		"id":           "test-patient-001",
		"meta": map[string]any{
			"lastUpdated": "2026-01-15T10:00:00Z",
		},
		"extension": []any{
			map[string]any{
				"url":         "https://fhir.zarishsphere.com/StructureDefinition/ext/tenant-id",
				"valueString": "cpi:bgd-health:camp-1w",
			},
		},
		"active": true,
		"name": []any{
			map[string]any{"use": "official", "family": "Rahman", "given": []any{"Abdul"}},
		},
		"gender":    "male",
		"birthDate": "1985-04-12",
	}

	result := v.Validate(patient)
	assert.Equal(t, 0, result.Errors, "valid patient should have no errors")
	assert.True(t, result.IsValid())
}

func TestValidatePatient_MissingResourceType(t *testing.T) {
	resource := map[string]any{"id": "orphan"}
	result := v.Validate(resource)
	assert.Greater(t, result.Errors, 0)
	assert.False(t, result.IsValid())
	// Should have a required error for resourceType
	found := false
	for _, issue := range result.Issues {
		if issue.Code == rules.CodeRequired && issue.Severity == rules.SeverityError {
			found = true
		}
	}
	assert.True(t, found, "should have required error for missing resourceType")
}

func TestValidatePatient_InvalidGender(t *testing.T) {
	patient := map[string]any{
		"resourceType": "Patient",
		"id":           "pt-bad-gender",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"active":       true,
		"gender":       "unknown-invalid",
		"birthDate":    "1990-01-01",
	}

	result := v.Validate(patient)
	assert.Greater(t, result.Errors, 0)
	found := false
	for _, issue := range result.Issues {
		if issue.Code == rules.CodeCodeInvalid {
			found = true
			assert.Contains(t, issue.Diagnostics, "gender")
		}
	}
	assert.True(t, found, "should have code-invalid error for bad gender")
}

func TestValidatePatient_ValidGenders(t *testing.T) {
	for _, gender := range []string{"male", "female", "other", "unknown"} {
		patient := map[string]any{
			"resourceType": "Patient",
			"id":           "pt-gender-" + gender,
			"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
			"active":       true,
			"gender":       gender,
		}
		result := v.Validate(patient)
		genderErrors := 0
		for _, issue := range result.Issues {
			if issue.Code == rules.CodeCodeInvalid {
				genderErrors++
			}
		}
		assert.Equal(t, 0, genderErrors, "gender '%s' should be valid", gender)
	}
}

func TestValidatePatient_InvalidBirthDate(t *testing.T) {
	patient := map[string]any{
		"resourceType": "Patient",
		"id":           "pt-bad-dob",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"active":       true,
		"birthDate":    "not-a-date",
	}

	result := v.Validate(patient)
	assert.Greater(t, result.Errors, 0)
}

func TestValidatePatient_ValidBirthDateFormats(t *testing.T) {
	formats := []string{"2024", "2024-01", "2024-01-15"}
	for _, bd := range formats {
		patient := map[string]any{
			"resourceType": "Patient",
			"id":           "pt-bd",
			"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
			"active":       true,
			"birthDate":    bd,
		}
		result := v.Validate(patient)
		bdErrors := 0
		for _, issue := range result.Issues {
			if issue.Code == rules.CodeValue {
				for _, expr := range issue.Expression {
					if expr == "Patient.birthDate" {
						bdErrors++
					}
				}
			}
		}
		assert.Equal(t, 0, bdErrors, "birthDate '%s' should be valid", bd)
	}
}

func TestValidatePatient_IDTooLong(t *testing.T) {
	longID := ""
	for i := 0; i < 65; i++ {
		longID += "a"
	}
	patient := map[string]any{
		"resourceType": "Patient",
		"id":           longID,
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"active":       true,
	}

	result := v.Validate(patient)
	assert.Greater(t, result.Errors, 0)
	found := false
	for _, issue := range result.Issues {
		if issue.Code == rules.CodeValue {
			found = true
		}
	}
	assert.True(t, found, "long ID should produce value error")
}

func TestValidatePatient_ExtensionMissingURL(t *testing.T) {
	patient := map[string]any{
		"resourceType": "Patient",
		"id":           "pt-bad-ext",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"extension": []any{
			map[string]any{
				// Missing url!
				"valueString": "some-value",
			},
		},
	}
	result := v.Validate(patient)
	assert.Greater(t, result.Errors, 0)
}

func TestValidatePatient_ExtensionNonAbsoluteURL(t *testing.T) {
	patient := map[string]any{
		"resourceType": "Patient",
		"id":           "pt-rel-ext",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"extension": []any{
			map[string]any{
				"url":         "relative/path/extension",
				"valueString": "value",
			},
		},
	}
	result := v.Validate(patient)
	assert.Greater(t, result.Errors, 0)
}

// --------------------------------------------------------------------------
// Encounter validation
// --------------------------------------------------------------------------

func TestValidateEncounter_Valid(t *testing.T) {
	encounter := map[string]any{
		"resourceType": "Encounter",
		"id":           "enc-001",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"status":       "finished",
		"class": []any{
			map[string]any{
				"coding": []any{
					map[string]any{
						"system":  "http://terminology.hl7.org/CodeSystem/v3-ActCode",
						"code":    "AMB",
						"display": "ambulatory",
					},
				},
			},
		},
		"subject": map[string]any{"reference": "Patient/test-001"},
	}

	result := v.Validate(encounter)
	assert.Equal(t, 0, result.Errors, "valid encounter should have no errors")
}

func TestValidateEncounter_MissingStatus(t *testing.T) {
	encounter := map[string]any{
		"resourceType": "Encounter",
		"id":           "enc-no-status",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"class":        []any{map[string]any{"coding": []any{map[string]any{"code": "AMB"}}}},
		"subject":      map[string]any{"reference": "Patient/test-001"},
	}

	result := v.Validate(encounter)
	assert.Greater(t, result.Errors, 0)
}

func TestValidateEncounter_MissingClass(t *testing.T) {
	encounter := map[string]any{
		"resourceType": "Encounter",
		"id":           "enc-no-class",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"status":       "finished",
		"subject":      map[string]any{"reference": "Patient/test-001"},
	}
	result := v.Validate(encounter)
	assert.Greater(t, result.Errors, 0)
}

// --------------------------------------------------------------------------
// Observation validation
// --------------------------------------------------------------------------

func TestValidateObservation_Valid(t *testing.T) {
	obs := map[string]any{
		"resourceType": "Observation",
		"id":           "obs-001",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"status":       "final",
		"code":         map[string]any{"coding": []any{map[string]any{"system": "http://loinc.org", "code": "8867-4"}}},
		"subject":      map[string]any{"reference": "Patient/test-001"},
		"valueQuantity": map[string]any{
			"value": 72, "unit": "beats/min", "system": "http://unitsofmeasure.org", "code": "/min",
		},
	}

	result := v.Validate(obs)
	assert.Equal(t, 0, result.Errors)
}

func TestValidateObservation_DataAbsentReasonWithValue_Invariant(t *testing.T) {
	// FHIR invariant obs-6: dataAbsentReason SHALL only be present if value[x] absent
	obs := map[string]any{
		"resourceType":  "Observation",
		"id":            "obs-inv",
		"meta":          map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"status":        "final",
		"code":          map[string]any{"text": "Heart rate"},
		"subject":       map[string]any{"reference": "Patient/test-001"},
		"valueQuantity": map[string]any{"value": 72, "unit": "beats/min"},
		"dataAbsentReason": map[string]any{
			"coding": []any{map[string]any{"code": "unknown"}},
		},
	}

	result := v.Validate(obs)
	assert.Greater(t, result.Errors, 0)
	found := false
	for _, issue := range result.Issues {
		if issue.Code == rules.CodeInvariant {
			found = true
			assert.Contains(t, issue.Diagnostics, "obs-6")
		}
	}
	assert.True(t, found, "should have obs-6 invariant error")
}

// --------------------------------------------------------------------------
// Bundle validation
// --------------------------------------------------------------------------

func TestValidateBundle_Valid(t *testing.T) {
	bundle := map[string]any{
		"resourceType": "Bundle",
		"id":           "bundle-001",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"type":         "transaction",
		"entry":        []any{},
	}
	result := v.Validate(bundle)
	assert.Equal(t, 0, result.Errors)
}

func TestValidateBundle_InvalidType(t *testing.T) {
	bundle := map[string]any{
		"resourceType": "Bundle",
		"id":           "bundle-bad-type",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"type":         "not-a-valid-bundle-type",
	}
	result := v.Validate(bundle)
	assert.Greater(t, result.Errors, 0)
}

func TestValidateBundle_SubscriptionNotification(t *testing.T) {
	bundle := map[string]any{
		"resourceType": "Bundle",
		"id":           "notif-bundle-001",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"type":         "subscription-notification",
		"timestamp":    "2026-01-15T10:00:00Z",
	}
	result := v.Validate(bundle)
	assert.Equal(t, 0, result.Errors, "subscription-notification is a valid R5 bundle type")
}

// --------------------------------------------------------------------------
// OperationOutcome output
// --------------------------------------------------------------------------

func TestValidationResult_ToOperationOutcome_Errors(t *testing.T) {
	resource := map[string]any{"resourceType": "Patient", "id": "pt-test", "meta": map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"}, "gender": "invalid-gender"}
	result := v.Validate(resource)
	outcome := result.ToOperationOutcome()

	assert.Equal(t, "OperationOutcome", outcome["resourceType"])
	issues, ok := outcome["issue"].([]map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, issues)

	// All issues should have severity and code
	for _, issue := range issues {
		assert.NotEmpty(t, issue["severity"])
		assert.NotEmpty(t, issue["code"])
		assert.NotEmpty(t, issue["diagnostics"])
	}
}

func TestValidationResult_ToOperationOutcome_Valid(t *testing.T) {
	resource := map[string]any{
		"resourceType": "Patient",
		"id":           "pt-valid",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"extension": []any{
			map[string]any{
				"url":         "https://fhir.zarishsphere.com/StructureDefinition/ext/tenant-id",
				"valueString": "cpi:bgd-health:camp-1w",
			},
		},
		"active": true,
		"gender": "female",
	}
	result := v.Validate(resource)
	require.True(t, result.IsValid())
	outcome := result.ToOperationOutcome()

	issues, _ := outcome["issue"].([]map[string]any)
	require.Len(t, issues, 1)
	assert.Equal(t, "information", issues[0]["severity"])
	assert.Equal(t, "Validation passed", issues[0]["diagnostics"])
}

// --------------------------------------------------------------------------
// ZarishSphere-specific validation: AuditEvent
// --------------------------------------------------------------------------

func TestValidateAuditEvent_Valid(t *testing.T) {
	audit := map[string]any{
		"resourceType": "AuditEvent",
		"id":           "audit-001",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		"action":       "R",
		"recorded":     "2026-01-15T10:00:00Z",
		"agent": []any{
			map[string]any{
				"requestor": true,
				"who":       map[string]any{"reference": "Practitioner/prac-001"},
			},
		},
		"source": map[string]any{
			"observer": map[string]any{"display": "ZarishSphere FHIR Engine"},
		},
	}

	result := v.Validate(audit)
	assert.Equal(t, 0, result.Errors)
}

func TestValidateAuditEvent_MissingAction(t *testing.T) {
	audit := map[string]any{
		"resourceType": "AuditEvent",
		"id":           "audit-no-action",
		"meta":         map[string]any{"lastUpdated": "2026-01-15T10:00:00Z"},
		// action is missing
		"recorded": "2026-01-15T10:00:00Z",
		"agent":    []any{},
		"source":   map[string]any{"observer": map[string]any{"display": "ZS Engine"}},
	}

	result := v.Validate(audit)
	assert.Greater(t, result.Errors, 0, "Missing AuditEvent.action should be an error")
}
