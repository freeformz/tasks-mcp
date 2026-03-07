---
name: release
description: Create a release by pulling main, tagging a semver version, pushing the tag, and monitoring the GitHub Actions release workflow until it completes. Reports failures and lists release artifacts.
user_invocable: true
---

# Release Skill

Create and publish a new release for this project.

## Usage

`/release <version>` — where `<version>` is a semver tag like `v0.5.0`

## Instructions

1. **Set git working directory** to the current project using `git_set_working_dir`.

2. **Pull main**: Fast-forward pull from origin/main. Abort if there are conflicts.

3. **Check existing tags**: List tags and verify the requested version doesn't already exist. If it does, inform the user and stop.

4. **Show what's new**: Run `git log --oneline` from the latest existing tag to HEAD. Summarize the changes for the user.

5. **Create the tag**: Create an annotated tag with a brief message summarizing the release. Use `forceUnsignedOnFailure: true` in case GPG signing isn't configured.

6. **Push the tag**: Push tags to origin.

7. **Monitor the workflow**: Use `gh run list` to find the Release workflow triggered by the tag push, then `gh run watch <run_id> --exit-status` to monitor it until completion. Use a background task with sufficient timeout (5 minutes) since the docker build can take a while.

8. **Report results**:
   - If the workflow **fails**: Show which job failed and fetch logs with `gh run view <run_id> --log-failed`.
   - If the workflow **succeeds**: Fetch the release from GitHub and report:
     - Release URL
     - All release artifacts with their sizes
     - Docker image status

9. **On any error**: Report the error clearly and suggest remediation steps (e.g., delete the tag if the workflow failed and a retry is needed).
