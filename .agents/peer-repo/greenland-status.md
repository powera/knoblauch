# Greenland - Current Status

## Repository Overview

**Greenland** is a multilingual linguistic database system designed to create, validate, and manage comprehensive language learning data. The project serves as the data generation pipeline and quality assurance infrastructure for the Trakaido language-learning application, producing structured vocabulary, translations, audio, and grammatical content across multiple languages.

## Core Purpose

Greenland's primary mission is to generate high-quality **WireWord export files** - structured JSON data containing vocabulary entries with translations, definitions, pronunciation, audio files, example sentences, and grammatical metadata. These exports power the Trakaido mobile and web applications, enabling learners to study Lithuanian, Chinese, French, Spanish, and other languages.

## Architecture

### Database Layer

The system centers around a SQLite database (`data/wordfreq/linguistics.sqlite`) managed via SQLAlchemy ORM. The schema (in `src/storage/`) defines models for:

- Word entries with multi-language translations
- Definitions and usage examples
- Pronunciation data (IPA phonetics)
- Audio file references
- Grammatical forms (declensions, conjugations)
- Sentence-word relationships
- Synonyms and alternative forms

### Agent-Based Processing

Greenland uses a collection of **specialized processing agents** (named after Lithuanian animals) that perform bulk operations against the database, typically involving LLM calls for linguistic validation and generation:

**Core Validation & Processing:**
- **lokys** - English lemma validation (forms and definitions)
- **dramblys** - Missing word detection and processing
- **vilkas** - Word forms generation (declensions, conjugations)
- **voras** - Multi-lingual translation management
- **papuga** - Pronunciation (IPA) validation and generation
- **sernas** - Synonym and alternative form generation

**Audio Generation:**
- **strazdas** - eSpeak-NG audio generation (open-source TTS)
- **vieversys** - OpenAI TTS audio generation (high-quality voices)

**Export & Utilities:**
- **ungurys** - WireWord export generation
- **bebras** - Sentence-word link management
- **zvirblis** - Sentence generation

All agents follow standardized command-line interfaces via `src/agents/common_args.py`, supporting consistent options like `--guid`, `--db-path`, `--model`, `--limit`, `--dry-run`, `--debug`, and `--yes`.

### Web Interface (Barsukas)

`src/barsukas` provides a Flask-based web UI for human interaction with the database. This interface allows linguists and contributors to:

- Browse and search vocabulary entries
- Edit translations and definitions
- Review and approve generated content
- Manage word relationships
- Quality control and validation

### Language Support

The system maintains comprehensive language code mappings in `src/storage/translation_helpers.py`, which serves as the single source of truth for:

- LLM field-to-language-code conversions
- Language code standardization
- Multi-lingual response parsing

Currently supported target languages include Lithuanian, Chinese (simplified), French, Spanish, German, Italian, Dutch, Portuguese, and Swedish.

## Technical Infrastructure

- **Primary Language**: Python with type hinting requirements
- **PYTHONPATH**: Set to `src/` for all script execution
- **Database**: SQLAlchemy with SQLite backend
- **LLM Integration**: `src/clients/` contains adapters for ChatGPT, Claude, and Gemini
- **Configuration**: DataSourceConfig objects (from `src/storage/backend/config.py`) pass db_path, model_name, and other settings
- **Code Quality**: Black formatting and mypy type checking enforced via pre-commit hooks

## Development Workflow

All Python scripts use absolute imports (`from agents.common_args import`) and are invoked with explicit PYTHONPATH:

```bash
PYTHONPATH=src python src/agents/dramblys.py --help
```

The project emphasizes:
- Type hints on all new code (except benchmarks)
- No variable name reuse within functions
- Absolute imports only
- Pre-commit hooks for black/mypy validation
- Tests in `src/tests/` for client code

## Data Release Pipeline

Word data is stored in `data/release/` files organized by language and GUID prefix. These files:

- Maintain sorted order by GUID
- Use GUID prefixes defined in `storage/models/guid_prefixes.py`
- Store words in lemma form with difficulty levels
- Include disambiguation when necessary
- Leave gaps for removed words (GUIDs are immutable)

The release files are processed by export agents to generate the final WireWord JSON consumed by Trakaido applications.

## Current State

Greenland is a mature data generation and quality assurance system that has produced comprehensive multilingual datasets for production language learning applications. The agent-based architecture enables continuous improvement and expansion of linguistic content while maintaining data quality through LLM-assisted validation and human review workflows.
