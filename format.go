package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/state"
	"github.com/pkg/errors"
)

type Formatter struct {
	State *state.State

	CSVPool sync.Pool
	BufPool sync.Pool

	messageTemplate  []string
	messageTemplater []*template.Template
}

type FormatterOpts struct {
	Delimiter rune
	State     *state.State
}

var (
	// TODO functions

	DefaultMessageTemplate = []string{
		"{{nickname .}}", "{{color .}}",
		`{{time .Timestamp "3:04PM"}}`, "{{.Content}}", "{{json .Embeds}}",
	}
	DefaultFormatterOpts = FormatterOpts{
		Delimiter: ',',
	}
)

func MustFormatter(opts *FormatterOpts) *Formatter {
	f, err := NewFormatter(opts)
	if err != nil {
		log.Fatalln(err)
	}

	return f
}

func NewFormatter(opts *FormatterOpts) (*Formatter, error) {
	if opts == nil {
		opts = &DefaultFormatterOpts
	}

	fmtter := &Formatter{
		CSVPool: sync.Pool{
			New: NewCSVCreator(opts.Delimiter),
		},
		BufPool: sync.Pool{
			New: NewBufferCreator(),
		},
		State: opts.State,
	}

	if err := fmtter.ChangeMessageTemplate(DefaultMessageTemplate); err != nil {
		panic("BUG on ChangeMessageTemplate:" + err.Error())
	}

	return fmtter, nil
}

func (f *Formatter) ChangeMessageTemplate(fmts []string) error {
	var tmpls = make([]*template.Template, len(fmts))

	for i, fmt := range fmts {
		d := strconv.Itoa(i)

		t, err := template.
			New("message_" + d).
			Funcs(f.funcMap()).
			Parse(fmt)
		if err != nil {
			return errors.Wrap(err, "Failed to parse arg "+d)
		}

		tmpls[i] = t
	}

	f.messageTemplate = fmts
	f.messageTemplater = tmpls
	return nil
}

var newliner = strings.NewReplacer(
	"\n", `\n`,
)

func (f *Formatter) RenderMessage(msg discord.Message) (string, error) {
	var cols = make([]string, len(f.messageTemplater))

	buf := f.BufPool.Get().(*bytes.Buffer)
	defer f.BufPool.Put(buf)

	for i, tmpl := range f.messageTemplater {
		buf.Reset()

		if err := tmpl.Execute(buf, msg); err != nil {
			return "", errors.Wrap(err, "Failed to execute template on msg")
		}

		cols[i] = newliner.Replace(buf.String())
	}

	csv := f.CSVPool.Get().(*CSV)
	defer f.CSVPool.Put(csv)

	return csv.Render(cols)
}

func (f *Formatter) RenderMessages(msgs []discord.Message) (string, error) {
	buf := f.BufPool.Get().(*bytes.Buffer)
	defer f.BufPool.Put(buf)

	buf.Reset()

	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]

		m, err := f.RenderMessage(msg)
		if err != nil {
			return "", errors.Wrap(err,
				"Failed to format message "+msg.ID.String())
		}

		buf.WriteString(m)
	}

	return buf.String(), nil
}

func (f *Formatter) funcMap() template.FuncMap {
	return map[string]interface{}{
		"nickname": func(m discord.Message) string {
			if !m.GuildID.Valid() {
				return m.Author.Username
			}

			member, err := f.State.Member(m.GuildID, m.Author.ID)
			if err != nil {
				return m.Author.Username
			}

			if member.Nick == "" {
				return m.Author.Username
			}

			return member.Nick
		},
		"color": func(m discord.Message) string {
			if !m.GuildID.Valid() {
				return m.Author.Username
			}

			member, err := f.State.Member(m.GuildID, m.Author.ID)
			if err != nil {
				return m.Author.Username
			}

			guild, err := f.State.Guild(m.GuildID)
			if err != nil {
				return m.Author.Username
			}

			return fmt.Sprintf("#%06X", discord.MemberColor(*guild, *member))
		},
		"time": func(ts discord.Timestamp, fmt string) string {
			return time.Time(ts).Format(fmt)
		},
		"content": func(m discord.Message) string {
			b := strings.Builder{}
			b.WriteString(m.Content)

			for _, a := range m.Attachments {
				b.WriteString(" " + a.URL)
			}

			return b.String()
		},
		"json": func(v interface{}) string {
			b, err := json.Marshal(v)
			if err != nil {
				log.Println("JSON error:", err)
				return "ERR"
			}

			return string(b)
		},
	}
}
