# Contributing to AI Infrastructure Agent

Thank you for your interest in contributing! This document provides guidelines for contributing to the AI Infrastructure Agent project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Environment Setup](#development-environment-setup)
- [Coding Standards](#coding-standards)
- [Commit Message Conventions](#commit-message-conventions)
- [Pull Request Process](#pull-request-process)
- [Reporting Security Vulnerabilities](#reporting-security-vulnerabilities)

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## Getting Started

1. **Fork the repository** and clone your fork locally.
2. **Create a branch** for your work: `git checkout -b feat/your-feature-name` or `fix/your-bug-fix`.
3. **Make your changes** following our [coding standards](#coding-standards).
4. **Run tests** before submitting: `go test ./...`
5. **Submit a pull request** using our [PR template](.github/PULL_REQUEST_TEMPLATE.md).

## Development Environment Setup

### Prerequisites

- **Go 1.24+** – [Install Go](https://golang.org/doc/install)
- **Docker** (optional) – For containerized runs
- **AWS credentials** – For testing AWS operations (use a dev account or localstack)

### Setup Steps

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/ai-infrastructure-agent.git
cd ai-infrastructure-agent

# Add upstream remote
git remote add upstream https://github.com/VersusControl/ai-infrastructure-agent.git

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o bin/agent ./cmd/agent
```

### Environment Variables

For local development, you may need:

- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` – AWS credentials
- `OPENAI_API_KEY` or `GEMINI_API_KEY` or `ANTHROPIC_API_KEY` – AI provider API key
- `CONFIG_PATH` – Path to `config.yaml` (optional)

See [docs/api-key-setup/](docs/api-key-setup/) for provider-specific setup.

## Coding Standards

### Go Style

- Follow [Effective Go](https://golang.org/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).
- Use `gofmt` for formatting: `gofmt -s -w .`
- Use `go vet`: `go vet ./...`
- Prefer `camelCase` for variables and functions, `PascalCase` for exported symbols.

### Project Structure

- `cmd/` – Application entrypoints
- `pkg/` – Reusable packages
- `internal/` – Private application code
- `docs/` – Documentation

### Testing

- Write unit tests for new logic.
- Use table-driven tests where appropriate.
- Mock AWS services when testing without real credentials.

## Commit Message Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

**Examples:**

- `feat(ec2): add instance tagging support`
- `fix(mcp): handle empty tool response`
- `docs: update Gemini API setup guide`

## Pull Request Process

1. Ensure your branch is up to date with `upstream/main`.
2. Fill out the [PR template](.github/PULL_REQUEST_TEMPLATE.md).
3. Link related issues (e.g., `Closes #123`).
4. Request review from maintainers.
5. Address feedback promptly.
6. Once approved, maintainers will merge.

## Reporting Security Vulnerabilities

**Do not** open a public issue for security vulnerabilities.

Please report security issues to the maintainers privately. Include:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

We will acknowledge receipt and work with you on a fix before any public disclosure.

## Questions?

Open a [Discussion](https://github.com/VersusControl/ai-infrastructure-agent/discussions) or an issue with the `question` label.
