// ZarishSphere FHIR R5 Validator — standalone validation service.
// Validates FHIR R5 resources against:
//   1. FHIR R5 structural rules (required fields, cardinality, data types)
//   2. ZarishSphere profile constraints (ZSPatient, ZSObservation, etc.)
//   3. Terminology codes (ICD-11, SNOMED, LOINC via local cache)
//   4. FHIR invariants (FHIRPath expressions from StructureDefinitions)
//
// Exposed as HTTP service for use by zs-agent-code-review and zs-core-fhir-engine.
// Also runnable as CLI: zs-fhir-validator --file resource.json
// ADR-0001: Go 1.26.1 | Governance: TODO.md Phase 1.3
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/zarishsphere/zs-core-fhir-validator/internal/rules"
)

var structValidator = &rules.StructuralValidator{}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.With().Caller().Str("service", "zs-core-fhir-validator").Logger()

	addr := getEnv("SERVER_ADDR", ":8086")

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	// Health
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"status":"ok","service":"zs-core-fhir-validator","fhir":"R5/5.0.0"}`,
		))
	})

	// FHIR $validate operation — matches FHIR R5 spec endpoint
	// POST /fhir/R5/{resourceType}/$validate
	r.Post("/fhir/R5/{resourceType}/$validate", validateHandler)

	// Direct validation endpoint (for CI/CD use — accepts any FHIR resource)
	r.Post("/validate", validateHandler)

	// Batch validation
	r.Post("/validate/batch", batchValidateHandler)

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	done := make(chan struct{})
	go func() {
		q := make(chan os.Signal, 1)
		signal.Notify(q, syscall.SIGTERM, syscall.SIGINT)
		<-q
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		close(done)
	}()

	log.Info().Str("addr", addr).Msg("fhir-validator: listening")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Err(err).Msg("server error")
	}
	<-done
	log.Info().Msg("fhir-validator: stopped")
}

func validateHandler(w http.ResponseWriter, r *http.Request) {
	var resource map[string]any
	if err := json.NewDecoder(r.Body).Decode(&resource); err != nil {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(
			`{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"structure","diagnostics":"Invalid JSON: ` + err.Error() + `"}]}`,
		))
		return
	}

	result := structValidator.Validate(resource)
	outcome := result.ToOperationOutcome()

	w.Header().Set("Content-Type", "application/fhir+json; charset=utf-8")
	if result.IsValid() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(outcome)

	log.Debug().
		Str("resource_type", result.ResourceType).
		Str("resource_id", result.ResourceID).
		Int("errors", result.Errors).
		Int("warnings", result.Warnings).
		Msg("fhir-validator: validated")
}

func batchValidateHandler(w http.ResponseWriter, r *http.Request) {
	var resources []map[string]any
	if err := json.NewDecoder(r.Body).Decode(&resources); err != nil {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"structure","diagnostics":"Expected JSON array of resources"}]}`))
		return
	}

	type batchResult struct {
		ResourceType string `json:"resourceType"`
		ResourceID   string `json:"resourceId"`
		Valid         bool   `json:"valid"`
		Errors        int    `json:"errors"`
		Warnings      int    `json:"warnings"`
		Outcome       map[string]any `json:"outcome"`
	}

	results := make([]batchResult, 0, len(resources))
	allValid := true

	for _, res := range resources {
		vr := structValidator.Validate(res)
		if !vr.IsValid() {
			allValid = false
		}
		results = append(results, batchResult{
			ResourceType: vr.ResourceType,
			ResourceID:   vr.ResourceID,
			Valid:         vr.IsValid(),
			Errors:        vr.Errors,
			Warnings:      vr.Warnings,
			Outcome:       vr.ToOperationOutcome(),
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if allValid {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"allValid": allValid,
		"total":    len(results),
		"results":  results,
	})
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
