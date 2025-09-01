# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## ⚠️ CRITICAL SECURITY AND LEGAL NOTICE

**THIS PROJECT IS STRICTLY FOR SECURITY RESEARCH AND DEFENSIVE PURPOSES ONLY**

- This tool is ONLY intended for legitimate security research, authorized audits, and defensive security operations
- ANY use of this project for unauthorized access, data theft, or malicious purposes is STRICTLY PROHIBITED and may violate computer fraud and abuse laws
- Users are SOLELY responsible for ensuring compliance with all applicable laws and regulations in their jurisdiction
- The original author and contributors assume NO legal responsibility for misuse of this tool
- You MUST have explicit authorization before using this tool on any system you do not own
- This tool should NEVER be used for attacking, credential harvesting, or any malicious intent
- All security research must be conducted ethically and within legal boundaries

## Project Overview

HackBrowserData is a command-line security research tool for extracting and decrypting browser data across multiple platforms (Windows, macOS, Linux). It supports data extraction from Chromium-based browsers (Chrome, Edge, Brave, etc.) and Firefox.

**Legitimate Use Cases**:
- Personal data backup and recovery
- Authorized enterprise security audits
- Digital forensics investigations (with proper authorization)
- Security vulnerability research and defense improvement
- Understanding browser security mechanisms for defensive purposes

## Development Commands

### Build the Project
```bash
# Build for current platform
cd cmd/hack-browser-data
go build

# Cross-compile for Windows from macOS/Linux
GOOS=windows GOARCH=amd64 go build

# Cross-compile for Linux from macOS/Windows  
GOOS=linux GOARCH=amd64 go build

# Cross-compile for macOS from Linux/Windows
GOOS=darwin GOARCH=amd64 go build
```

### Testing
```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v ./... -covermode=count -coverprofile=coverage.out

# Run specific package tests
go test -v ./browser/chromium/...
go test -v ./crypto/...
```

### Code Quality
```bash
# Format check
gofmt -d .

# Run linter (requires golangci-lint)
golangci-lint run

# Check spelling
typos

# Tidy dependencies
go mod tidy
```

## Architecture Overview

### Core Components

**Browser Abstraction Layer** (`browser/`)
- Interface-based design allowing easy addition of new browsers
- Platform-specific implementations using build tags (`_darwin.go`, `_windows.go`, `_linux.go`)
- Automatic profile discovery and multi-profile support

**Data Extraction Pipeline**
1. **Profile Discovery**: `profile/finder.go` locates browser profiles
2. **File Management**: `filemanager/` handles secure copying of browser files
3. **Decryption**: `crypto/` provides platform-specific decryption
   - Windows: DPAPI via Windows API
   - macOS: Keychain access (requires user password)
   - Linux: PBKDF2 key derivation
4. **Data Processing**: `browserdata/` parses and structures extracted data
5. **Output**: `browserdata/outputter.go` exports to CSV/JSON

**Key Interfaces**
- `Browser`: Main interface for browser implementations
- `DataType`: Enum for different data types (passwords, cookies, etc.)
- `BrowserData`: Container for all extracted browser data

### Platform-Specific Considerations

**macOS**
- Requires user password for Keychain access to decrypt Chrome passwords
- Uses Security framework for keychain operations
- Profile paths: `~/Library/Application Support/[Browser]/`

**Windows**
- Uses DPAPI for decryption (no password required)
- Accesses Local State file for encryption keys
- Profile paths: `%LOCALAPPDATA%/[Browser]/User Data/`

**Linux**
- Uses PBKDF2 with "peanuts" as salt
- Requires gnome-keyring or kwallet access
- Profile paths: `~/.config/[Browser]/`

### Security Mechanisms

**Data Protection**
- Temporary file cleanup after extraction
- No persistent storage of decrypted master keys
- Secure memory handling for sensitive data

**File Operations**
- Copy-on-read to avoid modifying original browser files
- Lock file filtering to prevent conflicts
- Atomic operations where possible

## Adding New Browser Support

1. Create browser-specific package in `browser/[name]/`
2. Implement the `Browser` interface
3. Add platform-specific profile paths in `browser/consts.go`
4. Register in `browser/browser.go` picker functions
5. Add data type mappings in `types/types.go`

## Important Files and Their Roles

- `cmd/hack-browser-data/main.go`: CLI entry point and flag handling
- `browser/chromium/chromium.go`: Core Chromium implementation
- `crypto/crypto_[platform].go`: Platform-specific decryption
- `extractor/extractor.go`: Main extraction orchestration
- `profile/finder.go`: Browser profile discovery logic
- `browserdata/password/password.go`: Password parsing and decryption

## Testing Considerations

