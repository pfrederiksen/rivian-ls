# Release Process

This document describes how to create a new release of `rivian-ls`.

## Overview

Releases are automated using [GoReleaser](https://goreleaser.com/) and GitHub Actions. When you push a new version tag, the release workflow automatically:

1. Builds binaries for multiple platforms (Linux, macOS, Windows × amd64/arm64)
2. Creates GitHub release with changelog
3. Uploads binaries and checksums
4. Updates the Homebrew tap at `pfrederiksen/homebrew-tap`

## Prerequisites

- Write access to the repository
- All CI checks passing on `main` branch
- HOMEBREW_TAP_TOKEN secret configured (already set up)

## Creating a Release

### 1. Ensure main is up to date

```bash
git checkout main
git pull origin main
```

### 2. Run tests locally

```bash
make check
```

Ensure all tests pass and coverage is above the threshold.

### 3. Create and push a version tag

Use [semantic versioning](https://semver.org/):
- **Major** (v1.0.0 → v2.0.0): Breaking changes
- **Minor** (v1.0.0 → v1.1.0): New features, backwards compatible
- **Patch** (v1.0.0 → v1.0.1): Bug fixes

```bash
# Create annotated tag
git tag -a v0.1.0 -m "Release v0.1.0: Initial release"

# Push tag to trigger release
git push origin v0.1.0
```

### 4. Monitor the release workflow

```bash
# Watch the workflow in real-time
gh run watch

# Or view on GitHub
gh run list --workflow=release.yml
```

The workflow typically takes 3-5 minutes.

### 5. Verify the release

Once complete:

1. **GitHub Release**: Check https://github.com/pfrederiksen/rivian-ls/releases
   - Binaries should be attached
   - Changelog should be generated
   - Release notes should include installation instructions

2. **Homebrew Tap**: Check https://github.com/pfrederiksen/homebrew-tap
   - Formula should be updated in `Formula/rivian-ls.rb`
   - Version and SHA256 checksums should match

3. **Test Homebrew Installation**:
   ```bash
   # Update tap
   brew update

   # Install or upgrade
   brew install pfrederiksen/tap/rivian-ls
   # or
   brew upgrade pfrederiksen/tap/rivian-ls

   # Test
   rivian-ls version
   ```

## Testing a Release Locally (Dry Run)

Before creating a real release, you can test GoReleaser locally:

```bash
# Install GoReleaser (if not already installed)
brew install goreleaser

# Test the configuration (builds but doesn't publish)
goreleaser release --snapshot --clean --skip=publish

# Built binaries will be in ./dist/
ls -lh dist/
```

## Release Checklist

- [ ] All tests passing on main
- [ ] CHANGELOG updated (if using manual changelog)
- [ ] Version tag follows semantic versioning
- [ ] Tag is annotated (use `git tag -a`)
- [ ] Tag pushed to origin
- [ ] GitHub Actions workflow succeeded
- [ ] GitHub release created with binaries
- [ ] Homebrew tap updated
- [ ] Homebrew installation tested
- [ ] Release announced (optional: Twitter, blog, etc.)

## Troubleshooting

### Release workflow fails

1. Check the workflow logs:
   ```bash
   gh run view --log-failed
   ```

2. Common issues:
   - **Tests fail**: Fix tests and push to main, then re-tag
   - **HOMEBREW_TAP_TOKEN invalid**: Regenerate token and update secret
   - **GoReleaser config error**: Test locally with `--snapshot` flag

### Homebrew tap not updated

1. Check if HOMEBREW_TAP_TOKEN secret is set:
   ```bash
   gh secret list --repo pfrederiksen/rivian-ls
   ```

2. Ensure the token has write access to `pfrederiksen/homebrew-tap`

3. Check GoReleaser logs in the workflow for errors

### Need to delete a bad release

```bash
# Delete the tag locally
git tag -d v0.1.0

# Delete the tag remotely
git push origin :refs/tags/v0.1.0

# Delete the GitHub release
gh release delete v0.1.0 --yes

# Revert the Homebrew tap commit if needed
# (Go to pfrederiksen/homebrew-tap and revert the formula commit)
```

Then create the correct tag and push again.

## Release Frequency

- **Patch releases**: As needed for bug fixes (can be frequent)
- **Minor releases**: Every few weeks for new features
- **Major releases**: Rarely, only for breaking changes

## Post-Release Tasks

After each release:

1. Update documentation if needed
2. Announce in relevant channels (optional)
3. Monitor issues for any release-related bugs
4. Consider creating a milestone for the next version

## Automated Release Notes

GoReleaser automatically generates release notes from commit messages. To ensure good release notes:

- Use conventional commits: `feat:`, `fix:`, `chore:`, etc.
- Write clear, user-facing commit messages
- Commits starting with `docs:`, `test:`, `chore:` are excluded from release notes

Example good commit messages:
```
feat: add support for multi-vehicle selection in TUI
fix: resolve websocket reconnection deadlock
feat(cli): add --format yaml option to status command
```

## Version Information in Binary

GoReleaser injects version info via ldflags. The binary will show:

```bash
$ rivian-ls version
rivian-ls version v0.1.0
  commit: abc123def
  built:  2026-01-14T12:34:56Z
```

This is automatic - no manual changes needed.
