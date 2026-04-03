// Package rules implements FHIR R5 validation rules for ZarishSphere.
//
// Validation layers (applied in order):
//   1. Structure validation   — resourceType, required fields, cardinality
//   2. Type validation        — value[x] types, primitive types, URL formats
//   3. Terminology validation — code/system pairs against local terminology DB
//   4. Invariant validation   — FHIR-defined OCL/FHIRPath invariants
//   5. Profile validation     — ZarishSphere IG profile constraints
//
// Every validation result is an OperationOutcome with detailed issue reporting.
// Governance: TODO.md Phase 1.3 — zs-core-fhir-validator
package rules

import (
	"fmt"
	"strings"
	"time"
)

// Severity levels for validation issues.
type Severity string

const (
	SeverityError       Severity = "error"
	SeverityWarning     Severity = "warning"
	SeverityInformation Severity = "information"
	SeverityFatal       Severity = "fatal"
)

// IssueCode maps to FHIR OperationOutcome.issue.code.
type IssueCode string

const (
	CodeStructure    IssueCode = "structure"
	CodeRequired     IssueCode = "required"
	CodeValue        IssueCode = "value"
	CodeInvariant    IssueCode = "invariant"
	CodeSecurity     IssueCode = "security"
	CodeUnknown      IssueCode = "unknown"
	CodeNotSupported IssueCode = "not-supported"
	CodeCodeInvalid  IssueCode = "code-invalid"
	CodeExtension    IssueCode = "extension"
	CodeBusinessRule IssueCode = "business-rule"
)

// ValidationIssue represents one issue found during validation.
type ValidationIssue struct {
	Severity    Severity
	Code        IssueCode
	Diagnostics string
	Expression  []string // FHIRPath locations of the issue
	Details     *IssueCoding
}

// IssueCoding holds a coded representation of the issue.
type IssueCoding struct {
	System  string
	Code    string
	Display string
}

// ValidationResult holds all issues found by the validator.
type ValidationResult struct {
	ResourceType string
	ResourceID   string
	Issues       []ValidationIssue
	Errors       int
	Warnings     int
}

// IsValid returns true if no errors were found.
func (r *ValidationResult) IsValid() bool {
	return r.Errors == 0
}

// AddIssue appends an issue and increments counters.
func (r *ValidationResult) AddIssue(issue ValidationIssue) {
	r.Issues = append(r.Issues, issue)
	switch issue.Severity {
	case SeverityError, SeverityFatal:
		r.Errors++
	case SeverityWarning:
		r.Warnings++
	}
}

// Error adds an error-severity issue.
func (r *ValidationResult) Error(code IssueCode, diagnostics string, expressions ...string) {
	r.AddIssue(ValidationIssue{
		Severity:    SeverityError,
		Code:        code,
		Diagnostics: diagnostics,
		Expression:  expressions,
	})
}

// Warning adds a warning-severity issue.
func (r *ValidationResult) Warning(code IssueCode, diagnostics string, expressions ...string) {
	r.AddIssue(ValidationIssue{
		Severity:    SeverityWarning,
		Code:        code,
		Diagnostics: diagnostics,
		Expression:  expressions,
	})
}

// ToOperationOutcome converts the result to a FHIR R5 OperationOutcome map.
func (r *ValidationResult) ToOperationOutcome() map[string]any {
	issues := make([]map[string]any, 0, len(r.Issues))
	for _, issue := range r.Issues {
		i := map[string]any{
			"severity":    string(issue.Severity),
			"code":        string(issue.Code),
			"diagnostics": issue.Diagnostics,
		}
		if len(issue.Expression) > 0 {
			i["expression"] = issue.Expression
		}
		if issue.Details != nil {
			i["details"] = map[string]any{
				"coding": []map[string]any{
					{
						"system":  issue.Details.System,
						"code":    issue.Details.Code,
						"display": issue.Details.Display,
					},
				},
			}
		}
		issues = append(issues, i)
	}

	if len(issues) == 0 {
		issues = []map[string]any{{
			"severity":    "information",
			"code":        "informational",
			"diagnostics": "Validation passed",
		}}
	}

	return map[string]any{
		"resourceType": "OperationOutcome",
		"id":           "validation-result",
		"issue":        issues,
	}
}

