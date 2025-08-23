# Contributing to go-messagex

Thank you for your interest in contributing to go-messagex! This document provides guidelines and information for contributors.

## 🚀 Quick Start

1. **Fork** the repository
2. **Clone** your fork locally
3. **Create** a feature branch
4. **Make** your changes
5. **Test** thoroughly
6. **Submit** a pull request

## 📋 Development Environment

### Prerequisites
- Go 1.24.5 or later
- Git
- Make (optional, for convenience commands)

### Setup
```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/go-messagex.git
cd go-messagex

# Install dependencies
go mod download

# Verify setup
make verify
```

## 🛠️ Development Workflow

### Code Quality Standards
- **Go Version**: 1.24.5
- **Formatting**: `gofmt -s` (enforced by CI)
- **Linting**: `golangci-lint` with production-grade rules
- **Testing**: All tests must pass with `-race` detector
- **Coverage**: Aim for 90%+ unit test coverage

### Makefile Targets
```bash
# Format code
make fmt

# Run linter
make lint

# Run tests with race detector
make test

# Run tests with coverage
make cover

# Run all checks (fmt, lint, test)
make verify

# Run CI pipeline locally
make ci

# Clean build artifacts
make clean
```

### Pre-commit Checklist
Before submitting a pull request, ensure:

- [ ] Code is formatted (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] All tests pass with race detector (`make test`)
- [ ] Test coverage is maintained
- [ ] Documentation is updated
- [ ] Observability hooks are added where appropriate

## 🧪 Testing Guidelines

### Unit Tests
- **Location**: `test/unit/` directory
- **Naming**: `*_test.go` files
- **Coverage**: 90%+ on core packages
- **Race Detection**: All tests must pass with `-race`

### Test Structure
```go
func TestFeatureName(t *testing.T) {
    // Arrange
    // Set up test data and mocks
    
    // Act
    // Execute the function being tested
    
    // Assert
    // Verify the results
}
```

### Mocking Guidelines
- Use lightweight interfaces for external dependencies
- Mock AMQP connections and channels for testing
- Avoid integration tests in unit test suite
- Use `testify` for assertions and mocking

### Performance Testing
```bash
# Run benchmarks
go test -bench=. ./pkg/rabbitmq/...

# Run with memory profiling
go test -bench=. -memprofile=mem.prof ./pkg/rabbitmq/...
```

## 📝 Code Style Guidelines

### Go Conventions
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt -s` for formatting
- Prefer composition over inheritance
- Use meaningful variable and function names
- Add comments for exported functions and types

### Error Handling
```go
// Good: Use %w for error wrapping
if err != nil {
    return fmt.Errorf("failed to publish message: %w", err)
}

// Good: Use error sentinels for specific errors
var ErrBackpressure = errors.New("backpressure limit exceeded")
```

### Logging
- Use `go-logx` directly (no wrappers)
- Include structured fields for observability
- Never log sensitive information
- Use appropriate log levels

```go
// Good: Structured logging with fields
logger.Info("message published",
    "exchange", exchange,
    "routing_key", routingKey,
    "message_id", msg.ID,
    "size_bytes", len(msg.Body),
)
```

### Concurrency
- Use `sync.Mutex`/`sync.RWMutex` for shared state
- Avoid sharing channels across goroutines without discipline
- Use context for cancellation and timeouts
- Test with race detector

## 🔧 Configuration and Environment

### Local Development
```bash
# Set development environment variables
export MSG_LOGGING_LEVEL=debug
export MSG_RABBITMQ_URIS=amqp://localhost:5672/

# Run example applications
go run cmd/publisher/main.go
go run cmd/consumer/main.go
```

### Testing Configuration
- Use `configs/messaging.example.yaml` as template
- Override with environment variables for testing
- Never commit sensitive credentials

## 📚 Documentation

### Code Documentation
- Document all exported functions and types
- Include usage examples for complex APIs
- Update README.md for user-facing changes
- Add inline comments for complex logic

### API Documentation
- Keep public API stable
- Use semantic versioning for breaking changes
- Document migration paths for API changes

## 🚀 Pull Request Process

### Before Submitting
1. **Rebase** on main branch
2. **Run** all tests locally
3. **Update** documentation if needed
4. **Squash** commits if appropriate

### Pull Request Template
```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests added/updated
- [ ] All tests pass with race detector
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Observability hooks added
```

### Review Process
1. **Automated Checks**: CI must pass
2. **Code Review**: At least one maintainer approval
3. **Testing**: All tests must pass
4. **Documentation**: Updated as needed

## 🐛 Bug Reports

### Before Reporting
1. Check existing issues
2. Try latest version
3. Reproduce with minimal example

### Bug Report Template
```markdown
## Description
Clear description of the bug

## Steps to Reproduce
1. Step 1
2. Step 2
3. Step 3

## Expected Behavior
What should happen

## Actual Behavior
What actually happens

## Environment
- Go version:
- OS:
- go-messagex version:

## Additional Context
Logs, configuration, etc.
```

## 🔒 Security

### Security Issues
- **DO NOT** create public issues for security vulnerabilities
- Email security issues to: security@seasbee.com
- Include detailed reproduction steps
- Allow time for assessment and fix

### Security Guidelines
- Never log sensitive information
- Use environment variables for secrets
- Validate all inputs
- Follow principle of least privilege

## 🏷️ Release Process

### Versioning
- Follow [Semantic Versioning](https://semver.org/)
- Use git tags for releases
- Update CHANGELOG.md for each release

### Release Checklist
- [ ] All tests pass
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version tagged
- [ ] Release notes prepared

## 🤝 Community Guidelines

### Code of Conduct
- Be respectful and inclusive
- Focus on technical merit
- Help others learn and grow
- Report inappropriate behavior

### Communication
- Use GitHub issues for technical discussions
- Be clear and concise
- Provide context and examples
- Respect others' time and expertise

## 📞 Getting Help

- **Issues**: [GitHub Issues](https://github.com/seasbee/go-messagex/issues)
- **Discussions**: [GitHub Discussions](https://github.com/seasbee/go-messagex/discussions)
- **Documentation**: [docs/](docs/)

## 🙏 Acknowledgments

Thank you for contributing to go-messagex! Your contributions help make this project better for everyone.

---

**Happy coding! 🚀**
