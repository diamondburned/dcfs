# dcfs

FUSE Discord filesystem, made with [Arikawa](https://github.com/diamondburned/arikawa).

## Why?

AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA.

## How?

```sh
mkdir /tmp/dcfs

# Only run this if first timer
# TODO: auto unmount
fusermount -u /tmp/dcfs

export TOKEN="your token here"
go run . /tmp/dcfs
```
