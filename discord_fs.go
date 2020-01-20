package main

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/state"
	"github.com/pkg/errors"
)

var inodeAtom *uint64 = new(uint64)

func NewInode() uint64 {
	return atomic.AddUint64(inodeAtom, 1)
}

type Filesystem struct {
	CreatedTime time.Time
	Inode       uint64

	// Shared resources
	Fmt *Formatter

	State  *state.State
	Guilds []*Guild

	mu sync.Mutex
}

func NewFS(s *state.State) (*Filesystem, error) {
	fmtter, err := NewFormatter(nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create a formatter")
	}

	fs := &Filesystem{
		CreatedTime: time.Now(),
		Inode:       NewInode(),
		Fmt:         fmtter,
		State:       s,
	}

	if err := fs.UpdateGuilds(); err != nil {
		return nil, errors.Wrap(err, "Failed to update guilds")
	}

	return fs, nil
}

func (fs *Filesystem) Root() (fs.Node, error) {
	return fs, nil
}

var _ fs.Node = (*Filesystem)(nil)

func (fs *Filesystem) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = os.ModeDir | 0664
	attr.Inode = fs.Inode
	// attr.Valid = time.Minute
	return nil
}

var _ fs.HandleReadDirAller = (*Filesystem)(nil)

func (fs *Filesystem) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	// Should be cached for at least a minute, whatever.
	if err := fs.UpdateGuilds(); err != nil {
		return nil, errors.Wrap(err, "Failed to update guilds")
	}

	var res = make([]fuse.Dirent, len(fs.Guilds))
	for i, g := range fs.Guilds {
		res[i].Name = g.ID.String()
		res[i].Type = fuse.DT_Dir
	}

	return res, nil
}

var _ fs.NodeRequestLookuper = (*Filesystem)(nil)

func (fs *Filesystem) Lookup(ctx context.Context,
	req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {

	for _, g := range fs.Guilds {
		if g.ID.String() == req.Name {
			return g, nil
		}
	}

	return nil, fuse.ENOENT
}

// Guild is a directory that contains multiple channel files.
type Guild struct {
	FS    *Filesystem
	Inode uint64

	ID discord.Snowflake

	Channels []*Channel
	mu       sync.Mutex
}

func (g *Guild) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Mode = os.ModeDir | 0664
	attr.Inode = g.Inode
	// attr.Valid = time.Minute
	return nil
}

func (g *Guild) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var res = make([]fuse.Dirent, len(g.Channels))

	// Also cached for a minute, whatever.
	if err := g.UpdateChannels(); err != nil {
		return nil, errors.Wrap(err, "Failed to update channels")
	}

	for i, ch := range g.Channels {
		res[i] = fuse.Dirent{
			Name: ch.ID.String(),
			Type: fuse.DT_File,
		}
	}

	return res, nil
}

func (g *Guild) Lookup(ctx context.Context,
	req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {

	for _, ch := range g.Channels {
		if ch.ID.String() == req.Name {
			return ch, nil
		}
	}

	return nil, fuse.ENOENT
}

// Channel is a message file/inode.
type Channel struct {
	FS    *Filesystem
	Inode uint64

	LastMod time.Time
	LastSz  uint64

	ID       discord.Snowflake
	Category discord.Snowflake
	Position int
}

func (ch *Channel) Attr(ctx context.Context, attr *fuse.Attr) error {
	// Fetch the messages and fill up latest LastSz and LastMod.
	if _, err := ch.render(); err != nil {
		return err
	}

	attr.Valid = 0
	attr.Inode = ch.Inode
	attr.Size = ch.LastSz
	attr.Mode = 0664
	attr.Mtime = ch.LastMod
	attr.Ctime = ch.FS.CreatedTime
	attr.Crtime = ch.FS.CreatedTime
	return nil
}

func (ch *Channel) Open(ctx context.Context,
	req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {

	return ch, nil
}

func (ch *Channel) ReadAll(ctx context.Context) ([]byte, error) {
	return ch.render()
}

func (ch *Channel) render() ([]byte, error) {
	// All praise the state cache!
	msgs, err := ch.Messages()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get messages")
	}

	s, err := ch.FS.Fmt.RenderMessages(msgs)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to render messages")
	}

	ch.LastSz = uint64(len(s))
	ch.LastMod = time.Now()

	return []byte(s), nil
}
