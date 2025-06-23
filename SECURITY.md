# Security Policy

## Supported Versions

We actively support security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security vulnerability, please report it responsibly.

### How to Report

1. **DO NOT** create a public GitHub issue for security vulnerabilities
2. Send an email to the maintainers with details of the vulnerability
3. Include steps to reproduce the issue if possible
4. Allow reasonable time for the issue to be addressed before public disclosure

### What to Include

When reporting a vulnerability, please include:

- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact
- Suggested fix (if you have one)
- Your contact information

### Response Timeline

We aim to:

- Acknowledge receipt of vulnerability reports within 48 hours
- Provide an initial assessment within 5 business days
- Release security fixes as soon as possible after validation

### Security Best Practices

When using Wombat, please follow these security best practices:

#### Configuration Security
- **Never commit secrets** to version control
- Use environment variables or secure secret management systems
- Rotate credentials regularly
- Use the principle of least privilege for service accounts

#### Network Security
- Use encrypted connections (TLS/SSL) for all external communications
- Validate and sanitize all inputs
- Implement proper authentication and authorization
- Monitor and log security-relevant events

#### Deployment Security
- Keep Wombat updated to the latest version
- Regularly scan for known vulnerabilities using tools like `govulncheck`
- Run with minimal privileges in production
- Use container security best practices if deploying with containers

### Dependency Security

We regularly update dependencies to address security vulnerabilities:

- Dependencies are scanned using `govulncheck`
- Security updates are prioritized and released promptly
- We monitor security advisories for all dependencies

### Known Security Considerations

#### Data Processing
- Wombat processes potentially sensitive data streams
- Ensure appropriate data classification and handling
- Implement data encryption at rest and in transit where required

#### Plugin Security
- Third-party plugins may introduce security risks
- Validate and audit any custom plugins
- Follow the principle of least privilege for plugin permissions

#### Configuration Files
- Configuration files may contain sensitive information
- Protect configuration files with appropriate file permissions
- Consider using configuration encryption where sensitive data is required

## Security Updates

Security updates will be:

- Released as patch versions (e.g., 1.0.1)
- Documented in the changelog with security impact
- Announced through appropriate channels
- Backward compatible when possible

## Acknowledgments

We appreciate the security research community's efforts in responsibly disclosing vulnerabilities and helping improve Wombat's security posture.

---

For questions about this security policy, please contact the maintainers.