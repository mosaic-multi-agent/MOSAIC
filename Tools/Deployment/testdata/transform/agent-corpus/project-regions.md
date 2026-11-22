---
id: 32
version: 1.0.0
name: project-regions
description: Worker carrying filled and empty project regions in several sections
role: subagent
model: {model-identifier}
tools: [file_read, file_write, terminal]
recommended_tier: MEDIUM
tier_rationale: corpus fixture for body preservation
required_skills: []
---

<Identity type="core">
# ProjectRegions Agent (Corpus Fixture)

Opening prose before the user-owned region.

<IdentityExtension type="project">
User-owned guidance line one.
User-owned guidance line two, with `code` and *emphasis*.
</IdentityExtension>

Prose between regions.
</Identity>

<Constraints type="core">
- Stay within scope.

<ConstraintsExtension type="project">
</ConstraintsExtension>
</Constraints>
