# Security Guide

This guide provides comprehensive security considerations and best practices for go-messagex applications.

## Table of Contents

1. [Security Overview](#security-overview)
2. [Transport Security](#transport-security)
3. [Authentication & Authorization](#authentication--authorization)
4. [Secret Management](#secret-management)
5. [Message Security](#message-security)
6. [Input Validation](#input-validation)
7. [Network Security](#network-security)
8. [Audit & Monitoring](#audit--monitoring)
9. [Security Checklist](#security-checklist)
10. [Vulnerability Reporting](#vulnerability-reporting)

## Security Overview

go-messagex is designed with security-first principles, providing multiple layers of protection for your messaging infrastructure.

### Security Principles

- **Defense in Depth**: Multiple security layers
- **Principle of Least Privilege**: Minimal required permissions
- **Secure by Default**: Security features enabled by default
- **Zero Trust**: Verify everything, trust nothing
- **Security Through Obscurity**: Not relied upon

### Security Features

- **TLS/mTLS Encryption**: End-to-end encryption
- **Hostname Verification**: Prevents MITM attacks
- **Secret Management**: Secure credential handling
- **Message Signing**: HMAC verification support
- **Input Validation**: Comprehensive validation
- **Audit Logging**: Security event tracking

## Transport Security

### TLS Configuration

```go
config := &messaging.Config{
    RabbitMQ: &messaging.RabbitMQConfig{
        URIs: []string{"amqps://user:pass@host:5671/vhost"},
        TLS: &messaging.TLSConfig{
            Enabled: true,
            CAFile:   "/etc/ssl/ca.pem",
            CertFile: "/etc/ssl/client.crt",
            KeyFile:  "/etc/ssl/client.key",
            
            // Security hardening
            MinVersion: tls.VersionTLS12,
            MaxVersion: tls.VersionTLS13,
            
            // Cipher suite preferences
            CipherSuites: []uint16{
                tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
                tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
            },
            
            // Certificate verification
            InsecureSkipVerify: false, // Always verify certificates
            ServerName:         "rabbitmq.example.com",
        },
    },
}
```

### Certificate Management

```bash
# Generate CA certificate
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 365 -key ca.key -out ca.crt

# Generate client certificate
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr
openssl x509 -req -days 365 -in client.csr -CA ca.crt -CAkey ca.key -out client.crt
```

### Security Best Practices

```go
// Always use TLS in production
if !config.RabbitMQ.TLS.Enabled {
    logx.Fatal("TLS must be enabled in production")
}

// Verify certificate hostname
if config.RabbitMQ.TLS.ServerName == "" {
    logx.Fatal("Server name must be specified for certificate verification")
}

// Use strong cipher suites
if len(config.RabbitMQ.TLS.CipherSuites) == 0 {
    config.RabbitMQ.TLS.CipherSuites = []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
    }
}
```

## Authentication & Authorization

### User Management

```bash
# Create application user with minimal permissions
rabbitmqctl add_user app_user secure_password
rabbitmqctl set_permissions -p / app_user "app.*" "app.*" "app.*"

# Create monitoring user with read-only access
rabbitmqctl add_user monitor_user monitor_password
rabbitmqctl set_permissions -p / monitor_user "" "" ".*"
```

### Connection Security

```go
// Use environment variables for credentials
config := &messaging.Config{
    RabbitMQ: &messaging.RabbitMQConfig{
        URIs: []string{
            os.Getenv("RABBITMQ_URI"),
        },
    },
}

// Validate connection security
func validateConnectionSecurity(config *messaging.Config) error {
    if !config.RabbitMQ.TLS.Enabled {
        return errors.New("TLS must be enabled")
    }
    
    if config.RabbitMQ.TLS.InsecureSkipVerify {
        return errors.New("certificate verification cannot be disabled")
    }
    
    return nil
}
```

### Access Control

```go
// Implement role-based access control
type AccessControl struct {
    roles map[string][]string
}

func (ac *AccessControl) CanPublish(user, exchange string) bool {
    permissions := ac.roles[user]
    for _, perm := range permissions {
        if perm == "publish:"+exchange || perm == "publish:*" {
            return true
        }
    }
    return false
}

func (ac *AccessControl) CanConsume(user, queue string) bool {
    permissions := ac.roles[user]
    for _, perm := range permissions {
        if perm == "consume:"+queue || perm == "consume:*" {
            return true
        }
    }
    return false
}
```

## Secret Management

### Environment Variables

```bash
# Use environment variables for secrets
export RABBITMQ_URI="amqps://user:pass@host:5671/vhost"
export RABBITMQ_TLS_CA_FILE="/etc/ssl/ca.pem"
export RABBITMQ_TLS_CERT_FILE="/etc/ssl/client.crt"
export RABBITMQ_TLS_KEY_FILE="/etc/ssl/client.key"

# Use secret management tools
export RABBITMQ_URI=$(vault kv get -field=uri rabbitmq/connection)
```

### Secret Masking

```go
// Mask sensitive information in logs
func maskURI(uri string) string {
    if strings.Contains(uri, "@") {
        parts := strings.Split(uri, "@")
        if len(parts) == 2 {
            userPass := strings.Split(parts[0], "://")
            if len(userPass) == 2 {
                return userPass[0] + "://***:***@" + parts[1]
            }
        }
    }
    return "***"
}

// Use in logging
logx.Info("Connecting to RabbitMQ",
    logx.String("uri", maskURI(config.RabbitMQ.URIs[0])),
)
```

### Secure Configuration Loading

```go
// Load secrets securely
func loadSecrets() (*messaging.Config, error) {
    config := &messaging.Config{}
    
    // Load from environment variables
    if uri := os.Getenv("RABBITMQ_URI"); uri != "" {
        config.RabbitMQ.URIs = []string{uri}
    }
    
    // Load from secret files
    if caFile := os.Getenv("RABBITMQ_TLS_CA_FILE"); caFile != "" {
        config.RabbitMQ.TLS.CAFile = caFile
    }
    
    // Validate configuration
    if err := validateSecrets(config); err != nil {
        return nil, err
    }
    
    return config, nil
}

func validateSecrets(config *messaging.Config) error {
    // Check file permissions
    if config.RabbitMQ.TLS.KeyFile != "" {
        info, err := os.Stat(config.RabbitMQ.TLS.KeyFile)
        if err != nil {
            return err
        }
        
        mode := info.Mode()
        if mode&0077 != 0 {
            return errors.New("private key file has insecure permissions")
        }
    }
    
    return nil
}
```

## Message Security

### Message Signing

```go
// Sign messages with HMAC
type MessageSigner struct {
    secretKey []byte
}

func (ms *MessageSigner) SignMessage(msg messaging.Message) error {
    // Create signature
    h := hmac.New(sha256.New, ms.secretKey)
    h.Write(msg.Body)
    h.Write([]byte(msg.ID))
    h.Write([]byte(msg.Key))
    
    signature := hex.EncodeToString(h.Sum(nil))
    
    // Add signature to headers
    if msg.Headers == nil {
        msg.Headers = make(map[string]interface{})
    }
    msg.Headers["x-signature"] = signature
    
    return nil
}

func (ms *MessageSigner) VerifyMessage(msg messaging.Message) error {
    signature, ok := msg.Headers["x-signature"].(string)
    if !ok {
        return errors.New("message signature not found")
    }
    
    // Verify signature
    h := hmac.New(sha256.New, ms.secretKey)
    h.Write(msg.Body)
    h.Write([]byte(msg.ID))
    h.Write([]byte(msg.Key))
    
    expectedSignature := hex.EncodeToString(h.Sum(nil))
    
    if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
        return errors.New("message signature verification failed")
    }
    
    return nil
}
```

### Message Encryption

```go
// Encrypt sensitive message content
type MessageEncryptor struct {
    key []byte
}

func (me *MessageEncryptor) EncryptMessage(msg messaging.Message) error {
    block, err := aes.NewCipher(me.key)
    if err != nil {
        return err
    }
    
    // Generate random IV
    iv := make([]byte, aes.BlockSize)
    if _, err := io.ReadFull(rand.Reader, iv); err != nil {
        return err
    }
    
    // Encrypt message body
    ciphertext := make([]byte, len(msg.Body))
    stream := cipher.NewCFBEncrypter(block, iv)
    stream.XORKeyStream(ciphertext, msg.Body)
    
    // Update message
    msg.Body = append(iv, ciphertext...)
    msg.ContentType = "application/octet-stream"
    
    return nil
}

func (me *MessageEncryptor) DecryptMessage(msg messaging.Message) error {
    if len(msg.Body) < aes.BlockSize {
        return errors.New("message too short")
    }
    
    block, err := aes.NewCipher(me.key)
    if err != nil {
        return err
    }
    
    // Extract IV
    iv := msg.Body[:aes.BlockSize]
    ciphertext := msg.Body[aes.BlockSize:]
    
    // Decrypt message body
    plaintext := make([]byte, len(ciphertext))
    stream := cipher.NewCFBDecrypter(block, iv)
    stream.XORKeyStream(plaintext, ciphertext)
    
    msg.Body = plaintext
    return nil
}
```

## Input Validation

### Message Validation

```go
// Validate message content
func validateMessage(msg messaging.Message) error {
    // Check message size
    if len(msg.Body) > MaxMessageSize {
        return errors.New("message too large")
    }
    
    // Validate message ID
    if msg.ID == "" {
        return errors.New("message ID required")
    }
    
    if len(msg.ID) > MaxIDLength {
        return errors.New("message ID too long")
    }
    
    // Validate routing key
    if msg.Key == "" {
        return errors.New("routing key required")
    }
    
    // Validate content type
    if msg.ContentType == "" {
        return errors.New("content type required")
    }
    
    // Validate JSON content
    if msg.ContentType == "application/json" {
        if !json.Valid(msg.Body) {
            return errors.New("invalid JSON content")
        }
    }
    
    return nil
}
```

### Configuration Validation

```go
// Validate configuration security
func validateConfigSecurity(config *messaging.Config) error {
    // Check TLS configuration
    if !config.RabbitMQ.TLS.Enabled {
        return errors.New("TLS must be enabled")
    }
    
    // Check certificate files
    if config.RabbitMQ.TLS.CAFile == "" {
        return errors.New("CA certificate file required")
    }
    
    if config.RabbitMQ.TLS.CertFile == "" {
        return errors.New("client certificate file required")
    }
    
    if config.RabbitMQ.TLS.KeyFile == "" {
        return errors.New("client key file required")
    }
    
    // Check URIs
    for _, uri := range config.RabbitMQ.URIs {
        if !strings.HasPrefix(uri, "amqps://") {
            return errors.New("only AMQPS URIs allowed")
        }
    }
    
    return nil
}
```

## Network Security

### Network Segmentation

```go
// Implement network security policies
type NetworkPolicy struct {
    allowedHosts []string
    allowedPorts []int
}

func (np *NetworkPolicy) IsAllowedHost(host string) bool {
    for _, allowed := range np.allowedHosts {
        if host == allowed {
            return true
        }
    }
    return false
}

func (np *NetworkPolicy) IsAllowedPort(port int) bool {
    for _, allowed := range np.allowedPorts {
        if port == allowed {
            return true
        }
    }
    return false
}
```

### Firewall Configuration

```bash
# Configure firewall rules
sudo ufw allow from 10.0.0.0/8 to any port 5671 proto tcp
sudo ufw allow from 172.16.0.0/12 to any port 5671 proto tcp
sudo ufw deny 5672/tcp  # Block non-TLS AMQP
```

### Network Monitoring

```go
// Monitor network connections
func monitorNetworkConnections(transport *rabbitmq.Transport) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := transport.GetConnectionStats()
        
        // Log connection metrics
        logx.Info("Network connections",
            logx.Int("active", stats.ActiveConnections),
            logx.Int("failed", stats.FailedConnections),
            logx.Duration("avg_latency", stats.AverageLatency),
        )
        
        // Alert on connection failures
        if stats.FailedConnections > 10 {
            logx.Error("High connection failure rate",
                logx.Int("failed", stats.FailedConnections),
            )
        }
    }
}
```

## Audit & Monitoring

### Security Event Logging

```go
// Log security events
type SecurityLogger struct {
    logger logx.Logger
}

func (sl *SecurityLogger) LogAuthentication(user, source string, success bool) {
    level := logx.InfoLevel
    if !success {
        level = logx.WarnLevel
    }
    
    sl.logger.Log(level, "Authentication event",
        logx.String("user", user),
        logx.String("source", source),
        logx.Bool("success", success),
        logx.String("timestamp", time.Now().UTC().Format(time.RFC3339)),
    )
}

func (sl *SecurityLogger) LogAuthorization(user, resource, action string, allowed bool) {
    level := logx.InfoLevel
    if !allowed {
        level = logx.WarnLevel
    }
    
    sl.logger.Log(level, "Authorization event",
        logx.String("user", user),
        logx.String("resource", resource),
        logx.String("action", action),
        logx.Bool("allowed", allowed),
        logx.String("timestamp", time.Now().UTC().Format(time.RFC3339)),
    )
}

func (sl *SecurityLogger) LogSecurityViolation(event, details string) {
    sl.logger.Error("Security violation",
        logx.String("event", event),
        logx.String("details", details),
        logx.String("timestamp", time.Now().UTC().Format(time.RFC3339)),
    )
}
```

### Security Metrics

```go
// Track security metrics
type SecurityMetrics struct {
    AuthenticationFailures prometheus.Counter
    AuthorizationFailures  prometheus.Counter
    SecurityViolations     prometheus.Counter
    TLSHandshakeFailures   prometheus.Counter
}

func (sm *SecurityMetrics) RecordAuthFailure() {
    sm.AuthenticationFailures.Inc()
}

func (sm *SecurityMetrics) RecordAuthzFailure() {
    sm.AuthorizationFailures.Inc()
}

func (sm *SecurityMetrics) RecordViolation() {
    sm.SecurityViolations.Inc()
}

func (sm *SecurityMetrics) RecordTLSFailure() {
    sm.TLSHandshakeFailures.Inc()
}
```

## Security Checklist

### Pre-deployment Security Review

- [ ] TLS/mTLS enabled and configured
- [ ] Certificate validation enabled
- [ ] Strong cipher suites configured
- [ ] Secrets stored securely (not in code)
- [ ] Input validation implemented
- [ ] Access controls configured
- [ ] Audit logging enabled
- [ ] Security monitoring configured
- [ ] Network segmentation implemented
- [ ] Firewall rules configured

### Runtime Security Monitoring

- [ ] Monitor authentication failures
- [ ] Monitor authorization failures
- [ ] Monitor TLS handshake failures
- [ ] Monitor connection anomalies
- [ ] Monitor message signing failures
- [ ] Monitor security violations
- [ ] Review audit logs regularly
- [ ] Update security patches
- [ ] Rotate secrets regularly
- [ ] Test security controls

### Incident Response

- [ ] Document security incident procedures
- [ ] Establish incident response team
- [ ] Define escalation procedures
- [ ] Prepare communication templates
- [ ] Test incident response procedures
- [ ] Maintain incident logs
- [ ] Conduct post-incident reviews
- [ ] Update security controls

## Vulnerability Reporting

### Reporting Security Issues

**DO NOT** create public issues for security vulnerabilities. Instead:

1. **Email**: security@seasbee.com
2. **Subject**: [SECURITY] go-messagex vulnerability
3. **Include**:
   - Detailed description of the vulnerability
   - Steps to reproduce
   - Potential impact assessment
   - Suggested fix (if available)

### Security Response Process

1. **Acknowledgment**: Within 24 hours
2. **Assessment**: Within 72 hours
3. **Fix Development**: As needed
4. **Testing**: Comprehensive testing
5. **Release**: Coordinated release
6. **Disclosure**: Public disclosure after fix

### Responsible Disclosure

- Allow time for assessment and fix
- Coordinate disclosure timing
- Provide credit for responsible reporting
- Maintain confidentiality during process

---

**Security is everyone's responsibility. Stay vigilant and report any concerns immediately.**
