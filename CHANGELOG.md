# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-11-25

### Added

- Initial stable release of clix
- Group/command tree model with clear semantics (groups show help, commands execute handlers)
- Built-in prompting for missing required arguments
- Configuration precedence: command flags > app flags > env > config > defaults
- Extension system for optional features (help, autocomplete, version, config, prompt, survey, validation)
- Three API styles: declarative (struct-based), functional (options), and fluent (builder-style)
- Support for text, select, multi-select, and confirm prompts
- Typed configuration access with optional schema validation
- Comprehensive godoc examples and documentation

### Core Types

- `App` – Application root with root command, flags, and extensions
- `Command` – Represents a node in the CLI tree (group or command)
- `Context` – Wraps `context.Context` with CLI metadata (App, Command, Args)
- `FlagSet` – Manages flags with precedence support
- `Argument` – Defines positional arguments with validation
- `Extension` – Interface for cross-cutting behavior

### Extensions

- `ext/help` – Command-based help system
- `ext/autocomplete` – Shell completion generation
- `ext/version` – Version command and flag
- `ext/config` – Configuration management (list/get/set/unset/reset)
- `ext/prompt` – Advanced terminal prompts (select, multi-select)
- `ext/survey` – Chained prompts with branching logic
- `ext/validation` – Common validation functions

[1.0.0]: https://github.com/SCKelemen/clix/releases/tag/v1.0.0

