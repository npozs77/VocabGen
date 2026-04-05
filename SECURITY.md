# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Email the maintainer or use [GitHub's private vulnerability reporting](../../security/advisories/new)
3. Include a description of the vulnerability, steps to reproduce, and potential impact

You can expect an initial response within 7 days.

## Security Considerations

- API keys and AWS credentials are never stored in config files — use environment variables or CLI flags
- The application runs locally and does not expose any network services beyond `localhost`
- SQLite database is stored at `~/.vocabgen/vocabgen.db` with standard file permissions
- Outbound HTTPS connections are made only to configured LLM provider APIs and GitHub Releases (for update checks)