- Tests use mocked data to avoid requiring actual browser installations
- Platform-specific tests are isolated with build tags
- Sensitive operations (like keychain access) are mocked in tests
- Use `DATA-DOG/go-sqlmock` for database operation testing

## Browser Security Analysis

### Chromium-Based Browsers Security

**Encryption Methods**:
- **Chrome v80+**: AES-256-GCM encryption for sensitive data
- **Pre-v80**: AES-128-CBC with PKCS#5 padding
- **Master Key Storage**:
  - Windows: Encrypted with DPAPI in `Local State` file
  - macOS: Stored in system Keychain (requires user password)
  - Linux: Derived using PBKDF2 with "peanuts" salt

**Data Protection Layers**:
1. **Password Storage**: Encrypted in SQLite database (`Login Data`)
2. **Cookie Encryption**: Encrypted values in `Cookies` database
3. **Credit Card Data**: Encrypted with same master key as passwords
4. **Local Storage**: Stored in LevelDB format, some values encrypted

### Firefox Security

**Encryption Architecture**:
- **Master Password**: Optional user-defined password for additional protection
- **Key Database**: `key4.db` stores encrypted master keys
- **NSS Library**: Network Security Services for cryptographic operations
- **Profile Encryption**: Each profile has independent encryption keys

**Key Derivation**:
- Uses PKCS#5 PBKDF2 for key derivation
- Triple-DES (3DES) for legacy compatibility
- AES-256-CBC for modern encryption
- ASN.1 encoding for key storage

### Platform-Specific Security Mechanisms

**Windows DPAPI (Data Protection API)**:
- User-specific encryption tied to Windows login
- No additional password required for decryption
- Keys protected by Windows security subsystem
- Vulnerable if attacker has user-level access

**macOS Keychain Services**:
- Requires user password for access
- Integration with system security framework
- Protected by System Integrity Protection (SIP)
- Security command-line tool for programmatic access

**Linux Secret Service**:
- GNOME Keyring or KDE Wallet integration
- D-Bus communication for key retrieval
- User session-based protection
- Fallback to PBKDF2 if keyring unavailable

### Security Vulnerabilities and Mitigations

**Known Attack Vectors**:
1. **Physical Access**: Direct file system access to browser profiles
2. **Memory Dumps**: Extraction of decrypted data from RAM
3. **Malware**: Keyloggers and info-stealers targeting browsers
4. **Process Injection**: DLL injection to extract decrypted data

**Defensive Recommendations**:
1. **Enable Master Password**: Firefox users should set master password
2. **Use OS-Level Encryption**: FileVault (macOS), BitLocker (Windows), LUKS (Linux)
3. **Regular Updates**: Keep browsers updated for latest security patches
4. **Profile Isolation**: Use separate profiles for sensitive activities
5. **Hardware Keys**: Use FIDO2/WebAuthn for critical accounts

### Cryptographic Implementation Details

**AES-GCM (Galois/Counter Mode)**:
- Authenticated encryption with associated data (AEAD)
- 96-bit nonce/IV for randomization
- 128-bit authentication tag for integrity
- Used in Chrome v80+ for enhanced security

**PBKDF2 (Password-Based Key Derivation Function 2)**:
- Iterations: 1003 (macOS), 1 (Linux default)
- Hash function: SHA-1 (legacy) or SHA-256
- Salt: "saltysalt" (Chrome), "peanuts" (Linux)
- Output: 128-bit or 256-bit keys

**DPAPI Internals**:
- Uses CryptProtectData/CryptUnprotectData Windows APIs
- Machine-specific or user-specific encryption
- Automatic key management by Windows
- Integrates with Windows credential manager

## Dependencies

- `modernc.org/sqlite`: Pure Go SQLite for cross-platform compatibility
- `github.com/godbus/dbus`: Linux keyring access
- `github.com/ppacher/go-dbus-keyring`: Secret service integration
- `github.com/tidwall/gjson`: JSON parsing for browser preferences
- `github.com/syndtr/goleveldb`: LevelDB for IndexedDB/LocalStorage

## Ethical Usage Guidelines

### Responsible Disclosure
- Report vulnerabilities to browser vendors through official channels
- Allow reasonable time for patches before public disclosure
- Never exploit vulnerabilities for personal gain

### Legal Compliance
- Obtain written authorization before testing third-party systems
- Comply with GDPR, CCPA, and other privacy regulations
- Respect intellectual property and terms of service
- Maintain audit logs of all security testing activities

### Best Practices for Security Researchers
1. **Scope Definition**: Clearly define testing boundaries
2. **Data Handling**: Securely delete any extracted sensitive data
3. **Documentation**: Maintain detailed records of methodologies
4. **Collaboration**: Work with security community ethically
5. **Education**: Share knowledge to improve overall security