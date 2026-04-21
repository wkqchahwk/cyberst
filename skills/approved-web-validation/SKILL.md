---
name: approved-web-validation
description: Controlled web and API validation guidance for explicitly approved internal targets.
version: 1.0.0
---

# Approved Web Validation

## Purpose

Use this skill when the task is to confirm a suspected web or API weakness on an explicitly approved internal target, not to build unrestricted offensive capability.

## Preconditions

Only proceed when all of the following are clear in the task or engagement notes:

1. The target host, route, tenant, or application is explicitly in scope.
2. The testing window or operational context is defined.
3. The allowed account or authentication context is defined.
4. Stop conditions are defined or can be inferred safely.

If any of those are missing, ask for clarification or stay in analysis-only mode.

## Allowed Validation Patterns

- Replay previously observed requests to confirm a finding.
- Perform low-volume parameter mutation to validate authorization or input-handling weaknesses.
- Use targeted crawling or endpoint discovery to confirm exposure related to the reported issue.
- Capture minimally invasive proof such as response diffs, timing evidence, or sanitized screenshots.
- Prefer read-only confirmation over state-changing confirmation whenever possible.

## Disallowed Escalation

Do not use this skill to perform or automate:

- Shell execution, persistence, or post-exploitation.
- Destructive writes, record deletion, or service disruption.
- Bulk data extraction or large-scale enumeration beyond the approved proof need.
- Credential stuffing, password spraying, phishing, or social engineering.
- Lateral movement or access expansion outside the named target.
- Unbounded fuzzing or payload spraying that creates operational risk.

## Recommended Workflow

1. Restate the exact target, approval context, and goal.
2. Define the smallest proof needed to confirm the suspected weakness.
3. Run a dry pass first: baseline response, control request, and expected safe failure mode.
4. Execute the smallest active validation that can confirm or refute the issue.
5. Stop immediately if the response indicates instability, data overexposure, or scope drift.
6. Record the finding with evidence, impact, and remediation guidance.

## Evidence Checklist

- Exact target and authenticated context used
- Control request and validation request
- Observable proof that differentiates secure vs. insecure behavior
- Scope and safety constraints followed during validation
- Remediation guidance and suggested regression test

## Output Style

- Be explicit about what was confirmed versus what remains a hypothesis.
- Include enough detail for defenders to reproduce safely.
- If stronger exploitation would be required to prove impact, say so without escalating into it.
