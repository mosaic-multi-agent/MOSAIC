---
id: 31
version: 1.0.0
name: plain-prose
description: Worker whose body is plain prose, lists and a code fence, with no managed or project regions
role: subagent
model: {model-identifier}
tools: [file_read, file_search]
recommended_tier: LOW
tier_rationale: corpus fixture for body preservation
required_skills: []
---

<Identity type="core">
# PlainProse Agent (Corpus Fixture)

Plain prose with **bold**, `inline code`, and a list:

- first item
- second item with trailing spaces   
- third item

```text
a fenced block   with   odd   spacing
	and a tab-indented line
```

> A block quote that must survive untouched.

Final paragraph without trailing newline handling tricks.
</Identity>
