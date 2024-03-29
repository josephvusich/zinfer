# zinfer: ZFS Infer

Over time, a pool or dataset's properties may change from those originally passed to `zpool create` or `zfs create`. Rather than relying on `zpool history`, `zinfer` parses the properties of imported pools and their datasets and infers the minimal `zpool create` or `zfs create` command(s) necessary to duplicate the current configuration.

## Installation

* `go install github.com/josephvusich/zinfer@latest`

## Usage
```
usage: zinfer [--minimal-features] [--recursive] [dataset ...]
      --help              show this help message
      --minimal-features  omit enabled pool features that are not currently active
  -R, --recursive         recursively include descendant datasets of the specified parents
```
