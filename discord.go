package main

import (
	"log"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/pkg/errors"
)

func (fs *Filesystem) UpdateGuilds() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	log.Println("Updating guilds.")

	guilds, err := fs.State.Guilds()
	if err != nil {
		log.Fatalln("Failed to get guilds:", err)
	}

	newGuilds := guilds[:0]

Main:
	for _, guild := range guilds {
		for _, g := range fs.Guilds {
			if g.ID == guild.ID {
				continue Main
			}
		}

		newGuilds = append(newGuilds, guild)
	}

	log.Println("New guilds:", len(newGuilds))

	for _, g := range newGuilds {
		guild := &Guild{
			ID:    g.ID,
			Name:  sanitize(g.Name),
			FS:    fs,
			Inode: NewInode(),
		}

		go func() {
			if err := guild.UpdateChannels(); err != nil {
				log.Println("Failed to update guild "+g.ID.String()+":", err)
			}
			log.Println("Fetched", guild.Name)
		}()

		// Subscribe to guilds
		// fs.State.Gateway.GuildSubscribe(gateway.GuildSubscribeData{
		// 	GuildID:    g.ID,
		// 	Typing:     false,
		// 	Activities: false,
		// })

		fs.Guilds = append(fs.Guilds, guild)
	}

	return nil
}

func (g *Guild) UpdateChannels() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	log.Println("Updating channels.")

	channels, err := g.FS.State.Channels(g.ID)
	if err != nil {
		return errors.Wrap(err, "Failed to get channels")
	}

	newChs := channels[:0]

Main:
	for _, channel := range channels {
		for _, ch := range g.Channels {
			if ch.ID == channel.ID {
				continue Main
			}
		}

		newChs = append(newChs, channel)
	}

	log.Println("New channels:", len(newChs))

	for _, ch := range newChs {
		switch ch.Type {
		case discord.GuildText, discord.GroupDM, discord.DirectMessage:
		default:
			continue
		}

		var name = ch.Name

		if len(ch.DMRecipients) > 0 {
			var names = make([]string, len(ch.DMRecipients))
			for i, u := range ch.DMRecipients {
				names[i] = u.Username
			}

			name = strings.Join(names, " ")
		}

		// Escape
		name = sanitize(name)

		g.Channels = append(g.Channels, &Channel{
			ID:       ch.ID,
			Category: ch.CategoryID,
			Name:     name,
			Position: ch.Position,
			FS:       g.FS,
			Inode:    NewInode(),
		})
	}

	return nil
}

func (ch *Channel) Messages() ([]discord.Message, error) {
	return ch.FS.State.Messages(ch.ID)
}

func sanitize(name string) string {
	return strings.Replace(name, "/", "_", -1)
}
