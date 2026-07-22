# Configuration and security

Patchlog loads `patchlog.yaml` by default. Unknown and misspelled YAML fields fail validation. Keep the file out of version control when it contains credentials; the repository `.gitignore` excludes it.

## Minimal provider configuration

```yaml
repo: fxdv/patchlog

provider:
  type: github
  repo: fxdv/patchlog
  token: ${GITHUB_TOKEN}
```

`provider.type`, `provider.repo`, and `provider.token` are preflighted before a publishing release can mutate version files.

## AI disclosure and controls

When a remote AI provider is enabled, prompt content derived from release data leaves the machine for the configured endpoint. Use exclusions and limits to constrain it:

```yaml
ai:
  provider: openai
  api_key: ${OPENAI_API_KEY}
  max_input_chars: 120000
  exclude_files:
    - "**/*.pem"
    - "**/.env*"
    - "vendor/**"
```

Patchlog redacts common credential forms before transmission. Exclusions and redaction are defense in depth; review configuration and generated prompts for highly sensitive repositories.

## Transport policy

Credential-bearing endpoints must use HTTPS. Loopback HTTP is allowed for local development. The global insecure override should be used only in isolated development environments:

```yaml
security:
  allow_insecure_credentials: false
```

HTTP clients reject cross-origin redirects so custom credential headers cannot be forwarded to another origin. Response bodies are bounded before decoding.

## Link templates

Commit, issue, and compare templates accept only HTTP(S) URLs:

```yaml
links:
  commit: https://github.com/%s/commit/%s
  issue: https://github.com/%s/issues/%s
  compare: https://github.com/%s/compare/%s...%s
```

For the exhaustive field reference and environment-variable mapping, see [REFERENCE.md](REFERENCE.md#configuration).
