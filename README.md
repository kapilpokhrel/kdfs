# KDFS – KeePass KDBX Filesystem

KDFS mounts a [KeePass](https://keepass.info/) `.kdbx` password database as a **fuse filesystem** 
allowing you to browse and edit entries as if they were regular directories and text files.

---

### Examples
- Copying a password to clipboard
```
cat example/General/"Sample Entry.entry"/password | wl-copy

```

- Adding a new entry
```
mkdir newsite.entry
echo -n "alice" > newsite.entry/username
echo -n "s3cret"  > newsite.entry/password
```

## Getting Started

### Prerequisites
- FUSE support on your system (`libfuse` on Linux, `osxfuse/macfuse` on macOS)

### Build

```bash
go build -o kdfs ./cmd/main
```

### Run

```bash
./kdfs /mount/point /path/to/database.kdbx
```

- The program will prompt for the database password (or you can wire it via env/config).
- The mountpoint must exist and be empty.
- The filesystem will stay mounted until unmounted as shwon below or interrupted (`Ctrl+C`) in `-daemon=false` mode
```
fusermount3 -u /mount/point
# or
umount /mount/point
```

### Filesystem Layout Overview
Suppose we have a KDBX file with the following hierarchy
```
.
└── example
    ├── General [Group]
    │   └── Sample Entry [Entry]
    └── Recycle Bin [Group]

```

When mounted, KDFS exposes the following filesystem structure:
```

examplefs
├── example
│   ├── General
│   │   └── Sample Entry.entry
│   │       ├── notes
│   │       ├── password
│   │       ├── url
│   │       └── username
│   └── Recycle Bin
├── lockaction
└── lockstate
```

- At the root directory (`/examplefs`), new groups can be created, but new entries cannot.
- Root directory contains two special files; `lockaction` and `lockstate`
    - `lockstate` is a **read only** file that reports the current protection state of underlying database
        - `t` -> locked
        - `f` -> unlocked
    - `lockaction` is a **write only** file that changed the used to lock/unlock a database.
        - `echo -n "lock" >> lockaction` -> locks the database
        - `echo -n "<password>" >> lockaction` -> unlockes the locked database
- Each entry’s fields are exposed as individual files. This makes it easy to read or modify fields using standard shell tools. For example:
    - `cat username | wl-copy` allows easy copy of the username
- Entry directories only support the following field files: `username`, `password`, `url`, and `notes`
- Entry directories use the suffix `.entry` to distinguish them from group directories.
- Making a directory with suffix `.entry` (eg `mkdir test.entry`) will create a entry otherwise a group will be created

### Note
It is still in development so it is recommended to work with a copy of your kdbx file.

### Limitations
- No recycle bin support. Deletion is permanent. Items are not moved to the KeePass Recycle Bin.
- No binary fields
- No attachments
- No keyfile support
