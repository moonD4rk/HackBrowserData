# RFC-002: HackBrowserData API Design and Naming Conventions

**Author**: moonD4rk  
**Status**: In Discussion  
**Created**: 2025-09-07  
**Updated**: 2025-09-07  

## Abstract

This RFC documents discussions regarding API design, naming conventions, and architectural decisions during the HackBrowserData project refactoring. The document is divided into three parts: existing code analysis, confirmed directions, and issues under discussion.

## Part 1: Existing Code Analysis

### 1.1 Project Structure

```
hackbrowserdata/
├── browser/           # Browser implementations
│   ├── chromium/     # Chromium-based browsers
│   └── firefox/      # Firefox browser
├── browserdata/      # Data processing
├── crypto/           # Encryption/decryption (platform-specific)
├── extractor/        # Data extractors
├── types/            # Type definitions
└── utils/            # Utility functions
```

### 1.2 Current Data Flow

#### 1.2.1 Chromium-based Browsers (Chrome/Edge/Brave)

**Overall Flow**:
```
Entry → Browser Discovery → Profile Scanning → MasterKey Retrieval → Data Extraction → Decryption → Output
```

**Windows Platform Detailed Flow**:
```
1. Browser Discovery
   main.go: PickBrowsers("chrome", profilePath)
   ↓
   browser.go: pickChromium() 
   ↓
   Check path: %LOCALAPPDATA%\Google\Chrome\User Data

2. Profile Scanning
   chromium.New() → userDataTypePaths()
   Scan: Default/, Profile 1/, Profile 2/, etc.
   Create Chromium instance for each profile

3. MasterKey Retrieval (shared across all profiles)
   chromium_windows.go: GetMasterKey()
   ↓
   Read: Local State file (JSON format)
   Extract: os_crypt.encrypted_key field
   Decode: Base64 decode
   Process: Skip first 5 bytes "DPAPI" prefix
   Decrypt: crypto.DecryptWithDPAPI()
   Result: 16-byte MasterKey

4. Data File Copying
   copyItemToLocal()
   Copy: Login Data, Cookies, History, etc. to temp directory

5. Data Extraction and Decryption
   browserdata.Recovery(masterKey)
   ↓
   Call corresponding Extractor for each DataType
   ↓
   password.go: ChromiumPassword.Extract()
   Query SQLite: SELECT origin_url, username_value, password_value
   Decryption flow:
   - First try: crypto.DecryptWithDPAPI(pwd)
   - On failure: crypto.DecryptWithChromium(masterKey, pwd)
```

**macOS Platform Differences**:
```
3. MasterKey Retrieval
   chromium_darwin.go: GetMasterKey()
   ↓
   Execute: security find-generic-password -wa 'Chrome'
   Get: Password from Keychain
   Derive: crypto.PBKDF2Key(password, "saltysalt", 1003, 16)
   Result: 16-byte MasterKey

5. Decryption Differences
   Uses AES-CBC instead of AES-GCM
```

**Linux Platform Differences**:
```
3. MasterKey Retrieval
   chromium_linux.go: GetMasterKey()
   ↓
   Try: Get from gnome-keyring or kwallet
   Fallback: Use hardcoded "peanuts"
   Derive: PBKDF2(password, "saltysalt", 1, 16)
```

#### 1.2.2 Firefox Browser

**All Platforms Unified Flow**:
```
1. Browser Discovery
   pickFirefox() → firefox.New()
   Scan: ~/Library/Application Support/Firefox/Profiles/

2. Profile Processing (each completely independent)
   Scan: *.default-release directories
   Each profile has independent keys and data

3. MasterKey Retrieval (per-profile)
   Read: key4.db (SQLite)
   Query: SELECT item1, item2 FROM metadata WHERE id = 'password'
   Decrypt: NSS library decryption (3DES or AES)

4. Data Extraction
   Read: logins.json
   Decrypt: Using that profile's MasterKey
```

### 1.3 Current Naming System

#### Current Type Definitions (types/types.go)
```go
type DataType int

const (
    ChromiumKey DataType = iota
    ChromiumPassword
    ChromiumCookie
    ChromiumBookmark
    ChromiumHistory
    // ... 
    FirefoxPassword
    FirefoxCookie
    // ...
)
```

**Issues**:
1. DataType mixes browser types with data types
2. Browser names passed as strings ("chrome", "firefox")
3. Profile concept inconsistent across browsers

