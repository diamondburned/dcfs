package main

import (
	"bytes"
	"encoding/csv"
)

type CSV struct {
	buf *bytes.Buffer
	csv *csv.Writer
}

func NewCSVCreator(delimiter rune) func() interface{} {
	return func() interface{} {
		buf := new(bytes.Buffer)
		csv := csv.NewWriter(buf)
		csv.Comma = delimiter

		return &CSV{
			buf: buf,
			csv: csv,
		}
	}
}

func (csv *CSV) Render(cols []string) (string, error) {
	csv.buf.Reset()
	csv.csv.Write(cols)
	csv.csv.Flush()
	return csv.buf.String(), csv.csv.Error()
}

func NewBufferCreator() func() interface{} {
	return func() interface{} {
		return new(bytes.Buffer)
	}
}
