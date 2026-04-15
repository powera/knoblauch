# Atacama - Current Status

## Repository Overview

**Atacama** is a multi-purpose web application that serves as a semantic web publishing platform and backend API infrastructure. The repository consists of three main integrated services that share a common Python/Flask foundation.

## Core Components

### 1. Atacama CMS (Blog Server)

The primary component is a sophisticated blog and content management system featuring a custom semantic markup language called **Atacama Markup Language (AML)**. This markup system uses color-coded tags that convey semantic meaning and tone:

- Sarcastic/overconfident statements (`<xantham>`)
- Forceful statements (`<red>`)
- Counterpoints (`<orange>`)
- Technical content (`<green>`)
- AI-generated content (`<teal>`)
- Serious announcements (`<violet>`)
- Past stories and narrative content (`<gray>`, `<hazel>`)

The CMS supports wiki-style linking with `[[Page Title]]` syntax, automatic Chinese pinyin annotations, chess notation rendering, YouTube embedding, and rich multimedia content. Content is organized into **channels** with three access levels (public, private, restricted) and supports multi-domain configurations with per-domain theming.

### 2. React Widget System

Atacama includes a complete React widget development and compilation system that allows embedding interactive JavaScript applications directly into blog content. The system:

- Compiles React 19 components using Webpack to UMD bundles
- Supports popular libraries (Recharts, D3, Lodash, Axios)
- Provides custom React hooks (`useFullscreen`, `useGlobalSettings`)
- Includes sample widgets like interactive games, flashcards, and educational tools
- Handles automatic dependency detection and bundling

This enables authors to create rich, interactive content experiences within the semantic markup environment.

### 3. Trakaido Stats API

A dedicated backend API service for tracking Lithuanian language learning statistics and flashcard progress. Key features include:

- Nonce-based authentication system
- Cross-domain cookie sharing (*.trakaido.com)
- User statistics tracking with historical snapshots
- Audio file serving for pronunciation
- User configuration management
- Multiple stat types (vocabulary, flashcards, progress)

This API serves as the backend infrastructure for the Trakaido language learning application.

### 4. Spaceship Daemon

A lightweight proof-of-life monitoring server that serves dynamically updated XPlanet Earth images, providing a visual indicator of server status and uptime.

## Technical Architecture

- **Backend**: Python 3.9+ with Flask framework
- **Database**: SQLAlchemy ORM supporting PostgreSQL and SQLite
- **Web Server**: Waitress (production), Flask dev server (development)
- **Frontend**: Custom JavaScript with React widget compiler (Node.js/Webpack)
- **Configuration**: TOML-based configuration files for channels, domains, and administration
- **Authentication**: Google OAuth integration

## Development Workflow

The project maintains strict code quality standards with a comprehensive PRESUBMIT.py validation system that enforces:

- Import organization (stdlib, third-party, local)
- No relative imports outside test files
- Unused import detection
- Third-party dependency verification against requirements.txt

Testing is organized into categories (common, web, aml_parser, react_compiler) with a comprehensive test runner supporting coverage reporting, verbose output, and pattern-based test selection.

## Key Directories

- `src/atacama/` - Core Flask application and blueprints
- `src/blog/` - Blog/CMS module
- `src/trakaido/` - Stats API endpoints
- `src/aml_parser/` - Markup language parser
- `src/react_compiler/` - Widget compilation system
- `src/spaceship/` - Proof-of-life server
- `config/` - TOML configuration files
- `src/models/` - SQLAlchemy database models

## Current State

Atacama is a production-ready system serving as both a personal publishing platform and critical backend infrastructure for the Trakaido language learning ecosystem. The architecture emphasizes separation of concerns, testability, and extensibility while maintaining a cohesive user experience across its diverse functionality.
