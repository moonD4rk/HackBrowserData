# RFC-001: HackBrowserData Architecture Refactoring

**Author**: moonD4rk  
**Status**: Proposed  
**Created**: 2025-09-01  
**Updated**: 2025-09-01  

## Abstract

This RFC analyzes the current architectural issues in the HackBrowserData project and proposes refactoring directions. The core goal of the refactoring is to establish a modular, extensible, and testable architecture while supporting usage as a library that can be imported by other projects.

## Current Issues Analysis

### 1. Limited Encryption Version Support

**Current State**:
- Only supports Chrome v10 (Chrome 80+) AES-GCM encryption format
- Hardcoded "v10" prefix handling logic in the code
- Lacks version detection and dynamic selection mechanism

**Impact**:
- Unable to support data extraction from older browser versions
- Cannot adapt to future browser encryption algorithm upgrades (e.g., v11, v20)
- Chrome is introducing new encryption mechanisms (e.g., App-Bound Encryption in Chrome 127+), which the current architecture struggles to extend

### 2. Scattered Cross-Platform MasterKey Retrieval

**Current State**:
- Windows: Decrypts encrypted_key from Local State via DPAPI
- macOS: Accesses Keychain through security command, derives key using PBKDF2
- Linux: Accesses Secret Service via D-Bus or uses hardcoded "peanuts" salt

**Issues**:
- Each platform implementation is completely independent without a unified interface
- Difficult to add new key retrieval methods
- Code duplication and maintenance challenges
- Chrome on Windows is updating retrieval methods, requiring support for multiple strategies

### 3. Windows Cookie File Access Permission Issues

**Specific Issues**:
- On Windows, browsers lock Cookie files during runtime
- Direct reading may encounter "The process cannot access the file" errors
- Some security software blocks access to Cookie files

**Current Approach Limitations**:
- Simple file copying may fail due to file locking
- Lacks alternative access strategies (e.g., shadow copy, process injection)
- No abstraction for permission elevation or bypass mechanisms

### 4. Coupled Code Architecture

**Problems**:
- CLI logic mixed with core functionality
- Data extraction, decryption, and output are tightly coupled
- Uses global variables and functions, difficult to use as a library

**Specific Impact**:
- Cannot use core functionality independently
- Difficult to unit test
- Code reuse challenges

### 5. Inconsistent Error Handling

**Current State**:
- Some functions return errors, others directly use logging
- Error messages lack context (which browser, data type, platform)
- Cannot distinguish error severity (ignorable vs. fatal errors)

**Impact**:
- Debugging difficulties with insufficient error information
- Cannot implement flexible error handling strategies
- Inconsistent user experience

### 6. Testing and Maintenance Difficulties

**Issues**:
- Depends on real file system and browser installations
- Cannot mock system calls and external dependencies
- Low test coverage
- Adding new features requires modifying multiple code locations

## Architecture Improvement Proposals

### 1. Versioned Encryption Strategies

**Design Approach**:
- Create encryption version interface where each version implements its own detection and decryption logic
- Use registration mechanism to manage all supported versions
- Support both automatic detection and manual version specification

**Key Capabilities**:
- Version Detection: Automatically identify encryption version through data characteristics
- Version Registration: Dynamically register new encryption version implementations
- Priority Control: Try different versions by priority

### 2. Unified MasterKey Retrieval Abstraction

**Design Approach**:
- Define cross-platform MasterKey retrieval interface
- Each platform can have multiple retrieval strategies
- Support strategy chain, trying different methods sequentially

**Windows Strategy Examples**:
- DPAPI Strategy (traditional method)
- App-Bound Strategy (Chrome 127+)
- Cloud Sync Strategy (potential future)

**Key Capabilities**:
- Platform detection and automatic selection
- Strategy priority and fallback mechanisms
- Error handling and logging

### 3. File Access Abstraction Layer

**Design Approach**:
- Create file access interface encapsulating different access strategies
- For Windows Cookie issues, implement multiple access methods
- Provide unified error handling and retry mechanisms

**Windows Cookie Access Strategies**:
- Direct Copy (current method)
- Volume Shadow Copy Service (VSS)
- Memory Reading (from browser process)
- Stream Reading (bypass exclusive locks)

### 4. Layered Package Structure

**Design Principles**:
- Separate public API from internal implementation
- Separate interface definitions from concrete implementations
- Isolate platform-specific code

**Package Structure Plan**:
```
pkg/           # Public API (externally importable)
├── browser/   # Browser interface definitions
├── crypto/    # Encryption interface definitions
└── extractor/ # Data extractor interface definitions

internal/      # Internal implementation (not exposed)
├── browser/   # Browser implementations
├── crypto/    # Encryption algorithm implementations
└── platform/  # Platform-specific implementations
```

### 5. Improved Browser Interface

**Design Goals**:
- Support dependency injection
- Configurable and extensible
- Easy to test

**Core Methods**:
- Configuration settings (profile, crypto provider, etc.)
- Data extraction (support selecting data types)
- Capability queries (supported data types and platforms)

### 6. Unified Error Handling

**Design Approach**:
- Define structured error types
- Include rich context information
- Support error classification and handling strategies

**Error Information Should Include**:
- Operation type
- Browser name
- Data type
- Platform information
- Severity level
- Original error

### 7. Library API Design

**Design Goals**:
- Provide clean client interface
- Support convenient methods for common use cases
- Allow advanced users to customize behavior

**Use Cases**:
- Simple: One-click extraction of all browser data
- Advanced: Custom encryption versions, error handling, data filtering

### 8. Testing Strategy

**Improvement Directions**:
- Use interfaces instead of concrete implementations
- Support dependency injection
- Provide mock implementations

**Test Types**:
- Unit tests: Test independent components
- Integration tests: Test component interactions
- Platform tests: Test platform-specific functionality

## Implementation Recommendations

### Priority Levels

1. **High Priority**:
   - Versioned encryption strategies (solve version support issues)
   - MasterKey retrieval abstraction (unify cross-platform implementations)
   - Windows Cookie access issues (solve permission problems)

2. **Medium Priority**:
   - Browser interface refactoring
   - Unified error handling
   - Basic testing framework

3. **Low Priority**:
   - Complete library API
   - Advanced feature extensions
   - Performance optimizations

### Compatibility Considerations

- Keep CLI backward compatible, internally calling new architecture
- Provide migration documentation
- Gradually deprecate old APIs across versions

## Security Considerations

1. **Minimize Permissions**: Only request necessary system permissions
2. **Memory Safety**: Zero out sensitive data after use
3. **Error Messages**: Avoid leaking sensitive information
4. **Input Validation**: Strictly validate paths and data

## Open Questions

1. **File Access Strategy Selection**: How to automatically select the best file access strategy?
2. **Error Recovery**: How to gracefully recover and continue when encountering partial failures?
3. **Configuration Management**: Should configuration files be supported to control behavior?
4. **Plugin System**: Should user-defined data extractors be supported?

## References

- [Chromium OS Crypt](https://source.chromium.org/chromium/chromium/src/+/main:components/os_crypt/)
- [Chrome Password Decryption](https://github.com/chromium/chromium/blob/main/components/os_crypt/sync/os_crypt_win.cc)
- [Firefox NSS](https://developer.mozilla.org/en-US/docs/Mozilla/Projects/NSS)
- [Windows File Locking](https://docs.microsoft.com/en-us/windows/win32/fileio/locking-and-unlocking-byte-ranges-in-files)