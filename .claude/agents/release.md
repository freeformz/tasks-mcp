---
name: release
description: Create a release by pulling main, tagging a semver version, pushing the tag, and monitoring the GitHub Actions release workflow until it completes. Reports failures and lists release artifacts.
user_invocable: true
---

# Release Skill

Create and publish a new release for this project.

## Usage

`/release [version]` — where `[version]` is an optional semver tag like `v0.5.0`

If no version is provided, the next version is determined automatically (see step 2).

## Instructions

1. **Set git working directory** to the current project using `git_set_working_dir`.

2. **Determine the version**:
   - If a version was provided, validate it matches the pattern `vMAJOR.MINOR.PATCH` (e.g., `v0.5.0`). If the user provides a version without the `v` prefix, add it. If the string is not valid semver, reject with a clear error.
   - If no version was provided, auto-detect the next version:
     - List existing semver tags sorted by version: `git tag --list 'v*' --sort=-version:refname`.
     - Take the latest tag (e.g., `v0.5.0`) and increment the **patch** version (e.g., `v0.5.1`).
     - If no tags exist yet, default to `v0.1.0`.
     - Show the user the computed version and the commits it will include, then ask for confirmation before proceeding.

3. **Prepare and pull main**:
   - Ensure the working tree is clean: run `git status --porcelain` and abort with a clear message if there are uncommitted changes.
   - Ensure you are on the `main` branch: run `git rev-parse --abbrev-ref HEAD` and, if not on `main`, run `git checkout main`. Abort if checkout fails.
   - Run `git fetch origin` to update remote refs.
   - Run `git pull --ff-only origin main`. Abort if a fast-forward is not possible.

4. **Check existing tags**: List tags and verify the requested version doesn't already exist. If it does, inform the user and stop.

5. **Show what's new**: Run `git log --oneline` from the latest existing tag to HEAD. Summarize the changes for the user.

6. **Create the tag**: Create an annotated tag with a brief message summarizing the release. Prefer a signed tag:
   - First, try `git tag -as <version> -m "Release <version>"`.
   - If signing fails because GPG/signing is not configured, fall back to an unsigned annotated tag with `git tag -a <version> -m "Release <version>"` and inform the user that the tag was created unsigned.

7. **Push the tag**: Push only the newly created tag to origin using `git push origin <version>`.

8. **Monitor the workflow**:
   - Determine the commit SHA for the pushed tag using `git rev-parse <version>`.
   - Use `gh run list --workflow release.yml --json databaseId,headSha,headBranch,event,status,createdAt --limit 20` and select the run whose `headSha` matches the tag's commit SHA. Do **not** just take the most recent run; always match on the tagged commit.
   - Use `gh run watch <run_id> --exit-status` on that matching run to monitor it until completion. Run this in a background task with a generous timeout (at least 20 minutes) to account for the multi-job GoReleaser and multi-arch Docker build.

9. **Report results**:
   - If the workflow **fails**: Show which job failed and fetch logs with `gh run view <run_id> --log-failed`.
   - If the workflow **succeeds**:
     - Fetch the release using `gh release view <version> --json url,assets --jq '{url: .url, assets: [.assets[] | {name: .name, size: .size}]}'`.
     - Report the **release URL** from the `url` field.
     - Report **all release artifacts with their sizes** from the `assets` array.
     - For **Docker image status**, query the container package using `gh api /users/{owner}/packages/container/{package_name}/versions --jq '[.[] | {id: .id, tags: .metadata.container.tags, updated_at: .updated_at}]'` (for `ghcr.io/{owner}/{repo}`, use `{repo}` as `{package_name}`) and confirm the new version tag was published.

10. **On any error**: Report the error clearly and suggest remediation steps (e.g., delete the tag if the workflow failed and a retry is needed).
