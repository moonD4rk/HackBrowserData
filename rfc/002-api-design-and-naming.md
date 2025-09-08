# RFC-002: HackBrowserData API Design and Naming Conventions

**Author**: moonD4rk  
**Status**: In Discussion  
**Created**: 2025-09-07  
**Updated**: 2025-09-08  

## Abstract

This RFC documents discussions regarding API design, naming conventions, and architectural decisions during the HackBrowserData project refactoring. The document includes existing code analysis, confirmed architectural designs, and remaining issues under discussion.

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

## Part 3: Confirmed Architectural Design

### 3.1 Browser-Profile Relationship Model

After extensive discussion, the following relationship model has been established:

#### Core Relationship
- **Browser is the main entity**: Manages key strategy and profile discovery
- **Profile is subordinate**: Only responsible for data location, not key management
- **Key Management**: Handled at Browser level for Chromium, Profile level for Firefox

#### Layered Architecture
```
Scanner Layer (CLI only)
    ↓
Browser Layer (Key Management)
    ↓
Profile Layer (Data Location)
    ↓
Data Layer (Actual Files)
```

### 3.2 Usage Modes: API vs CLI

The system supports two distinct usage modes with different entry points:

#### API Mode (Programmatic Usage)
```go
// User specifies browser directly
client := hbd.New(
    hbd.WithBrowser(hbd.Chrome),
    hbd.WithItems(hbd.Password, hbd.Cookie),
)
data, err := client.Extract()
```

**Flow**:
1. Create specified Browser instance directly
2. Browser initializes (gets MasterKey for Chromium)
3. Browser scans its own Profiles
4. Process each Profile with cached key

#### CLI Mode (System Scanning)
```bash
# Automatically discover all browsers
./hackbrowserdata --all
```

**Flow**:
1. BrowserScanner scans system
2. Check predefined paths for each browser type
3. Create Browser instances for found browsers
4. Each Browser processes its Profiles

#### Scanner Layer (CLI Only)
```go
type BrowserScanner struct {
    // Scanning strategy
}

func (s *BrowserScanner) ScanSystem() ([]Browser, error) {
    var browsers []Browser
    
    // Check all known browser paths
    for browserType, defaultPath := range DefaultPaths {
        if pathExists(defaultPath) {
            browser := createBrowser(browserType, defaultPath)
            browsers = append(browsers, browser)
        }
    }
    
    return browsers, nil
}
```

### 3.3 Platform-Specific Path Handling

Using Go build tags to handle platform differences:

```go
// browser_paths_darwin.go
// +build darwin

var DefaultPaths = map[BrowserType]string{
    Chrome:  "~/Library/Application Support/Google/Chrome",
    Firefox: "~/Library/Application Support/Firefox/Profiles",
    Edge:    "~/Library/Application Support/Microsoft Edge",
}
```

```go
// browser_paths_windows.go
// +build windows

var DefaultPaths = map[BrowserType]string{
    Chrome:  "%LOCALAPPDATA%/Google/Chrome/User Data",
    Firefox: "%APPDATA%/Mozilla/Firefox/Profiles",
    Edge:    "%LOCALAPPDATA%/Microsoft/Edge/User Data",
}
```

```go
// browser_paths_linux.go
// +build linux

var DefaultPaths = map[BrowserType]string{
    Chrome:  "~/.config/google-chrome",
    Firefox: "~/.mozilla/firefox",
    Edge:    "~/.config/microsoft-edge",
}
```

### 3.4 Key Management Architecture

#### Chromium Key Management Flow

```
Browser Initialization:
├── Create ChromiumKeyManager
├── Read Local State file
├── Extract encrypted_key field
├── Platform-specific decryption:
│   ├── Windows: CryptUnprotectData()
│   ├── macOS: security command + PBKDF2(password, "saltysalt", 1003)
│   └── Linux: SecretService + PBKDF2(password, "saltysalt", 1)
└── Cache MasterKey (16 bytes)

Profile Processing:
├── Scan all Profile directories
├── For each Profile:
│   ├── Read data files (Cookies, Login Data)
│   └── Use cached MasterKey for decryption
└── Return results
```

**Key Points**:
- MasterKey obtained once at Browser level
- All Profiles share the same MasterKey
- Key is cached for entire extraction process

#### Firefox Key Management Flow

