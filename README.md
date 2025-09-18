# KDFS – KeePass KDBX Filesystem

KDFS mounts a [KeePass](https://keepass.info/) `.kdbx` password database as a **read-only filesystem**.  
It lets you explore entries inside your database as if they were regular files and directories.

⚠️ Currently, only **read-only mode** is implemented.

---

## Getting Started

### Prerequisites
- FUSE support on your system (`libfuse` on Linux, `osxfuse/macfuse` on macOS)

### Build

```bash
go build -o kdfs ./cmd/main
```

### Run

```bash
./kdfs /path/to/database.kdbx /mount/point
```

- The program will prompt for the database password (or you can wire it via env/config).
- The mountpoint must exist and be empty.
- The filesystem will stay mounted until interrupted (`Ctrl+C`) (With `-daemon=false`; default)
