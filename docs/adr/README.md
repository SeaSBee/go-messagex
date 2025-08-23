# Architectural Decision Records (ADRs)

This directory contains Architectural Decision Records for the go-messagex project.

## What are ADRs?

Architectural Decision Records (ADRs) are short text documents that capture important architectural decisions made along with their context, consequences, and status.

## ADR Format

Each ADR follows this format:

- **Title**: A short descriptive title
- **Status**: Proposed, Accepted, Deprecated, Superseded
- **Context**: The problem being addressed
- **Decision**: The decision that was made
- **Consequences**: The resulting context after applying the decision

## ADRs

- [ADR-001: Transport-agnostic core + plug-in transports](./001-transport-agnostic-core.md)
- [ADR-002: Pooling strategy (few connections, many channels)](./002-pooling-strategy.md)
- [ADR-003: Backpressure policy (bounded queue + configurable drop/block)](./003-backpressure-policy.md)
- [ADR-004: Observability strategy (go-logx + OpenTelemetry metrics/tracing)](./004-observability-strategy.md)
- [ADR-005: Config precedence (ENV > YAML), schema and validation](./005-config-precedence.md)
- [ADR-006: Error handling (retries with capped exponential backoff + jitter)](./006-error-handling.md)

## Contributing

When making significant architectural decisions:

1. Create a new ADR file
2. Follow the established format
3. Update this README with the new ADR
4. Submit for review

## References

- [ADR Tools](https://adr.github.io/)
- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