// --------------------------------------------------------------------------
// Core structural validator
// --------------------------------------------------------------------------

// StructuralValidator validates the basic structure of a FHIR resource.
type StructuralValidator struct{}

// Validate runs structural validation on a FHIR resource.
func (v *StructuralValidator) Validate(resource map[string]any) *ValidationResult {
	rt, _ := resource["resourceType"].(string)
	id, _ := resource["id"].(string)
	result := &ValidationResult{ResourceType: rt, ResourceID: id}

	// Rule 1: resourceType is required and must be a non-empty string
	if rt == "" {
		result.Error(CodeRequired, "resourceType is required and must be a non-empty string", "resourceType")
		return result // Cannot continue without resource type
	}

	// Rule 2: id must be a valid FHIR ID (pattern: [A-Za-z0-9\-\.]{1,64})
	if id != "" {
		if len(id) > 64 {
			result.Error(CodeValue, fmt.Sprintf("id exceeds maximum length of 64 characters (got %d)", len(id)), rt+".id")
		}
		if !isValidFHIRID(id) {
			result.Error(CodeValue, "id contains invalid characters (allowed: [A-Za-z0-9\\-.]{1,64})", rt+".id")
		}
	}

	// Rule 3: meta.lastUpdated must be present (ZarishSphere mandatory)
	if meta, ok := resource["meta"].(map[string]any); ok {
		if lastUpdated, ok := meta["lastUpdated"].(string); ok {
			if _, err := time.Parse(time.RFC3339, lastUpdated); err != nil {
				result.Error(CodeValue, "meta.lastUpdated must be a valid dateTime (RFC3339 format)", rt+".meta.lastUpdated")
			}
		} else {
			result.Warning(CodeRequired, "meta.lastUpdated is recommended for all ZarishSphere resources", rt+".meta")
		}
	}

	// Rule 4: extension URL must be absolute URI
	if exts, ok := resource["extension"].([]any); ok {
		for i, ext := range exts {
			if em, ok := ext.(map[string]any); ok {
				if url, ok := em["url"].(string); !ok || url == "" {
					result.Error(CodeRequired, fmt.Sprintf("extension[%d].url is required", i), fmt.Sprintf("%s.extension[%d].url", rt, i))
				} else if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "urn:") {
					result.Error(CodeValue, fmt.Sprintf("extension[%d].url must be an absolute URI", i), fmt.Sprintf("%s.extension[%d].url", rt, i))
				}
			}
		}
	}

	// Apply resource-type-specific structural rules
	v.validateResourceType(rt, resource, result)

	return result
}

func (v *StructuralValidator) validateResourceType(rt string, r map[string]any, result *ValidationResult) {
	switch rt {
	case "Patient":
		v.validatePatient(r, result)
	case "Encounter":
		v.validateEncounter(r, result)
	case "Observation":
		v.validateObservation(r, result)
	case "Condition":
		v.validateCondition(r, result)
	case "MedicationRequest":
		v.validateMedicationRequest(r, result)
	case "Immunization":
		v.validateImmunization(r, result)
	case "AuditEvent":
		v.validateAuditEvent(r, result)
	case "Bundle":
		v.validateBundle(r, result)
	}
}

