package main

import (
	"bytes"
	"log"
	"strconv"
	"strings"
	"sync"
	"text/template"

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
}

var (
	// TODO functions

	DefaultMessageTemplate = []string{
		"{{.Author.Username}}", "{{.Content}}",
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
	}

	if err := fmtter.ChangeMessageTemplate(DefaultMessageTemplate); err != nil {
		return nil, errors.Wrap(err, "Failed to create the message template")
	}

	return fmtter, nil
}

func (f *Formatter) ChangeMessageTemplate(fmts []string) error {
	var tmpls = make([]*template.Template, len(fmts))

	for i, fmt := range fmts {
		d := strconv.Itoa(i)

		t, err := template.New("message_" + d).Parse(fmt)
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
		if err := tmpl.Execute(buf, msg); err != nil {
			return "", errors.Wrap(err, "Failed to execute template on msg")
		}

		cols[i] = newliner.Replace(buf.String())
		buf.Reset()
	}

	csv := f.CSVPool.Get().(*CSV)
	defer f.CSVPool.Put(csv)

	return csv.Render(cols)
}

func (f *Formatter) RenderMessages(msgs []discord.Message) (string, error) {
	buf := f.BufPool.Get().(*bytes.Buffer)
	defer f.BufPool.Put(buf)

	buf.Reset()

	for _, msg := range msgs {
		m, err := f.RenderMessage(msg)
		if err != nil {
			return "", errors.Wrap(err,
				"Failed to format message "+msg.ID.String())
		}

		buf.WriteString(m)
	}

	return buf.String(), nil
}
