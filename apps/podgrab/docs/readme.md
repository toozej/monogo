# Podgrab Documentation

Welcome to the Podgrab documentation. This comprehensive guide covers
architecture, deployment, API usage, and development.

## 📚 Documentation Structure

### Architecture

- **[Overview](architecture/overview.md)** - High-level system architecture and
  component interactions
- **[System Design](architecture/system-design.md)** - Detailed design patterns
  and architectural decisions
- **[Data Flow](architecture/data-flow.md)** - How data flows through the system
- **[Database Schema](architecture/database-schema.md)** - Database structure
  and relationships

### Deployment

- **[Docker Deployment](deployment/docker.md)** - Running Podgrab with Docker
  and docker-compose
- **[Production Deployment](deployment/production.md)** - Production deployment
  best practices
- **[Ubuntu Installation](ubuntu-install.md)** - Native Ubuntu installation
  guide

### API Documentation

- **[REST API](api/rest-api.md)** - Complete REST API reference
- **[WebSocket API](api/websocket.md)** - Real-time WebSocket communication

### Testing & Quality

- **[Testing Guide](testing.md)** - Comprehensive testing documentation (unit,
  integration, E2E)
- **[CI/CD Guide](ci-cd.md)** - GitHub Actions workflows and automation
- **[Pre-commit Hooks](development/pre-commit.md)** - Code quality automation

### Development

- **[Development Setup](development/setup.md)** - Setting up your development
  environment
- **[Contributing Guide](../CONTRIBUTING.md)** - How to contribute to Podgrab
- **[Logging Guide](../internal/logger/README.md)** - Structured logging with
  Uber Zap

### User Guides

- **[User Guide](guides/user-guide.md)** - Complete user guide for Podgrab
- **[Configuration Guide](guides/configuration.md)** - Configuration options and
  settings

## 🚀 Quick Start

New to Podgrab? Start here:

1. Read the [Overview](architecture/overview.md) to understand what Podgrab does
1. Follow the [Docker Deployment](deployment/docker.md) guide to get it running
1. Check the [User Guide](guides/user-guide.md) to learn how to use it
1. Review the [Configuration Guide](guides/configuration.md) to customize
   settings

## 🛠️ For Developers

Contributing to Podgrab? Check these docs:

1. [System Design](architecture/system-design.md) - Understand the codebase
   architecture
1. [Development Setup](development/setup.md) - Set up your dev environment
1. [Contributing Guide](../CONTRIBUTING.md) - Learn our development workflow
1. [Testing Guide](testing.md) - Testing strategies and coverage requirements
1. [CI/CD Guide](ci-cd.md) - GitHub Actions workflows and automation
1. [REST API](api/rest-api.md) - Understand the API endpoints

## 📖 Additional Resources

- **[Main README](../README.md)** - Project overview and quick start
- **[CLAUDE.md](../CLAUDE.md)** - AI assistant guide for working with this
  codebase

## 🔍 Navigation Tips

- All diagrams use [Mermaid](https://mermaid.js.github.io/) for easy rendering
  on GitHub
- Code examples are tested against the current codebase
- Each document includes links to related documentation

## 📝 Documentation Standards

This documentation follows these principles:

- **Accuracy**: All information is verified against the current codebase
- **Clarity**: Complex concepts are explained with diagrams and examples
- **Completeness**: All features and components are documented
- **Maintainability**: Documentation is updated with code changes

## ❓ Need Help?

- **Issues**: Check [GitHub Issues](https://github.com/akhilrex/podgrab/issues)
- **Discussions**: Join
  [GitHub Discussions](https://github.com/akhilrex/podgrab/discussions)
- **Security**: Report security issues privately to the maintainers

______________________________________________________________________

**Last Updated**: 2026-02-01 **Version**: 2022.07.07
