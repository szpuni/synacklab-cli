# Releasing

Releases are built by GoReleaser (`.goreleaser.yaml`) in `.github/workflows/release.yml`
whenever a `v*` tag is pushed. Each release publishes binaries, `.deb`/`.rpm` packages,
checksums, and updates the Homebrew formula in `szpuni/homebrew-tap`.

## One-time setup

1. Create an empty public repository `szpuni/homebrew-tap` (the `homebrew-` prefix is
   required for `brew install szpuni/tap/synacklab` to work).
2. Create a fine-grained personal access token with **Contents: Read and write** on
   `szpuni/homebrew-tap` only.
3. Add it as the Actions secret `HOMEBREW_TAP_GITHUB_TOKEN` in `szpuni/synacklab-cli`
   (Settings → Secrets and variables → Actions).

## Cutting a release

```bash
git checkout main && git pull
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

The workflow runs tests, publishes the GitHub release, and commits
`Formula/synacklab.rb` to the tap. Pre-release tags (e.g. `v1.2.3-rc.1`) are released
on GitHub but do not update the formula.

Verify:

```bash
brew update && brew upgrade synacklab   # or: brew install szpuni/tap/synacklab
synacklab --help
```

## Testing the config locally

CI pins GoReleaser v1.24.0; use the same version.

```bash
HOMEBREW_TAP_GITHUB_TOKEN=dummy goreleaser check
HOMEBREW_TAP_GITHUB_TOKEN=dummy goreleaser release --snapshot --clean   # output in dist/
```
