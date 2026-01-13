# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

TestPlane is a Kubernetes Operator built with Kubebuilder that provides declarative infrastructure testing capabilities for cloud platform resources. It manages test lifecycles through CRDs, supporting both integration tests and load tests.

**Key CRDs:**
- `IntegrationTest` - Sequential/parallel test steps with expectations
- `LoadTest` - Long-running load tests with periodic health checks

## Tech Stack

- **Language**: Go 1.24+
- **Framework**: Kubebuilder / controller-runtime v0.21
- **Testing**: Ginkgo v2 + Gomega
- **Linting**: golangci-lint v2
- **Container**: Docker

## Common Commands

```bash
# Development
make run              # Run controller locally
make build            # Build manager binary
make manifests        # Generate CRD manifests (run after API changes)
make generate         # Generate DeepCopy methods (run after API changes)

# Testing
make test             # Run unit tests
make test-e2e         # Run e2e tests (requires Kind)

# Code Quality
make fmt              # Format code
make vet              # Run go vet
make lint             # Run golangci-lint
make lint-fix         # Auto-fix lint issues

# Deployment
make install          # Install CRDs to cluster
make uninstall        # Remove CRDs from cluster
make deploy IMG=<image>    # Deploy controller
make undeploy         # Remove controller
```

## Project Structure

```
api/v1alpha1/           # CRD type definitions
  ├── integrationtest_types.go
  ├── loadtest_types.go
  └── common_types.go

internal/controller/framework/
  ├── integrationtest/  # IntegrationTest controller
  ├── loadtest/         # LoadTest controller
  ├── plugin/           # Expectation functions (assertions)
  │   ├── builtin.go    # Function registration
  │   └── functions.go  # Function implementations
  └── resource/         # Resource management utilities

config/
  ├── crd/bases/        # Generated CRD YAMLs
  └── samples/          # Example CR manifests

test/e2e/               # End-to-end tests
```

## Key Development Patterns

### Adding Expectation Functions

1. Implement function in `internal/controller/framework/plugin/functions.go`:
```go
func MyExpect(s Snapshot, p Params) Result {
    expected := p.String("expected")
    actual := s.Status().String("myField")
    return Check(actual == expected).Expected(expected).Actual(actual).Result()
}
```

2. Register in `internal/controller/framework/plugin/builtin.go`:
```go
registry.Register("MyExpect", MyExpect)
```

### After Modifying API Types

Always run both commands after changing files in `api/v1alpha1/`:
```bash
make manifests generate
```

## Code Conventions

- Follow standard Go conventions and Kubebuilder patterns
- Use controller-runtime's structured logging
- Keep controller reconciliation idempotent
- Write status updates to CR `.status` fields for observability
