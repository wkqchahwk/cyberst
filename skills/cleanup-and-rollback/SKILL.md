---
name: cleanup-and-rollback
description: Post-validation cleanup and rollback guidance for authorized internal testing without hiding or tampering with audit evidence.
version: 1.0.0
---

# Cleanup and Rollback

## Purpose

Use this skill after an authorized validation step to remove temporary test artifacts, roll back approved changes, and leave the environment in a known-good state while preserving auditability.

## Never Do

- Do not tamper with security logs, platform audit trails, or centralized telemetry.
- Do not alter timestamps, rotate logs to hide activity, or disable monitoring.
- Do not remove production data unless the written test plan explicitly required creating that data.
- Do not delete anything that the blue team, SOC, or application owner needs for validation or review.

## Cleanup Targets

- Temporary accounts created specifically for the test
- Temporary files, uploads, or objects created solely for validation
- Short-lived configuration changes approved for the test window
- Session tokens or secrets issued only for the exercise
- Scheduled tasks, callbacks, or test-only automation hooks created during the exercise

## Recommended Workflow

1. List every artifact created by the test.
2. Separate safe cleanup items from artifacts that must be preserved for review.
3. Roll back the lowest-risk items first.
4. Verify that the target service still behaves normally after each rollback step.
5. Produce a final handoff note with what was cleaned, what was intentionally preserved, and what still requires owner action.

## Output Checklist

- Artifact or change that was created
- Rollback action taken
- Verification result after rollback
- Item intentionally preserved for audit or reproduction
- Any residual risk or follow-up for the service owner
