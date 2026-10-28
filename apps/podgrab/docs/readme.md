# Podgrab Documentation

Welcome to the Podgrab documentation. This guide covers
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

- **[Docker Deployment](deployment/docker.md)** - Run Podgrab with Docker
  and docker-compose
- **[Production Deployment](deployment/production.md)** - Production deployment
  recommended practices
- **[Ubuntu Installation](ubuntu-install.md)** - Guide for native installation on Ubuntu

### API Documentation

- **[REST API](api/rest-api.md)** - Complete reference for the REST API
- **[WebSocket API](api/websocket.md)** - Real-time WebSocket communication

### Tests & Quality

- **[Test Guide](testing.md)** - test documentation (unit,
  integration, E2E)
- **[CI/CD Guide](ci-cd.md)** - GitHub Actions workflows and automation
- **[Pre-commit Hooks](development/pre-commit.md)** - Code quality automation

### Development

- **[Development Setup](development/setup.md)** - Configure your development environment
- **[Contribution Guide](development/setup.md#contribute-workflow)** - How to contribute to Podgrab
- **[Logs Guide](../internal/logger/README.md)** - Structured log output with
  Uber Zap

### User Guides

- **[User Guide](guides/user-guide.md)** - Complete user guide for Podgrab
- **[Configuration Guide](guides/configuration.md)** - Configuration options and
  settings

## 🚀 Quick Start

For a first installation:

1. Read the [Overview](architecture/overview.md) to understand what Podgrab does
1. Follow the [Docker Deployment](deployment/docker.md) guide to start the application
1. Check the [User Guide](guides/user-guide.md) to learn how to use it
1. Review the [Configuration Guide](guides/configuration.md) to customize
   settings

## 🛠️ For Developers

To contribute to Podgrab, check these docs:

1. [System Design](architecture/system-design.md) - Understand the codebase
   architecture
1. [Development Setup](development/setup.md) - Set up your dev environment
1. [Contribution Guide](development/setup.md#contribute-workflow) - Learn our development workflow
1. [Test Guide](testing.md) - Tests strategies and coverage requirements
1. [CI/CD Guide](ci-cd.md) - GitHub Actions workflows and automation
1. [REST API](api/rest-api.md) - Understand the API endpoints

## 📖 Additional Resources

- **[Main README](../README.md)** - Project overview and quick start
- **[AGENTS.md](../AGENTS.md)** - AI assistant guide for tasks in this
  codebase

## 🔍 Navigation Tips

- All diagrams use [Mermaid](https://mermaid.js.org/) to simplify diagram display
  on GitHub
- Tests check code examples against the current codebase
- Each document includes links to related documentation

## 📝 Documentation Standards

This documentation follows these principles:

- **Accuracy**: Checks compare all information against the current codebase
- **Clarity**: Diagrams and examples explain complex concepts
- **Completeness**: Documentation covers all features and components
- **Maintainability**: Documentation updates accompany code changes

## ❓ Need Help?

- **Issues**: Check [GitHub Issues](https://github.com/akhilrex/podgrab/issues)
- **Discussions**: Join
  [GitHub Discussions](https://github.com/akhilrex/podgrab/discussions)
- **Security**: Report security issues privately to the maintainers

______________________________________________________________________

**Last Updated**: 2026-02-01 **Version**: 2022.07.07
