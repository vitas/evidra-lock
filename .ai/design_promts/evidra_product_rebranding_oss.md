You are acting as a senior open-source maintainer.

Objective:
Standardize branding and legal positioning across this repository.

Brand Positioning:
"Evidra — an open-source tool by SameBits"

Critical Constraints:
- DO NOT modify any functional source code.
- DO NOT change imports, module names, package names, or versions.
- DO NOT introduce breaking changes.
- Only modify documentation, markdown files, project metadata, and non-executable text.
- Keep changes minimal and clean.

Tasks:

1. Update README.md

Ensure it follows this structure:

# <Project Name>

<One-sentence technical description>

Evidra is an open-source tool developed and maintained by SameBits.

Add footer section:

---
Part of the Evidra open-source toolset by SameBits.
This name is used strictly for open-source identification purposes.

2. Add NOTICE file if missing:

Content:

Evidra is an open-source software project developed by SameBits.
The name "Evidra" is used as a project identifier for open-source tooling.
No trademark claim is made within this repository.

3. Documentation files (docs/*.md)

Add at the top (if missing):

Part of the Evidra OSS toolset by SameBits.

4. CLI help text (ONLY if safe and does not affect behavior)

Ensure help output contains:

Evidra <Tool Name> — open-source utility by SameBits.

5. Repository description (if editable)
Short format:

"<Specific function> — part of the Evidra OSS toolset by SameBits"

6. Avoid commercial positioning.

Do NOT describe this repository as:
- a platform
- a SaaS
- a service
- a cloud product

Position strictly as open-source tooling.

Return:
- List of modified files
- Short summary of changes
- Confirmation that no runtime behavior was altered