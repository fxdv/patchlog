# Package-manager publishing

Each stable release renders two checksum-pinned manifests:

- `patchlog.rb`, ready to commit to the future `fxdv/homebrew-tap` repository.
- `patchlog.json`, ready to commit to the future `fxdv/scoop-bucket` repository.

The release workflow publishes these files as release assets and includes them
in `SHA256SUMS`. This repository deliberately does not claim that either
installation channel is live until the corresponding external repository and
its update credentials exist.

Homebrew is the first planned channel. Scoop follows after the tap is operating
and its installation verification is part of the release trust loop.

Publication follows the official repository layouts:

- [Homebrew tap documentation](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap)
- [Scoop app-manifest documentation](https://github.com/ScoopInstaller/Scoop/wiki/App-Manifests)
