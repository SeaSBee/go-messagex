# Release Process

This document outlines the release process for go-messagex, ensuring consistent, high-quality releases.

## Table of Contents

1. [Release Strategy](#release-strategy)
2. [Versioning](#versioning)
3. [Release Types](#release-types)
4. [Pre-release Checklist](#pre-release-checklist)
5. [Release Process](#release-process)
6. [Post-release Activities](#post-release-activities)
7. [Emergency Releases](#emergency-releases)
8. [Release Automation](#release-automation)

## Release Strategy

### Release Cadence

- **Major Releases**: Every 6 months (or as needed for breaking changes)
- **Minor Releases**: Every 2-4 weeks (new features)
- **Patch Releases**: As needed (bug fixes and security updates)
- **Pre-releases**: Alpha/beta releases for testing

### Release Branches

- `main`: Development branch
- `release/v*`: Release branches for version preparation
- `hotfix/v*`: Emergency fix branches

## Versioning

### Semantic Versioning

go-messagex follows [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR**: Incompatible API changes
- **MINOR**: Backwards-compatible functionality additions
- **PATCH**: Backwards-compatible bug fixes

### Version Format

```
v<MAJOR>.<MINOR>.<PATCH>[-<PRERELEASE>][+<BUILD>]
```

Examples:
- `v1.0.0` - First stable release
- `v1.2.3` - Patch release
- `v1.2.3-beta.1` - Beta pre-release
- `v1.2.3+20231219` - Build with metadata

### Version Management

```bash
# Check current version
git describe --tags --abbrev=0

# Create new version tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push tags
git push origin --tags
```

## Release Types

### Major Release (v1.0.0 → v2.0.0)

**Criteria:**
- Breaking API changes
- Major architectural changes
- Incompatible configuration changes

**Process:**
1. Create release branch: `release/v2.0.0`
2. Implement breaking changes
3. Update documentation
4. Create migration guide
5. Extensive testing
6. Release candidate testing
7. Final release

### Minor Release (v1.2.0 → v1.3.0)

**Criteria:**
- New features
- Performance improvements
- New configuration options

**Process:**
1. Create release branch: `release/v1.3.0`
2. Implement new features
3. Update documentation
4. Performance testing
5. Release candidate testing
6. Final release

### Patch Release (v1.2.3 → v1.2.4)

**Criteria:**
- Bug fixes
- Security updates
- Documentation updates

**Process:**
1. Create hotfix branch: `hotfix/v1.2.4`
2. Implement fixes
3. Update CHANGELOG.md
4. Testing
5. Release

### Pre-release (v1.2.3-alpha.1)

**Criteria:**
- Feature preview
- Testing releases
- Early feedback

**Process:**
1. Create pre-release branch
2. Implement features
3. Limited testing
4. Release with pre-release tag

## Pre-release Checklist

### Code Quality

- [ ] All tests pass with race detector
- [ ] Code coverage ≥ 90%
- [ ] Linting passes without warnings
- [ ] Security scan completed
- [ ] Performance benchmarks run
- [ ] Documentation updated

### Documentation

- [ ] README.md updated
- [ ] API documentation current
- [ ] CHANGELOG.md updated
- [ ] Migration guide (if needed)
- [ ] Release notes prepared
- [ ] Examples updated

### Testing

- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Performance tests pass
- [ ] Security tests pass
- [ ] Compatibility tests run
- [ ] Manual testing completed

### Release Preparation

- [ ] Version number updated
- [ ] Git tag created
- [ ] Release branch ready
- [ ] CI/CD pipeline green
- [ ] Release notes drafted
- [ ] Stakeholders notified

## Release Process

### Step 1: Prepare Release Branch

```bash
# Create release branch
git checkout -b release/v1.0.0

# Update version in go.mod (if needed)
# Update version in documentation
# Update CHANGELOG.md

# Commit changes
git add .
git commit -m "Prepare release v1.0.0"

# Push branch
git push origin release/v1.0.0
```

### Step 2: Create Release Candidate

```bash
# Create release candidate tag
git tag -a v1.0.0-rc.1 -m "Release candidate v1.0.0-rc.1"
git push origin v1.0.0-rc.1

# Run release candidate tests
make test-rc
make performance-test
make security-scan
```

### Step 3: Final Release

```bash
# Create final release tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# Merge to main
git checkout main
git merge release/v1.0.0
git push origin main

# Delete release branch
git branch -d release/v1.0.0
git push origin --delete release/v1.0.0
```

### Step 4: Create GitHub Release

1. Go to GitHub Releases page
2. Click "Create a new release"
3. Select the release tag
4. Add release title and description
5. Upload release assets
6. Publish release

### Step 5: Update Documentation

```bash
# Update documentation links
# Update version references
# Update examples
# Update installation instructions
```

## Post-release Activities

### Monitoring

- [ ] Monitor release metrics
- [ ] Watch for issues
- [ ] Monitor performance
- [ ] Track adoption
- [ ] Collect feedback

### Communication

- [ ] Announce release
- [ ] Update community
- [ ] Respond to questions
- [ ] Address issues
- [ ] Plan next release

### Maintenance

- [ ] Backport fixes (if needed)
- [ ] Update dependencies
- [ ] Security updates
- [ ] Performance improvements
- [ ] Documentation updates

## Emergency Releases

### Hotfix Process

```bash
# Create hotfix branch
git checkout -b hotfix/v1.2.4

# Implement fix
# Update CHANGELOG.md
# Test thoroughly

# Create hotfix tag
git tag -a v1.2.4 -m "Hotfix v1.2.4"
git push origin v1.2.4

# Merge to main and release branch
git checkout main
git merge hotfix/v1.2.4
git push origin main

# Clean up
git branch -d hotfix/v1.2.4
```

### Security Releases

**Process:**
1. **Immediate Response**: Assess vulnerability
2. **Fix Development**: Develop security fix
3. **Testing**: Thorough security testing
4. **Release**: Coordinated release
5. **Disclosure**: Public disclosure
6. **Monitoring**: Monitor for issues

**Timeline:**
- Critical: 24-48 hours
- High: 1-2 weeks
- Medium: 2-4 weeks
- Low: Next regular release

## Release Automation

### CI/CD Pipeline

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v3
        with:
          go-version: '1.24.5'
      
      - name: Run tests
        run: make test
      
      - name: Run security scan
        run: make security-scan
      
      - name: Build binaries
        run: make release-build
      
      - name: Create Release
        uses: actions/create-release@v1
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          tag_name: ${{ github.ref }}
          release_name: Release ${{ github.ref }}
          body: ${{ github.event.head_commit.message }}
          draft: false
          prerelease: false
```

### Automated Testing

```bash
# Release testing script
#!/bin/bash

echo "Running release tests..."

# Run all tests
make test
if [ $? -ne 0 ]; then
    echo "Tests failed"
    exit 1
fi

# Run performance tests
make performance-test
if [ $? -ne 0 ]; then
    echo "Performance tests failed"
    exit 1
fi

# Run security scan
make security-scan
if [ $? -ne 0 ]; then
    echo "Security scan failed"
    exit 1
fi

# Build release
make release-build
if [ $? -ne 0 ]; then
    echo "Build failed"
    exit 1
fi

echo "Release tests passed"
```

### Release Scripts

```bash
# scripts/release.sh
#!/bin/bash

VERSION=$1
if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    exit 1
fi

echo "Preparing release $VERSION..."

# Update version
sed -i "s/version: .*/version: $VERSION/" go.mod

# Update CHANGELOG
echo "## [$VERSION] - $(date +%Y-%m-%d)" >> CHANGELOG.md

# Create tag
git add .
git commit -m "Release $VERSION"
git tag -a "v$VERSION" -m "Release $VERSION"

# Push
git push origin main
git push origin "v$VERSION"

echo "Release $VERSION prepared"
```

## Release Checklist Template

### Pre-release

- [ ] **Code Quality**
  - [ ] All tests pass
  - [ ] Code coverage ≥ 90%
  - [ ] Linting clean
  - [ ] Security scan passed
  - [ ] Performance benchmarks OK

- [ ] **Documentation**
  - [ ] README updated
  - [ ] API docs current
  - [ ] CHANGELOG updated
  - [ ] Release notes ready
  - [ ] Examples working

- [ ] **Testing**
  - [ ] Unit tests pass
  - [ ] Integration tests pass
  - [ ] Performance tests pass
  - [ ] Manual testing done
  - [ ] Compatibility verified

### Release

- [ ] **Preparation**
  - [ ] Version updated
  - [ ] Branch created
  - [ ] Changes committed
  - [ ] CI/CD green

- [ ] **Tagging**
  - [ ] Git tag created
  - [ ] Tag pushed
  - [ ] Release created
  - [ ] Assets uploaded

- [ ] **Communication**
  - [ ] Release announced
  - [ ] Community notified
  - [ ] Documentation updated
  - [ ] Support ready

### Post-release

- [ ] **Monitoring**
  - [ ] Metrics tracked
  - [ ] Issues monitored
  - [ ] Performance watched
  - [ ] Feedback collected

- [ ] **Maintenance**
  - [ ] Hotfixes ready
  - [ ] Dependencies updated
  - [ ] Security patches
  - [ ] Next release planned

## Release Notes Template

```markdown
# Release v1.0.0

## 🎉 What's New

### Features
- New feature 1
- New feature 2
- New feature 3

### Improvements
- Performance improvement 1
- Performance improvement 2

### Bug Fixes
- Fixed bug 1
- Fixed bug 2

### Security
- Security update 1
- Security update 2

## 🔧 Breaking Changes

- Breaking change 1
- Breaking change 2

## 📚 Documentation

- Updated API documentation
- Added performance guide
- Enhanced troubleshooting

## 🚀 Performance

- 50k+ messages/minute throughput
- < 20ms p95 latency
- 90%+ test coverage

## 📦 Installation

```bash
go get github.com/SeaSBee/go-messagex@v1.0.0
```

## 🔗 Links

- [Documentation](https://github.com/SeaSBee/go-messagex/docs)
- [API Reference](https://github.com/SeaSBee/go-messagex/docs/API.md)
- [Migration Guide](https://github.com/SeaSBee/go-messagex/docs/MIGRATION.md)

## 🙏 Thanks

Thanks to all contributors and users for making this release possible!
```

---

**Consistent, well-documented releases ensure reliability and trust in the go-messagex project.**
