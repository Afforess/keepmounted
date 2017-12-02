# keepmounted
A simple daemon that keeps a mount point mounted (checks by attempting to write a file to it, and if it fails, unmounts and remounds)

## Notes
Must be run as root

## Build
Built with golang 1.9.2 (may work with older versions, ymmv)

`go get -u github.com/Afforess/keepmounted`

## Usage
```./keepmounted -help
Usage of ./keepmounted:
  -interval int
        how often the mount is checked (in seconds) (default 60)
  -options string
        mount options
  -source string
        the source device
  -target string
        path to the target mount location
  -type string
        mount type
```
