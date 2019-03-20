package main

var boolTpl = `{{define "boolField"}}type BoolField struct {
	parquet.RequiredField
	vals []bool
	val  func(r {{.Type}}) bool
	read func(r *{{.Type}}, v bool)
}

func NewBoolField(val func(r {{.Type}}) bool, read func(r *{{.Type}}, v bool), col string, opts ...func(*parquet.RequiredField)) *BoolField {
	return &BoolField{
		val:           val,
		read:          read,
		RequiredField: parquet.NewRequiredField(col, opts...),
	}
}

func (f *BoolField) Schema() parquet.Field {
	return parquet.Field{Name: f.Name(), Type: parquet.BoolType, RepetitionType: parquet.RepetitionRequired}
}

func (f *BoolField) Scan(r *{{.Type}}) {
	if len(f.vals) == 0 {
		return
	}

	v := f.vals[0]
	f.vals = f.vals[1:]
	f.read(r, v)
}

func (f *BoolField) Add(r {{.Type}}) {
	f.vals = append(f.vals, f.val(r))
}

func (f *BoolField) Write(w io.Writer, meta *parquet.Metadata) error {
	ln := len(f.vals)
	byteNum := (ln + 7) / 8
	rawBuf := make([]byte, byteNum)

	for i := 0; i < ln; i++ {
		if f.vals[i] {
			rawBuf[i/8] = rawBuf[i/8] | (1 << uint32(i%8))
		}
	}

	return f.DoWrite(w, meta, rawBuf, len(f.vals), newBoolStats())
}

func (f *BoolField) Read(r io.ReadSeeker, meta *parquet.Metadata, pg parquet.Page) error {
	rr, sizes, err := f.DoRead(r, meta, pg)
	if err != nil {
		return err
	}

	f.vals, err = parquet.GetBools(rr, int(pg.N), sizes)
	return err
}
{{end}}`

var boolStatsTpl = `{{define "boolStats"}}
type boolStats struct {}
func newBoolStats() *boolStats {return &boolStats{}}
func (b *boolStats) NullCount() *int64 {return nil}
func (b *boolStats) DistinctCount() *int64 {return nil}
func (b *boolStats) Min() []byte {return nil}
func (b *boolStats) Max() []byte {return nil}
{{end}}`
