# Release and Distribution

`oneagent` now ships with GoReleaser configuration for tagged GitHub releases and Homebrew cask publishing.

## Release flow

1. Create the tap repo at `github.com/1broseidon/homebrew-tap`.
2. Create a dedicated fine-grained personal access token scoped only to `1broseidon/homebrew-tap` with repository `Contents` permission set to `write`.
3. Add that token to this repo as the GitHub Actions secret `HOMEBREW_TAP_GITHUB_TOKEN`.
4. Push a semver tag such as `v0.10.0`.
5. GitHub Actions runs tests, builds release archives, publishes the GitHub release, and updates `Casks/oa.rb` in the tap repo.

The release workflow lives in `.github/workflows/release.yml`, and the packaging rules live in `.goreleaser.yaml`.

The Homebrew side uses a cask rather than a Formula because current GoReleaser releases deprecate the old `brews` publisher in favor of `homebrew_casks`.

## Dedicated tap token

Use a dedicated fine-grained PAT rather than your main GitHub CLI token.

- Resource owner: `1broseidon`
- Repository access: `Only select repositories`
- Selected repository: `homebrew-tap`
- Repository permissions: `Contents: Write`

Then store it in this repo:

```sh
gh secret set HOMEBREW_TAP_GITHUB_TOKEN -R 1broseidon/oneagent
```

## Homebrew install

After the first tagged release has published the cask:

```sh
brew tap 1broseidon/tap
brew install --cask 1broseidon/tap/oa
```

## `oa` namespace status

Checked on 2026-03-16 across the main install surfaces that matter for a standalone CLI:

| Surface | Status | Notes |
| --- | --- | --- |
| Homebrew tap cask | Available | `1broseidon/tap/oa` is available once the tap repo exists. |
| Homebrew core Formula | Open | No exact `oa` formula in the current Homebrew formula index. |
| Scoop manifest | Open | No `bucket/oa.json` manifest in `ScoopInstaller/Main`. |
| AUR package | Open | The AUR RPC returns no exact `oa` package today. |
| npm package | Taken | `oa` is already published on npm for a different project. |
| `go install` binary name | Already works | `go install github.com/1broseidon/oneagent/cmd/oa@latest` installs an `oa` binary. |
| Winget package ID | Not scarce | Winget IDs are publisher-scoped, so you can publish a package that installs the `oa` binary without winning a global `oa` package name. |

That makes Homebrew tap, Homebrew core, Scoop, and AUR the clearest places to keep the short `oa` name aligned with the binary.

## Source checks

- Homebrew formula index: <https://formulae.brew.sh/api/formula.json>
- Scoop main bucket: <https://raw.githubusercontent.com/ScoopInstaller/Main/master/bucket/oa.json>
- AUR exact package lookup: <https://aur.archlinux.org/rpc/?v=5&type=info&arg[]=oa>
- npm package: <https://www.npmjs.com/package/oa>
