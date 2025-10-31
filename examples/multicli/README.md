# Multi-CLI Example

This example demonstrates how to share command implementations across multiple CLI applications with different hierarchies and naming.

## Concept

Different teams can have their own focused CLIs while sharing common command implementations:

- **`dev`** - Aggregates all engineering commands for the entire developer team
- **`db`** - Database team's focused CLI with direct access to database commands
- **`sec`** - Security team's focused CLI with shorter aliases
- **`bq`** - BigQuery CLI with versioning support (similar to `gcloud`)

## Shared Commands

All commands are implemented in `internal/` and can be reused across different CLIs:

- **`database`** - Database operations (create, list)
- **`vulnerabilities`** - Security vulnerability management (list with severity filter)
- **`bigquery`** - BigQuery operations with versioning support (v1alpha, v1beta, v1)

## Usage Examples

### Dev CLI (Aggregated)

```bash
# All commands available with full paths
dev database create mydb
dev database list
dev vulnerabilities list
dev bigquery dataset list
dev bigquery v1beta dataset list
```

### DB CLI (Focused)

```bash
# Direct access, no "database" prefix
db create mydb
db list
```

### Sec CLI (Focused with Aliases)

```bash
# Uses shorter "vulns" alias
sec vulns list
sec vulns list --severity critical
```

### BQ CLI (Versioned)

```bash
# Similar to gcloud bigquery
bq dataset list                    # Latest/default version
bq v1alpha dataset list           # Alpha version
bq v1beta dataset list            # Beta version  
bq v1 dataset list                # Stable v1
```

## Format Support

All commands support the global `--format` flag:

```bash
dev database list --format=json
dev database list --format=yaml
dev database list --format=text  # default

sec vulns list --format=json --severity high
```

## Building

```bash
# Build all CLIs (outputs to current directory)
go build -o dev ./cmd/dev
go build -o db ./cmd/db
go build -o sec ./cmd/sec
go build -o bq ./cmd/bq

# Or build individually
cd cmd/dev && go build
```

The binaries will be created in the examples/multicli directory:
- `./dev` - Developer tools CLI
- `./db` - Database team CLI
- `./sec` - Security team CLI
- `./bq` - BigQuery CLI

## Key Patterns

1. **Shared Internal Packages**: Commands live in `internal/` and are reused
2. **Flexible Mounting**: Commands can be mounted at different paths in different CLIs
3. **Aliases**: Use command aliases for shorter names in focused CLIs
4. **Versioning**: Support multiple API versions (like `gcloud bigquery v1alpha`)
5. **Format Support**: All commands use `FormatOutput()` for consistent json/yaml/text output

