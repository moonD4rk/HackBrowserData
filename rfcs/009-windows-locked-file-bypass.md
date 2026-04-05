# RFC-009: Windows Locked File Bypass

**Author**: moonD4rk
**Status**: Living Document
**Created**: 2026-04-05

## 1. Problem

Chromium on Windows sets `PRAGMA locking_mode=EXCLUSIVE` on certain SQLite databases, most notably the `Cookies` file (`Network/Cookies`). This means Chrome's process holds the file open with `dwShareMode=0` (no sharing), preventing any other process from opening it — even for reading.

```
Chrome.exe
  → CreateFileW("Network/Cookies", ..., dwShareMode=0)  // exclusive lock
  → PRAGMA locking_mode=EXCLUSIVE

hack-browser-data.exe
  → os.ReadFile("Network/Cookies")
  → ERROR: access denied
```

This is a **Windows-specific problem**. On macOS and Linux, SQLite uses `fcntl`/`flock` advisory locks, which do not prevent other processes from reading the file. The standard copy path works fine on those platforms.

## 2. Solution Overview

Bypass the exclusive lock using Windows kernel APIs: enumerate system handles to find Chrome's file handle, duplicate it into our process, then read the file contents via memory-mapped I/O. **No admin privileges required.**

```
NtQuerySystemInformation → find Chrome's handle to Cookies file
  → DuplicateHandle into our process
  → CreateFileMappingW + MapViewOfFile (read from kernel cache)
  → write bytes to temp destination
```

## 3. Step-by-Step

### 3.1 Enumerate System Handles

Call `NtQuerySystemInformation` with `SystemExtendedHandleInformation` (class 64) to get every open handle in the system. The "extended" variant uses `ULONG_PTR` for PIDs and handle values, avoiding truncation on 64-bit Windows.

The query starts with a 4 MB buffer and doubles it (up to 256 MB) if the API returns `STATUS_INFO_LENGTH_MISMATCH`.

Each entry in the result table:

| Field | Size | Description |
|-------|------|-------------|
| UniqueProcessID | `uintptr` | Owning process PID |
| HandleValue | `uintptr` | Handle value in the owning process |
| GrantedAccess | `uint32` | Access mask |
| ObjectTypeIndex | `uint16` | Kernel object type |

### 3.2 Find the Target Handle

For each handle entry:

1. `OpenProcess(PROCESS_DUP_HANDLE, pid)` — open the owning process
2. `DuplicateHandle` — duplicate the handle into our process with `DUPLICATE_SAME_ACCESS`
3. `GetFileType` — verify it is `FILE_TYPE_DISK` (skip pipes, sockets, etc.)
4. `GetFinalPathNameByHandleW` — get the full file path

### 3.3 Path Matching with Short-Name Tolerance

Windows 8.3 short path names (e.g. `RUNNER~1` vs `runneradmin`) cause direct path comparison to fail. The solution extracts a **stable suffix** by stripping everything before `AppData\Local\` or `AppData\Roaming\` and comparing in lowercase:

```
Input:  C:\Users\RUNNER~1\AppData\Local\Google\Chrome\...\Network\Cookies
Suffix: google\chrome\...\network\cookies

Input:  C:\Users\runneradmin\AppData\Local\Google\Chrome\...\Network\Cookies
Suffix: google\chrome\...\network\cookies

→ match!
```

### 3.4 Read via Memory-Mapped I/O

Once we have a duplicated handle to the locked file:

```
| DuplicateHandle (read access)                   |
|-------------------------------------------------|
               ↓
| CreateFileMappingW(handle, PAGE_READONLY)       |
|-------------------------------------------------|
               ↓
| MapViewOfFile(mapping, FILE_MAP_READ, fileSize) |
|-------------------------------------------------|
               ↓
| byte slice from kernel file cache               |
| (includes uncommitted WAL data from Chrome)     |
|-------------------------------------------------|
               ↓
| os.WriteFile(destination, bytes, 0600)          |
|-------------------------------------------------|
```

Memory-mapped I/O reads from the OS kernel's **file cache**, which includes data Chrome has written but not yet checkpointed to disk. This produces a more complete snapshot than a raw `ReadFile`.

**Fallback**: if `CreateFileMappingW` fails (e.g., the file is empty or zero-length), falls back to `Seek(0)` + `ReadFile` on the duplicated handle.

## 4. Why This Works

The key insight is that `dwShareMode=0` only prevents **new** `CreateFileW` calls from opening the file. It does **not** prevent:

- `DuplicateHandle` — which creates a copy of an existing handle (Chrome's own handle)
- `CreateFileMappingW` — which operates on a handle we already own
- `MapViewOfFile` — which reads from the kernel's page cache

This is a documented Windows behavior, not an exploit. The technique requires only standard user privileges because `PROCESS_DUP_HANDLE` access is available for processes owned by the same user.

## 5. Limitations

- **Performance**: enumerating all system handles is expensive (the system may have 100,000+ handles). The entire table must be scanned to find the target file.
- **Race condition**: Chrome could close and reopen the file between enumeration and duplication, though this is unlikely for long-lived database files.
- **Not needed on macOS/Linux**: advisory locking on these platforms does not prevent reading, so the standard `copyFile` path is always sufficient.

## Related RFCs

| RFC | Topic |
|-----|-------|
| [RFC-008](008-file-acquisition-and-platform-quirks.md) | File acquisition lifecycle and session management |
