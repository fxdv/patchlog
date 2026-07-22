# Security Policy

## Reporting a vulnerability

Do not open a public issue containing exploit details, credentials, private
repository content, or customer data. Report vulnerabilities privately through
the repository host's security-advisory feature. Maintainers should acknowledge
the report within five business days and coordinate disclosure after a fix is
available.

## Credential handling

- Keep `patchlog.yaml` local and use environment-variable references. The file
  is ignored; copy `patchlog.example.yaml` to start.
- Never commit API tokens. If a token appears in Git history, terminal logs, CI
  output, or a shared review, revoke and replace it immediately; deleting the
  current file is not sufficient.
- Credential-bearing integrations require HTTPS. Loopback HTTP is allowed for
  local test servers; any other exception requires the explicit
  `security.allow_insecure_credentials` setting.

## AI data disclosure

When an AI feature is enabled, patchlog may send commit messages, selected code
diffs, linked issue text, and computed release metrics to the configured AI
endpoint. Before the first remote request, the CLI prints the endpoint, data
categories, excluded file patterns, and request-size limit. A shared boundary
redacts common secret formats, excludes configured sensitive/generated paths,
and rejects prompts over `ai.max_input_chars`. Redaction is defense in depth;
review endpoint ownership and exclusions before sending private source code.

## Release verification

Official release assets include `SHA256SUMS`. Verify the checksum before
installing and prefer an immutable version instead of `latest`.
