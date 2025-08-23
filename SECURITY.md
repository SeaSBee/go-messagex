# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| 0.x.x   | :white_check_mark: |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to our security team:

**Email**: security@seasbee.com

You should receive a response within 48 hours. If for some reason you do not, please follow up via email to ensure we received your original message.

Please include the requested information listed below (as much as you can provide) to help us better understand the nature and scope of the possible issue:

- Type of issue (buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the vulnerability
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

This information will help us triage your report more quickly.

## Preferred Languages

We prefer all communications to be in English.

## Disclosure Policy

When we receive a security bug report, we will assign it to a primary handler. This person will coordinate the fix and release process, involving the following steps:

1. Confirm the problem and determine the affected versions.
2. Audit code to find any similar problems.
3. Prepare fixes for all supported versions. These fixes will be released as new minor versions.

## Security Best Practices

### For Users

1. **Keep Dependencies Updated**: Regularly update go-messagex to the latest version
2. **Use TLS**: Always use TLS/mTLS for production deployments
3. **Environment Variables**: Store sensitive configuration in environment variables only
4. **Principle of Least Privilege**: Use minimal required permissions for RabbitMQ users
5. **Network Security**: Restrict network access to messaging infrastructure
6. **Monitoring**: Monitor for unusual activity and failed authentication attempts

### For Developers

1. **Input Validation**: Always validate and sanitize inputs
2. **Secret Management**: Never log or expose sensitive information
3. **Error Handling**: Avoid exposing internal system details in error messages
4. **Dependency Scanning**: Regularly scan dependencies for known vulnerabilities
5. **Code Review**: Ensure security-focused code reviews for all changes

## Security Features

### Built-in Security

- **TLS/mTLS Support**: End-to-end encryption with certificate management
- **Hostname Verification**: Enabled by default for security
- **Secret Masking**: Automatic masking of sensitive data in logs
- **Input Validation**: Comprehensive validation of configuration and messages
- **Error Sanitization**: Safe error messages that don't leak sensitive information

### Security Headers

- **Idempotency Keys**: Prevent duplicate message processing
- **Correlation IDs**: Track message flow for security auditing
- **Message Signing**: Optional HMAC verification support

## Responsible Disclosure

We are committed to working with security researchers to resolve security issues. We ask that you:

1. **Give us reasonable time** to respond to issues before any disclosure
2. **Provide detailed information** to help us reproduce and fix the issue
3. **Work with us** to coordinate disclosure if needed
4. **Follow responsible disclosure practices** and avoid public disclosure before fixes are available

## Security Updates

Security updates will be released as:

- **Patch releases** (e.g., 1.2.3) for critical security fixes
- **Minor releases** (e.g., 1.3.0) for security features and improvements
- **Major releases** (e.g., 2.0.0) for breaking security changes

## Security Contacts

- **Security Team**: security@seasbee.com
- **Maintainers**: @seasbee/messaging-team
- **Emergency Contact**: security@seasbee.com (24/7 for critical issues)

## Acknowledgments

We would like to thank all security researchers who responsibly disclose vulnerabilities to us. Your contributions help make go-messagex more secure for everyone.

---

**Thank you for helping keep go-messagex secure! 🔒**
