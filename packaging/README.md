# Package-manager publishing

Each stable release renders two checksum-pinned manifests:

- `patchlog.rb`, consumed by the live `fxdv/homebrew-tap` repository.
- `patchlog.json`, consumed by the live `fxdv/scoop-bucket` repository.

The release workflow publishes these files as release assets, includes them in
`SHA256SUMS`, and installs/tests `patchlog.rb` on macOS and `patchlog.json` on
Windows. The external Homebrew
tap checks for a new stable release hourly, validates the formula, installs and
tests it, then commits the version update.

Install through the live tap:

```bash
brew tap fxdv/tap
brew install patchlog
```

Install through the Scoop bucket:

```powershell
scoop bucket add fxdv https://github.com/fxdv/scoop-bucket
scoop install fxdv/patchlog
```

The package-manager repositories distribute checksum-pinned manifests only;
Patchlog's release workflow remains the single artifact builder.

Publication follows the official repository layouts:

- [Homebrew tap documentation](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap)
- [Scoop app-manifest documentation](https://github.com/ScoopInstaller/Scoop/wiki/App-Manifests)
