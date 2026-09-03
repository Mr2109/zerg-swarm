# CodeAuditSkill

## Description
Automated code quality and security audit skill. This skill analyzes source code for logic errors, security vulnerabilities, and style consistency.

## Inputs
- `path` (string): The file path to be audited.
- `ruleset` (string): The rule set to apply (e.g., "security", "style", "logic").

## Outputs
- `report` (JSON): A structured report containing issues found.
  - `issues`: List of objects {`line`, `type`, `message`, `severity`}.

## Integration
- **Registration**: Must be registered in `core/internal/agent/skill_manager.go`.
- **Execution**: Dispatched via `core/internal/agent/exec.go`.
- **Metadata**: Part of the Zerg Core Skill Ecosystem.
