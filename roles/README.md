# Roles

Role YAML files define reusable testing personas. Update the name, description, prompt, tools, and skills fields to customize behavior.

For internal red-team workflows in this repository, prefer linking roles to narrow, task-specific skills.

Recommended role for controlled validation:

- `Approved Web Validation`: moves from analysis into explicitly approved, low-impact confirmation of suspected web and API weaknesses.

Recommended companion skill:

- `cleanup-and-rollback`: use after validation to remove test-only artifacts and leave a clean, auditable handoff.
