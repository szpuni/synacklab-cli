# Releasing

Releases are built by GoReleaser (`.goreleaser.yaml`) in `.github/workflows/release.yml`
whenever a `v*` tag is pushed. Each release publishes binaries, `.deb`/`.rpm` packages,
checksums, and updates the Homebrew formula at `Formula/synacklab.rb` in this repository.

This repository doubles as its own Homebrew tap, so no separate tap repo or extra secret
is needed: GoReleaser commits the formula to `main` using the workflow's `GITHUB_TOKEN`.
If `main` becomes branch-protected, that commit will be rejected — allow GitHub Actions
to bypass the rule or the Homebrew step will fail.

## Cutting a release

```bash
git checkout main && git pull
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

The workflow runs tests, publishes the GitHub release, and commits
`Formula/synacklab.rb` to `main` (run `git pull` afterwards). Pre-release tags
(e.g. `v1.2.3-rc.1`) are released on GitHub but do not update the formula.

Verify:

```bash
brew trust --formula szpuni/synacklab-cli/synacklab                     # first time only
brew tap szpuni/synacklab-cli https://github.com/szpuni/synacklab-cli   # first time only
brew update && brew upgrade synacklab   # or: brew install synacklab
synacklab --help
```

## Testing the config locally

CI pins GoReleaser v1.24.0; use the same version.

```bash
GITHUB_TOKEN=dummy goreleaser check
GITHUB_TOKEN=dummy goreleaser release --snapshot --clean   # output in dist/
```
