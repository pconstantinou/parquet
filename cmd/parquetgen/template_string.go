package main

var stringTpl = `{{define "stringField"}}
type StringField struct {
	parquet.RequiredField
	vals []string
	val  func(r {{.Type}}) string
	read func(r *{{.Type}}, v string)
}

func NewStringField(val func(r {{.Type}}) string, read func(r *{{.Type}}, v string), col string, opts ...func(*parquet.RequiredField)) *StringField {
	return &StringField{
		val:           val,
		read:          read,
		RequiredField: parquet.NewRequiredField(col, opts...),
	}
}

func (f *StringField) Schema() parquet.Field {
	return parquet.Field{Name: f.Name(), Type: parquet.StringType, RepetitionType: parquet.RepetitionRequired}
}

func (f *StringField) Scan(r *{{.Type}}) {
	if len(f.vals) == 0 {
		return
	}

	v := f.vals[0]
	f.vals = f.vals[1:]
	f.read(r, v)
}

func (f *StringField) Add(r {{.Type}}) {
	f.vals = append(f.vals, f.val(r))
}

func (f *StringField) Write(w io.Writer, meta *parquet.Metadata) error {
	buf := bytes.Buffer{}

	for _, s := range f.vals {
		if err := binary.Write(&buf, binary.LittleEndian, int32(len(s))); err != nil {
			return err
		}
		buf.Write([]byte(s))
	}

	return f.DoWrite(w, meta, buf.Bytes(), len(f.vals))
}

func (f *StringField) Read(r io.ReadSeeker, meta *parquet.Metadata, pg parquet.Page) error {
	rr, _, err := f.DoRead(r, meta, pg)
	if err != nil {
		return err
	}

	for j := 0; j < pg.N; j++ {
		var x int32
		if err := binary.Read(rr, binary.LittleEndian, &x); err != nil {
			return err
		}
		s := make([]byte, x)
		if _, err := rr.Read(s); err != nil {
			return err
		}

		f.vals = append(f.vals, string(s))
	}
	return nil
}
{{end}}`