func (v *StructuralValidator) validatePatient(r map[string]any, result *ValidationResult) {
	// FHIR invariant pat-1: If a patient has a name, it should have family or given
	if names, ok := r["name"].([]any); ok {
		for i, name := range names {
			if nm, ok := name.(map[string]any); ok {
				_, hasFamily := nm["family"]
				_, hasGiven := nm["given"]
				_, hasText := nm["text"]
				if !hasFamily && !hasGiven && !hasText {
					result.Warning(CodeInvariant,
						fmt.Sprintf("Patient.name[%d] has no family, given, or text (inv: pat-1)", i),
						fmt.Sprintf("Patient.name[%d]", i))
				}
			}
		}
	}

	// Gender must be a valid code
	if gender, ok := r["gender"].(string); ok {
		validGenders := map[string]bool{"male": true, "female": true, "other": true, "unknown": true}
		if !validGenders[gender] {
			result.Error(CodeCodeInvalid,
				fmt.Sprintf("Patient.gender must be one of: male, female, other, unknown (got '%s')", gender),
				"Patient.gender")
		}
	}

	// birthDate must be valid date
	if bd, ok := r["birthDate"].(string); ok {
		if _, err := time.Parse("2006-01-02", bd); err != nil {
			if _, err2 := time.Parse("2006-01", bd); err2 != nil {
				if _, err3 := time.Parse("2006", bd); err3 != nil {
					result.Error(CodeValue,
						fmt.Sprintf("Patient.birthDate '%s' is not a valid FHIR date (YYYY, YYYY-MM, or YYYY-MM-DD)", bd),
						"Patient.birthDate")
				}
			}
		}
	}

	// ZarishSphere invariant: tenant-id extension required
	if !hasExtension(r, "https://fhir.zarishsphere.com/StructureDefinition/ext/tenant-id") {
		result.Warning(CodeExtension,
			"ZarishSphere ZSPatient profile requires tenant-id extension",
			"Patient.extension")
	}
}

func (v *StructuralValidator) validateEncounter(r map[string]any, result *ValidationResult) {
	// status is required
	if _, ok := r["status"]; !ok {
		result.Error(CodeRequired, "Encounter.status is required", "Encounter.status")
	}

	// class is required in R5
	if _, ok := r["class"]; !ok {
		result.Error(CodeRequired, "Encounter.class is required in FHIR R5", "Encounter.class")
	}

	// subject is required
	if _, ok := r["subject"]; !ok {
		result.Error(CodeRequired, "Encounter.subject is required", "Encounter.subject")
	}
}

func (v *StructuralValidator) validateObservation(r map[string]any, result *ValidationResult) {
	// status is required
	if _, ok := r["status"]; !ok {
		result.Error(CodeRequired, "Observation.status is required", "Observation.status")
	}

	// code is required
	if _, ok := r["code"]; !ok {
		result.Error(CodeRequired, "Observation.code is required", "Observation.code")
	}

	// FHIR invariant obs-6: dataAbsentReason SHALL only be present if value[x] is not present
	_, hasValue := findValueX(r)
	_, hasDAR := r["dataAbsentReason"]
	if hasValue && hasDAR {
		result.Error(CodeInvariant,
			"Observation.dataAbsentReason SHALL only be present if value[x] is absent (inv: obs-6)",
			"Observation.dataAbsentReason")
	}

	// FHIR invariant obs-7: If code is a panel code, there should be components or hasMember
	// Simplified check omitted — full implementation uses FHIRPath evaluator
}

func (v *StructuralValidator) validateCondition(r map[string]any, result *ValidationResult) {
	// clinicalStatus is required in R5
	if _, ok := r["clinicalStatus"]; !ok {
		result.Error(CodeRequired, "Condition.clinicalStatus is required in FHIR R5", "Condition.clinicalStatus")
	}

	// code or category required (at least one)
	_, hasCode := r["code"]
	_, hasCat := r["category"]
	if !hasCode && !hasCat {
		result.Warning(CodeRequired, "Condition should have either code or category", "Condition")
	}
}