```
Browser Initialization:
├── Create FirefoxBrowser
└── No key retrieval (delayed to Profile processing)

Profile Processing:
├── Scan all Profile directories
├── For each Profile:
│   ├── Create Profile-specific FirefoxKeyManager
│   ├── Read key4.db
│   ├── Query: SELECT item1, item2 FROM metadata WHERE id = 'password'
│   ├── NSS decryption to get MasterKey
│   ├── Read data files (logins.json, cookies.sqlite)
│   ├── Use Profile's MasterKey for decryption
│   └── Clean up Profile's key
└── Return results
```

**Key Points**:
- Each Profile has independent key4.db
- MasterKey obtained per Profile
- Keys are Profile-specific and isolated

### 3.5 Interface Design

Based on the architectural decisions, the following interfaces have been designed:

```go
// Browser is the main entity
type Browser interface {
    Type() BrowserType                       // Chrome, Firefox, Edge
    Name() string                            // "Google Chrome", "Firefox"
    RootPath() string                        // Browser data root directory
    Profiles() ([]Profile, error)           // Get all profiles
    ProcessProfile(p Profile) (*Data, error) // Process single profile
}

// Profile only handles data location
type Profile interface {
    Name() string                    // "Default", "Profile 1"
    Path() string                    // Full profile path
    DataFiles() map[Item]string     // Data file mapping
}

// KeyManager handles platform-specific key operations
type KeyManager interface {
    FetchRawKey() ([]byte, error)      // Get encrypted key
    DecryptKey(raw []byte) ([]byte, error) // Decrypt to MasterKey
    GetMasterKey() ([]byte, error)     // Combined operation
}

// BrowserScanner for CLI mode
type BrowserScanner interface {
    ScanSystem() ([]Browser, error)
    FindByType(t BrowserType) ([]Browser, error)
    IdentifyBrowser(path string) (BrowserType, error)
}
```

### 3.6 Implementation Examples

#### Chromium Implementation
```go
type ChromiumBrowser struct {
    browserType BrowserType
    rootPath    string
    keyManager  *ChromiumKeyManager  // Browser-level key management
}

func (b *ChromiumBrowser) ProcessProfile(profile Profile) (*ProfileData, error) {
    // 1. Get shared MasterKey (cached)
    masterKey, err := b.keyManager.GetMasterKey()
    
    // 2. Read Profile data files
    files := profile.DataFiles()
    
    // 3. Decrypt using MasterKey
    data := DecryptData(files, masterKey)
    
    return data, nil
}
```

#### Firefox Implementation
```go
type FirefoxBrowser struct {
    browserType BrowserType
    rootPath    string
    // No Browser-level key management
}

func (b *FirefoxBrowser) ProcessProfile(profile Profile) (*ProfileData, error) {
    // 1. Create Profile-specific key manager
    keyManager := NewFirefoxKeyManager(profile.Path())
    
    // 2. Get Profile's MasterKey
    masterKey, err := keyManager.GetMasterKey()
    
    // 3. Read Profile data files
    files := profile.DataFiles()
    
    // 4. Decrypt using MasterKey
    data := DecryptData(files, masterKey)
    
    return data, nil
}
```

## Part 4: Issues Still Under Discussion

### 4.1 Type Naming (Core Issue)

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

### 4.2 Data Flow Stage Naming

Naming for data at different stages is undecided:

```
File → ??? → ??? → ??? → User Result

Possible naming chains:
1. FileData → EncryptedData → DecryptedData → ParsedData → Result
2. RawData → SecureData → PlainData → StructuredData → Output
3. Source → Encrypted → Plain → Structured → Result
```

### 4.3 Key Management Hierarchy Naming

Key-related concept naming:
```
System Retrieval → ??? → Derivation Processing → ??? → Final Key

Possible naming:
- SystemSecret / RawPassword / KeychainValue
- DerivedKey / MasterKey / ProcessedKey  
- DecryptionKey / FinalKey / ActiveKey
```

## Part 5: Implementation Challenges

### 5.1 Cross-platform Code Organization

Using build tags to organize platform-specific code:
- How to name platform-specific functions?
- Where to place common code?
- How to handle platform-specific errors?

### 5.2 Error Handling Consistency

Current error handling is inconsistent:
- Chrome decryption failure attempts fallback
- Firefox failure skips directly
- Need unified error strategy

## Next Steps

1. **Determine type naming scheme** - Browser, Item, and other basic types
2. **Determine data flow naming** - Names for each stage of data
3. **Refine interface details** - Based on confirmed architecture
4. **Begin implementation** - Start with core interfaces and Browser abstraction

## References

- RFC-001: HackBrowserData Architecture Refactoring
- Existing codebase: https://github.com/moonD4rk/HackBrowserData