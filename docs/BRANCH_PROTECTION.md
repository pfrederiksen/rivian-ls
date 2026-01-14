# Branch Protection Setup

This document explains how to set up branch protection for the `main` branch to ensure all changes go through proper review and CI checks.

## Why Branch Protection?

Branch protection ensures that:
- All changes are reviewed via Pull Requests
- CI/CD checks (lint, test, coverage) must pass before merging
- No direct commits to `main` (all work happens on feature branches)
- Code quality and test coverage standards are enforced automatically

## Setting Up Branch Protection in GitHub

Once you've pushed this repository to GitHub, follow these steps:

### 1. Navigate to Repository Settings

1. Go to your repository on GitHub
2. Click **Settings** (top right)
3. Click **Branches** in the left sidebar

### 2. Add Branch Protection Rule

1. Click **Add branch protection rule**
2. In "Branch name pattern", enter: `main`

### 3. Configure Protection Settings

Enable these settings:

#### Required Checks
- ✅ **Require a pull request before merging**
  - ✅ Require approvals: `1` (or more for team projects)
  - ✅ Dismiss stale pull request approvals when new commits are pushed
  - ✅ Require review from Code Owners (optional, if using CODEOWNERS file)

- ✅ **Require status checks to pass before merging**
  - ✅ Require branches to be up to date before merging
  - Select required checks:
    - ✅ `lint` (from CI workflow)
    - ✅ `test` (from CI workflow)
    - ✅ `build` (from CI workflow)

#### Additional Settings
- ✅ **Require conversation resolution before merging**
- ✅ **Do not allow bypassing the above settings** (recommended for teams)
- ⬜ **Allow force pushes** (keep DISABLED)
- ⬜ **Allow deletions** (keep DISABLED)

### 4. Save Changes

Click **Create** or **Save changes** at the bottom.

## Workflow with Branch Protection

With branch protection enabled, your workflow becomes:

```bash
# 1. Always create a feature branch
git checkout -b feat/my-feature

# 2. Make changes and commit
git add .
git commit -m "Add my feature"

# 3. Push to remote
git push origin feat/my-feature

# 4. Open a Pull Request on GitHub
# - Go to your repo on GitHub
# - Click "Compare & pull request"
# - Fill in PR description
# - Submit for review

# 5. CI runs automatically
# - Lint check
# - Test suite
# - Coverage gate (80% minimum)
# - Build verification

# 6. Once approved and CI passes, merge via GitHub UI
# - Click "Merge pull request"
# - Delete the feature branch

# 7. Update your local main
git checkout main
git pull origin main
```

## Testing Branch Protection Locally

You can simulate the workflow locally before setting up remote protection:

```bash
# Try to commit directly to main (should be avoided)
git checkout main
# Don't do this! Always use feature branches

# Correct workflow
git checkout -b feat/test-protection
echo "test" > test.txt
git add test.txt
git commit -m "Test change"
git push origin feat/test-protection
# Then create PR on GitHub
```

## Coverage Gate

The CI workflow includes a coverage gate that **blocks merging** if test coverage drops below 80%:

```yaml
# From .github/workflows/ci.yml
- name: Check coverage threshold
  run: |
    coverage=$(go tool cover -func=coverage.txt | grep total | awk '{print substr($3, 1, length($3)-1)}')
    threshold=80.0
    if (( $(echo "$coverage < $threshold" | bc -l) )); then
      echo "❌ Coverage ${coverage}% is below threshold ${threshold}%"
      exit 1
    fi
```

This ensures code quality remains high as the project grows.

## Enforcement

With branch protection:
- ❌ Cannot push directly to `main`
- ❌ Cannot merge PR if CI fails
- ❌ Cannot merge PR if coverage < 80%
- ✅ Must create feature branch
- ✅ Must pass all checks
- ✅ Must get PR approval
- ✅ Can then merge safely

## For Solo Development

Even when working solo, branch protection is valuable because:
- CI catches issues before they reach main
- Forces you to review your own changes (via PR view)
- Maintains a clean commit history
- Prevents accidental pushes to main
- Ensures coverage standards are met

You can set "Require approvals" to `0` for solo work, but keep all other protections enabled.

## Bypassing Protection (Emergency Only)

As a repository admin, you can temporarily bypass protection if absolutely necessary:

1. Go to Settings → Branches
2. Edit the branch protection rule
3. Check "Allow specified actors to bypass required pull requests"
4. Add yourself (use sparingly!)

**Best practice**: Never bypass protection. If you need to make an urgent fix, create a `hotfix/...` branch and merge it quickly via expedited PR.

## Additional Resources

- [GitHub Branch Protection Documentation](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
- [Status Check Documentation](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/about-status-checks)
