# Security Policy

## Reporting a Vulnerability

Do not report undisclosed vulnerabilities through public GitHub issues.

Instead, report them privately to the maintainer listed in [`CONTRIBUTORS.md`](./CONTRIBUTORS.md). Include:

- A description of the issue
- Affected versions or commit references
- Reproduction steps or proof of concept
- Impact assessment if known

You should receive an acknowledgment within a reasonable time after receipt.

## Security Expectations

Crossagent executes local developer tools and can open interactive agent sessions against user-selected repositories. Operators should review prompts, outputs, and any generated code before trusting or deploying results.

Security-sensitive changes should avoid:

- Expanding filesystem access without a clear need
- Implicit remote execution behavior
- Silent changes to workflow state or repositories
