# zs-core-fhir-validator

> **ZarishSphere Platform** · [github.com/orgs/zarishsphere](https://github.com/orgs/zarishsphere)

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](../../LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26.1-00ADD8?logo=go)](https://golang.org)
[![FHIR R5](https://img.shields.io/badge/FHIR-R5%205.0.0-orange)](https://hl7.org/fhir/R5/)
[![CI](https://github.com/zarishsphere/zs-core-fhir-validator/actions/workflows/ci.yml/badge.svg)](https://github.com/zarishsphere/zs-core-fhir-validator/actions)

Standalone FHIR R5 validation service. Validates resources against structural rules, ZarishSphere profile constraints, terminology codes (ICD-11, SNOMED, LOINC), and FHIR invariants. Used by CI/CD and zs-core-fhir-engine.

---

## Quick start

```bash
# Run locally (requires Go 1.26.1)
make dev

# Run tests
make test

# Build binary
make build

# Build multi-arch Docker image (amd64 + arm64 / Raspberry Pi 5)
make docker-build
```

## API

| Path | Method | Description |
|------|--------|-------------|
| `/healthz` | GET | Liveness probe |
| `/readyz` | GET | Readiness probe |
| `/metrics` | GET | Prometheus metrics |

Listening on port **8086** by default. Override with `SERVER_ADDR=:PORT`.

---

**Part of ZarishSphere** · Apache 2.0 · Free forever · [platform@zarishsphere.com](mailto:platform@zarishsphere.com)
