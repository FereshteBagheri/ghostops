# GhostOps: Streaming Log Analysis CLI

**Language:** Go  
**Project type:** Command-line application  
**Deadline:** 7 days

## Overview

Build **GhostOps**, a command-line tool that analyzes application logs, extracts useful statistics, detects predefined incident patterns, and generates a report.

The application must handle large input files efficiently without loading the entire file into memory.

This project is designed to help you practice Go fundamentals, file handling, error management, program design, and interfaces.

## 1. Command-Line Interface

Your application must accept a log-file path as input.

It must support:
- Printing a human-readable report to the terminal.
- Exporting a report as JSON to a file using an optional `-o` flag.
- Handling invalid arguments and file errors appropriately.

The application should run locally from a terminal and must not require additional services.

## 2. Log Processing

Each log entry contains a timestamp, a severity level, a service name, and a message.

Supported severity levels are:
- `INFO`
- `WARN`
- `ERROR`
- `FATAL`

Your application must validate entries, handle malformed lines without unnecessarily interrupting the analysis, and preserve the information needed for accurate reporting.

Input files may be large. Design the processing workflow with memory efficiency in mind.

## 3. Analysis

GhostOps must calculate useful statistics, including:
- The total number of valid entries and malformed lines.
- Entry counts by severity level.
- Entry counts by service.

It must also detect incidents using predefined rules based on log levels and message content.

Matching entries must be grouped meaningfully rather than reported as unrelated individual events. Each incident group should provide enough information to understand its type, severity, frequency, affected service, and time range.

Your implementation must handle overlapping detection rules consistently.

## 4. Reporting

The report must include the required statistics and incident details.

Support two output formats:
- Human-readable terminal output.
- JSON file output.

Reports should be deterministic, valid, and useful even when the input contains no valid entries or no detected incidents.

## 5. Design and Code Organization

Organize the code into clear, maintainable components.

Keep command-line handling, input processing, analysis, and report generation logically separated. Choose a package structure that makes the responsibilities of each component understandable.

Use interfaces when they provide a meaningful abstraction or clarify component boundaries. Avoid unnecessary complexity, and be prepared to explain your design decisions.

The exact package structure and internal APIs are up to you.

## 6. Error Handling

Distinguish between invalid log entries that can be skipped and execution errors that prevent a successful report.

Handle file operations, resource cleanup, and output errors carefully. The application must not silently report success when the analysis has failed.

## 7. Deliverables

Submit a Git repository containing:
- Go source code and `go.mod`.
- At least one sample log file.
- A `README.md` with build and usage instructions.
- Example output.
- A short explanation of your architecture and important design decisions.

Use only the Go standard library. No database, web server, external API, or third-party dependency is required.

## 8. Optional Extensions

After the required functionality is complete, you may add useful filters or analytical features of your choice.

Optional work must not come at the expense of correctness or readability.

## Evaluation

The project will be evaluated on correctness, memory-efficient processing, incident detection, error handling, code organization, interface design, CLI behavior, and documentation.

**The objective is not simply to make the program work. Build a reliable tool and demonstrate that you understand the engineering decisions behind it.**
