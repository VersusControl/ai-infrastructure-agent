# Project Governance

This document describes the governance model, decision-making process, and community structure for the AI Infrastructure Agent project.

## Governance Model

The project follows a **benevolent dictator** model with a core maintainer team. Decisions are made by consensus when possible, with maintainers having final say on technical and process matters.

## Maintainer Roles and Responsibilities

### Core Maintainers

- **Technical leadership**: Approve architectural changes, review critical PRs, ensure code quality.
- **Release management**: Cut releases, maintain changelog, versioning.
- **Community**: Triage issues, guide contributors, moderate discussions.

### Maintainer Expectations

- Respond to issues and PRs in a timely manner
- Uphold the [Code of Conduct](../CODE_OF_CONDUCT.md)
- Mentor new contributors
- Participate in RFC discussions when relevant

### Maintainer Onboarding

New maintainers are invited by existing maintainers based on:

- Sustained contributions to the project
- Demonstrated technical judgment
- Alignment with project goals and values

## Decision-Making Process

### Routine Changes

- **Bug fixes, docs, minor features**: PR review and approval by at least one maintainer.
- **No breaking changes** without explicit discussion.

### Significant Changes

- **New AWS services, major refactors, breaking changes**: Require an RFC or design discussion in an issue before implementation.
- **Consensus**: Aim for maintainer consensus; in case of disagreement, maintainers may escalate to the project lead.

### Escalation Path

1. Discuss in issue or PR comments
2. Open a dedicated RFC issue if needed
3. Maintainers make final decision

## RFC (Request for Comments) Process

For substantial changes:

1. **Create an RFC issue** with the `rfc` label (if available) or `enhancement`.
2. **Title**: `[RFC] Brief description`
3. **Content**: Problem statement, proposed solution, alternatives, impact.
4. **Discussion**: Allow at least 1 week for community feedback.
5. **Decision**: Maintainers summarize and decide.

## Release Planning and Versioning

### Versioning

We follow [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes and minor improvements

### Release Cadence

- **Patch releases**: As needed for bug fixes and security updates.
- **Minor releases**: When significant features are ready.
- **Major releases**: Planned with migration guides where possible.

### Release Process

1. Update changelog
2. Tag release (e.g., `v1.2.0`)
3. Create GitHub release with notes
4. Announce in Discussions (if applicable)

## Conflict Resolution

- **Code conflicts**: Resolved through PR review and maintainer feedback.
- **Interpersonal conflicts**: Refer to [Code of Conduct](../CODE_OF_CONDUCT.md). Maintainers may step in to mediate.
- **Disagreements on direction**: Use RFC process; maintainers have final say.

## Project Roadmap Communication

- **Roadmap**: Tracked in [GitHub Projects](https://github.com/orgs/VersusControl/projects) and referenced in the README.
- **Updates**: Major milestones communicated via release notes and Discussions.
- **Feedback**: Open issues and Discussions for community input on priorities.

## Community Leadership Structure

- **Maintainers**: Core team with merge and release authority.
- **Contributors**: Anyone who submits PRs or participates in issues.
- **Users**: Consumers of the project; feedback encouraged via issues and Discussions.

---

For contribution guidelines, see [CONTRIBUTING.md](../CONTRIBUTING.md).
