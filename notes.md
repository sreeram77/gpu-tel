# GPU Telemetry Project Development Notes

## Project Information
- **IDE Used**: IntelliJ GoLand
- **AI Assistant**: Windsurf/Cascade

## Table of Contents
1. [Development Workflow](#development-workflow)
2. [Project Bootstrapping](#1-project-bootstrapping)
3. [Code Implementation](#2-code-implementation)
4. [Testing](#3-unit-testing)
5. [Build Environment](#4-build-environment)

## Development Workflow

This document outlines the development workflow, highlighting aspects accelerated by AI versus manual implementation.

## 1. Project Bootstrapping

### Process
- Manually created and cloned the repository
- Provided requirements to Cascade for initial analysis
- AI generated high-level architecture and project structure

### AI Prompt
```
Analyse the requirements and provide a high level architecture and design for the project.
```

### Notes
- Requirements were straightforward for AI to process
- AI successfully generated the initial project structure

## 2. Code Implementation

### Initial Setup
- Bootstrapped using `go mod init`
- Generated directory structure and initial files

### Key AI Prompts
```
Bootstrap the codebase with the required directory structure.
Create a makefile with minimal scripts to build and run the services locally.
```

### Service Development
Project consists of 4 services, each developed independently:

1. **Message Queue Service**
   ```
   Based on the specs, generate the code for each message queue service.
   The service should use gRPC for communication.
   Generate a proto with 2 services - Publisher and Subscriber.
   Both need to use bidirectional streaming RPC for publishing and subscribing.
   Include acknowledgement mechanism.
   ```

2. **Streamer Service**
   ```
   Create a streamer service that streams telemetry data from CSV to the message queue.
   Default input: test-data/metrics.csv
   ```

3. **Collector Service**
   ```
   Create a collector service that subscribes to telemetry data from the message queue 
   and stores it in the database.
   ```

4. **API Server**
   ```
   Create an API server that provides RESTful API for querying telemetry data.
   ```

### Implementation Notes
- AI sometimes chose different approaches than expected (e.g., config files vs ENV variables)
- Some package versions were outdated, causing compatibility issues
- Inconsistent coding patterns across services (e.g., different logging implementations)

## 3. Unit Testing

### Implementation
- Used standard Go testing framework
- Utilized mockery for mocking dependencies

### Notes
- Initial test generation required multiple iterations
- Cascade was effective at analyzing test failures and suggesting fixes

## 4. Build Environment

### Setup
- Created Makefile for service builds
- Dockerized all services

### AI Prompts
```
Create Dockerfile for each service within the cmd directory.
Create a Makefile with minimal scripts to build and run the services locally.
```

### Kubernetes Migration
- Initially started with Docker Compose, then migrated to Kubernetes

### AI Prompts
```
Replace docker-compose and generate helm and kubernetes configs to run the service in kubernetes.
Add postgresql to the kubernetes configs and update the service configs for collector and api-server.
Expose api-server to the host to access the APIs. Use port forwarding and add it in the makefile.
```

## Key Learnings

### Version Control
- Git tracking becomes challenging with AI-generated code
- Commits tend to be large due to bulk code generation

### AI Limitations
- Not always 100% accurate
- May generate incompatible or incorrect code
- Requires careful review and validation

