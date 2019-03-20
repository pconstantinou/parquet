package main

var newFieldTpl = `{{define "newField"}}New{{.FieldType}}(func(x {{.Type}}) {{.TypeName}} { return x.{{.FieldName}} }, func(x *{{.Type}}, v {{.TypeName}}) { x.{{.FieldName}} = v }, "{{.ColumnName}}", {{compressionFunc .}}(compression)...),{{end}}`

var tpl = `package {{.Package}}

// This code is generated by github.com/parsyl/parquet.

import (
	"fmt"
	"io"
	"bytes"
	"encoding/binary"

	"github.com/parsyl/parquet"
	{{.Import}}
	{{range imports .Fields}}{{.}}
	{{end}}
)

type compression int

const (
	compressionUncompressed             = 0
	compressionSnappy                   = 1
	compressionUnknown      compression = -1
)

// ParquetWriter reprents a row group
type ParquetWriter struct {
	fields []Field

	len int

	// child points to the next page
	child *ParquetWriter

	// max is the number of Record items that can get written before
	// a new set of column chunks is written
	max int

	meta *parquet.Metadata
	w    io.Writer
	compression compression
}

func Fields(compression compression) []Field {
	return []Field{ {{range .Fields}}
		{{template "newField" .}}{{end}}
	}
}

func fieldCompression(c compression) []func(*parquet.RequiredField) {
	switch c {
	case compressionUncompressed:
		return []func(*parquet.RequiredField){parquet.RequiredFieldUncompressed}
	case compressionSnappy:
		return []func(*parquet.RequiredField){parquet.RequiredFieldSnappy}
	default:
		return []func(*parquet.RequiredField){}
	}
}

func optionalFieldCompression(c compression) []func(*parquet.OptionalField) {
	switch c {
	case compressionUncompressed:
		return []func(*parquet.OptionalField){parquet.OptionalFieldUncompressed}
	case compressionSnappy:
		return []func(*parquet.OptionalField){parquet.OptionalFieldSnappy}
	default:
		return []func(*parquet.OptionalField){}
	}
}

func NewParquetWriter(w io.Writer, opts ...func(*ParquetWriter) error) (*ParquetWriter, error) {
	return newParquetWriter(w, append(opts, begin)...)
}

func newParquetWriter(w io.Writer, opts ...func(*ParquetWriter) error) (*ParquetWriter, error) {
	p := &ParquetWriter{
		max:         1000,
		w:           w,
		compression: compressionSnappy,
	}

	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}

	p.fields = Fields(p.compression)
	if p.meta == nil {
		ff := Fields(p.compression)
		schema := make([]parquet.Field, len(ff))
		for i, f := range ff {
			schema[i] = f.Schema()
		}
		p.meta = parquet.New(schema...)
	}

	return p, nil
}

// MaxPageSize is the maximum number of rows in each row groups' page.
func MaxPageSize(m int) func(*ParquetWriter) error {
	return func(p *ParquetWriter) error {
		p.max = m
		return nil
	}
}

func begin(p *ParquetWriter) error {
	_, err := p.w.Write([]byte("PAR1"))
	return err
}

func withMeta(m *parquet.Metadata) func(*ParquetWriter) error {
	return func(p *ParquetWriter) error {
		p.meta = m
		return nil
	}
}

func Uncompressed(p *ParquetWriter) error {
	p.compression = compressionUncompressed
	return nil
}

func Snappy(p *ParquetWriter) error {
	p.compression = compressionSnappy
	return nil
}

func withCompression(c compression) func(*ParquetWriter) error {
	return func(p *ParquetWriter) error {
		p.compression = c
		return nil
	}
}

func (p *ParquetWriter) Write() error {
	for i, f := range p.fields {
		if err := f.Write(p.w, p.meta); err != nil {
			return err
		}

		for child := p.child; child != nil; child = child.child {
			if err := child.fields[i].Write(p.w, p.meta); err != nil {
				return err
			}
		}
	}

	p.fields = Fields(p.compression)
	p.child = nil
	p.len = 0

	schema := make([]parquet.Field, len(p.fields))
	for i, f := range p.fields {
		schema[i] = f.Schema()
	}
	p.meta.StartRowGroup(schema...)
	return nil
}

func (p *ParquetWriter) Close() error {
	if err := p.meta.Footer(p.w); err != nil {
		return err
	}

	_, err := p.w.Write([]byte("PAR1"))
	return err
}

func (p *ParquetWriter) Add(rec {{.Type}}) {
	if p.len == p.max {
		if p.child == nil {
			// an error can't happen here
			p.child, _ = newParquetWriter(p.w, MaxPageSize(p.max), withMeta(p.meta), withCompression(p.compression))
		}

		p.child.Add(rec)
		return
	}

	for _, f := range p.fields {
		f.Add(rec)
	}

	p.len++
}

type Field interface {
	Add(r {{.Type}})
	Write(w io.Writer, meta *parquet.Metadata) error
	Schema() parquet.Field
	Scan(r *{{.Type}})
	Read(r io.ReadSeeker, meta *parquet.Metadata, pg parquet.Page) error
	Name() string
}

func getFields(ff []Field) map[string]Field {
	m := make(map[string]Field, len(ff))
	for _, f := range ff {
		m[f.Name()] = f
	}
	return m
}

func NewParquetReader(r io.ReadSeeker, opts ...func(*ParquetReader)) (*ParquetReader, error) {
	ff := Fields(compressionUnknown)
	pr := &ParquetReader{
		r: r,
	}

	for _, opt := range opts {
		opt(pr)
	}

	schema := make([]parquet.Field, len(ff))
	for i, f := range ff {
		schema[i] = f.Schema()
	}

	meta := parquet.New(schema...)
	if err := meta.ReadFooter(r); err != nil {
		return nil, err
	}
	pr.rows = meta.Rows()
	var err error
	pr.pages, err = meta.Pages()
	if err != nil {
		return nil, err
	}

	pr.rowGroups = meta.RowGroups()
	_, err = r.Seek(4, io.SeekStart)
	if err != nil {
		return nil, err
	}
	pr.meta = meta

	return pr, pr.readRowGroup()
}

func readerIndex(i int) func(*ParquetReader) {
	return func(p *ParquetReader) {
		p.index = i
	}
}

// ParquetReader reads one page from a row group.
type ParquetReader struct {
	fields         map[string]Field
	index          int
	cursor         int64
	rows           int64
	rowGroupCursor int64
	rowGroupCount  int64
	pages        map[string][]parquet.Page
	meta           *parquet.Metadata
	err            error

	r         io.ReadSeeker
	rowGroups []parquet.RowGroup
}

func (p *ParquetReader) Error() error {
	return p.err
}

func (p *ParquetReader) readRowGroup() error {
	p.rowGroupCursor = 0

	if len(p.rowGroups) == 0 {
		p.rowGroupCount = 0
		return nil
	}

	rg := p.rowGroups[0]
	p.fields = getFields(Fields(compressionUnknown))
	p.rowGroupCount = rg.Rows
	p.rowGroupCursor = 0
	for _, col := range rg.Columns() {
		name := col.MetaData.PathInSchema[len(col.MetaData.PathInSchema)-1]
		f, ok := p.fields[name]
		if !ok {
			return fmt.Errorf("unknown field: %s", name)
		}
		pages := p.pages[f.Name()]
		if len(pages) <= p.index {
			break
		}

		pg := pages[0]
		if err := f.Read(p.r, p.meta, pg); err != nil {
			return fmt.Errorf("unable to read field %s, err: %s", f.Name(), err)
		}
		p.pages[f.Name()] = p.pages[f.Name()][1:]
	}
	p.rowGroups = p.rowGroups[1:]
	return nil
}

func (p *ParquetReader) Rows() int64 {
	return p.rows
}

func (p *ParquetReader) Next() bool {
	if p.err == nil && p.cursor >= p.rows {
		return false
	}
	if p.rowGroupCursor >= p.rowGroupCount {
		p.err = p.readRowGroup()
		if p.err != nil {
			return false
		}
	}

	p.cursor++
	p.rowGroupCursor++
	return true
}

func (p *ParquetReader) Scan(x *{{.Type}}) {
	if p.err != nil {
		return
	}

	for _, f := range p.fields {
		f.Scan(x)
	}
}

{{range dedupe .Fields}}
{{if eq .Category "numeric"}}
{{ template "requiredField" .}}
{{end}}
{{if eq .Category "numericOptional"}}
{{ template "optionalField" .}}
{{end}}
{{if eq .Category "string"}}
{{ template "stringField" .}}
{{end}}
{{if eq .Category "stringOptional"}}
{{ template "stringOptionalField" .}}
{{end}}
{{if eq .Category "bool"}}
{{ template "boolField" .}}
{{end}}
{{if eq .Category "boolOptional"}}
{{ template "boolOptionalField" .}}
{{end}}
{{end}}

{{range dedupe .Fields}}
{{if eq .Category "numeric"}}
{{ template "requiredStats" .}}
{{end}}
{{if eq .Category "numericOptional"}}
{{ template "optionalStats" .}}
{{end}}
{{if eq .Category "string"}}
{{ template "stringStats" .}}
{{end}}
{{if eq .Category "stringOptional"}}
{{ template "stringOptionalStats" .}}
{{end}}
{{if eq .Category "bool"}}
{{ template "boolStats" .}}
{{end}}
{{if eq .Category "boolOptional"}}
{{ template "boolOptionalStats" .}}
{{end}}
{{end}}
`