func (v *StructuralValidator) validateMedicationRequest(r map[string]any, result *ValidationResult) {
	if _, ok := r["status"]; !ok {
		result.Error(CodeRequired, "MedicationRequest.status is required", "MedicationRequest.status")
	}
	if _, ok := r["intent"]; !ok {
		result.Error(CodeRequired, "MedicationRequest.intent is required", "MedicationRequest.intent")
	}
	if _, ok := r["medication"]; !ok {
		result.Error(CodeRequired, "MedicationRequest.medication is required (R5: CodeableReference)", "MedicationRequest.medication")
	}
	if _, ok := r["subject"]; !ok {
		result.Error(CodeRequired, "MedicationRequest.subject is required", "MedicationRequest.subject")
	}
}

func (v *StructuralValidator) validateImmunization(r map[string]any, result *ValidationResult) {
	if _, ok := r["status"]; !ok {
		result.Error(CodeRequired, "Immunization.status is required", "Immunization.status")
	}
	if _, ok := r["vaccineCode"]; !ok {
		result.Error(CodeRequired, "Immunization.vaccineCode is required", "Immunization.vaccineCode")
	}
	if _, ok := r["patient"]; !ok {
		result.Error(CodeRequired, "Immunization.patient is required", "Immunization.patient")
	}
	// occurrence[x] required
	_, hasOccurrenceDateTime := r["occurrenceDateTime"]
	_, hasOccurrenceString := r["occurrenceString"]
	if !hasOccurrenceDateTime && !hasOccurrenceString {
		result.Error(CodeRequired, "Immunization.occurrence[x] is required", "Immunization.occurrence[x]")
	}
}

func (v *StructuralValidator) validateAuditEvent(r map[string]any, result *ValidationResult) {
	// ZarishSphere mandatory: AuditEvent.action required
	if _, ok := r["action"]; !ok {
		result.Error(CodeRequired, "AuditEvent.action is required (ZarishSphere PHI audit requirement)", "AuditEvent.action")
	}
	if _, ok := r["recorded"]; !ok {
		result.Error(CodeRequired, "AuditEvent.recorded is required", "AuditEvent.recorded")
	}
	if _, ok := r["agent"]; !ok {
		result.Error(CodeRequired, "AuditEvent.agent is required (at least one)", "AuditEvent.agent")
	}
	if _, ok := r["source"]; !ok {
		result.Error(CodeRequired, "AuditEvent.source is required", "AuditEvent.source")
	}
}

func (v *StructuralValidator) validateBundle(r map[string]any, result *ValidationResult) {
	if _, ok := r["type"]; !ok {
		result.Error(CodeRequired, "Bundle.type is required", "Bundle.type")
		return
	}
	bundleType, _ := r["type"].(string)

	validTypes := map[string]bool{
		"document": true, "message": true, "transaction": true,
		"transaction-response": true, "batch": true, "batch-response": true,
		"history": true, "searchset": true, "collection": true,
		"subscription-notification": true,
	}
	if !validTypes[bundleType] {
		result.Error(CodeCodeInvalid,
			fmt.Sprintf("Bundle.type '%s' is not a valid FHIR R5 bundle type", bundleType),
			"Bundle.type")
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func isValidFHIRID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for _, ch := range id {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') ||
			(ch >= '0' && ch <= '9') || ch == '-' || ch == '.') {
			return false
		}
	}
	return true
}

func hasExtension(r map[string]any, url string) bool {
	exts, ok := r["extension"].([]any)
	if !ok {
		return false
	}
	for _, ext := range exts {
		if em, ok := ext.(map[string]any); ok {
			if em["url"] == url {
				return true
			}
		}
	}
	return false
}

func findValueX(r map[string]any) (any, bool) {
	valueKeys := []string{
		"valueQuantity", "valueCodeableConcept", "valueString",
		"valueBoolean", "valueInteger", "valueRange", "valueRatio",
		"valueSampledData", "valueTime", "valueDateTime", "valuePeriod",
	}
	for _, key := range valueKeys {
		if v, ok := r[key]; ok {
			return v, true
		}
	}
	return nil, false
}