#### Current Data Structures
```go
// browser/chromium/chromium.go
type Chromium struct {
    name        string              // Browser name
    storage     string              // Keychain service name
    profilePath string              // Profile path
    masterKey   []byte              // Decryption key
    dataTypes   []types.DataType    // Data types
    Paths       map[types.DataType]string
}

// browserdata/browserdata.go
type BrowserData struct {
    extractors map[types.DataType]extractor.Extractor
}
```

## Part 2: Confirmed Directions

### 2.1 API Design Principles

**Confirmed: Function Options Pattern**, reasons:
- Follows Go conventions (similar to grpc, zap, etc.)
- Provides type safety and IDE auto-completion
- Easy to extend without breaking existing code
- Zero value usability

**Basic Usage Pattern** (conceptual, specific naming TBD):
```go
// Conceptual example, naming not finalized
client := hbd.New(
    hbd.WithBrowser(...),  // Browser selection
    hbd.WithItems(...),    // Data type selection
)
result, err := client.Extract()
```

### 2.2 Design Principles

1. **No Strings**: All options use type-safe enums
2. **Separate Extract and Save**: Data extraction and saving are independent operations
3. **Cross-platform Transparency**: Use build tags for platform differences
4. **No Backward Compatibility**: New version can be completely redesigned
5. **Performance Not Critical**: Prioritize code clarity and testability

## Part 3: Issues Under Discussion

### 3.1 Type Naming (Core Issue)

Naming system to be determined:

#### Browser Type Naming
```go
// Option A: Simple and direct
type Browser int
const (
    Chrome Browser = iota
    Firefox
    Edge
)

// Option B: With suffix for distinction
type BrowserType int
const (
    ChromeBrowser BrowserType = iota
    FirefoxBrowser
)

// How to express internal engine concept?
// ChromiumEngine vs GeckoEngine?
```

#### Data Type Naming
```go
// How to solve plural issues with words like History?

// Option A: Use Item suffix
type Item int
const (
    PasswordItem Item = iota
    CookieItem
    HistoryItem  // Avoids plural issue
)

// Option B: Accept grammatical oddity
type DataType int
const (
    Passwords DataType = iota  // Plural
    Cookies
    Histories  // Grammatically odd but consistent
)
```

### 3.2 Data Flow Stage Naming

Naming for data at different stages is undecided:

```
File → ??? → ??? → ??? → User Result

Possible naming chains:
1. FileData → EncryptedData → DecryptedData → ParsedData → Result
2. RawData → SecureData → PlainData → StructuredData → Output
3. Source → Encrypted → Plain → Structured → Result
```

### 3.3 Profile Concept Unification

**Biggest Challenge**: Chrome and Firefox have completely different Profile concepts

Chrome Profile characteristics:
- Subdirectory under User Data
- All profiles share one MasterKey
- Profile only for data isolation

Firefox Profile characteristics:
- Completely independent instance
- Each profile has its own key4.db
- Complete isolation between profiles

**Design proposals under discussion**:
```go
// How to unify the abstraction?
type Profile struct {
    // What fields to include?
    // How to express shared keys vs independent keys?
}
```

### 3.4 Key Management Hierarchy Naming

Key-related concept naming:
```
System Retrieval → ??? → Derivation Processing → ??? → Final Key

Possible naming:
- SystemSecret / RawPassword / KeychainValue
- DerivedKey / MasterKey / ProcessedKey  
- DecryptionKey / FinalKey / ActiveKey
```

### 3.5 Internal Interface Design

Interfaces and method naming to be determined:

```go
// Interface naming and responsibility division
type ??? interface {
    // Browser discovery
}

type ??? interface {
    // Key management
}

type ??? interface {
    // Data extraction
}

type ??? interface {
    // Decryption processing
}
```

## Part 4: Implementation Challenges

### 4.1 Profile Abstraction Unification (Most Difficult)

The fundamental differences between Chrome and Firefox make unified abstraction difficult, affecting:
- Key management strategy
- Data extraction flow
- Result organization

### 4.2 Cross-platform Code Organization

Using build tags to organize platform-specific code:
- How to name platform-specific functions?
- Where to place common code?
- How to handle platform-specific errors?

### 4.3 Error Handling Consistency

Current error handling is inconsistent:
- Chrome decryption failure attempts fallback
- Firefox failure skips directly
- Need unified error strategy

## Next Steps

1. **Priority: Solve Profile abstraction** - Foundation for all other design
2. **Determine type naming scheme** - Browser, Item, and other basic types
3. **Determine data flow naming** - Names for each stage of data
4. **Design internal interfaces** - Based on previous decisions

## References

- RFC-001: HackBrowserData Architecture Refactoring
- Existing codebase: https://github.com/moonD4rk/HackBrowserData