package main

var boolOptionalTpl = `{{define "boolOptionalField"}}type BoolOptionalField struct {
	parquet.OptionalField
	vals  []bool
	val   func(r {{.Type}}) *bool
	read  func(r *{{.Type}}, v *bool)
	stats parquet.Stats
}

func NewBoolOptionalField(val func(r {{.Type}}) *bool, read func(r *{{.Type}}, v *bool), col string, opts ...func(*parquet.OptionalField)) *BoolOptionalField {
	return &BoolOptionalField{
		val:           val,
		read:          read,
		OptionalField: parquet.NewOptionalField(col, opts...),
		stats:         newOptionalStats(),
	}
}

func (f *BoolOptionalField) Schema() parquet.Field {
	return parquet.Field{Name: f.Name(), Type: parquet.BoolType, RepetitionType: parquet.RepetitionOptional}
}

func (f *BoolOptionalField) Read(r io.ReadSeeker, meta *parquet.Metadata, pg parquet.Page) error {
	rr, sizes, err := f.DoRead(r, meta, pg)
	if err != nil {
		return err
	}

	v, err := parquet.GetBools(rr, f.Values()-len(f.vals), sizes)
	f.vals = append(f.vals, v...)
	return err
}

func (f *BoolOptionalField) Scan(r *{{.Type}}) {
	if len(f.Defs) == 0 {
		return
	}

	var val *bool
	if f.Defs[0] == 1 {
		v := f.vals[0]
		f.vals = f.vals[1:]
		val = &v
		f.read(r, val)
	}
	f.Defs = f.Defs[1:]
}

func (f *BoolOptionalField) Add(r {{.Type}}) {
	v := f.val(r)
	if v != nil {
		f.vals = append(f.vals, *v)
		f.Defs = append(f.Defs, 1)
	} else {
		f.Defs = append(f.Defs, 0)
	}
}

func (f *BoolOptionalField) Write(w io.Writer, meta *parquet.Metadata) error {
	ln := len(f.vals)
	byteNum := (ln + 7) / 8
	rawBuf := make([]byte, byteNum)

	for i := 0; i < ln; i++ {
		if f.vals[i] {
			rawBuf[i/8] = rawBuf[i/8] | (1 << uint32(i%8))
		}
	}

	return f.DoWrite(w, meta, rawBuf, len(f.vals), f.stats())
}
{{end}}`

var boolOptionalStatsTpl = `{{define "boolOptionalStats"}}
type boolOptionalStats struct {
	nilCount int64
}

func newboolOptionalStats() *boolOptionalStats {
	return &boolOptionalStats{}
}

func (b *boolOptionalstats) add(val *bool) {
	if val == nil {
		b.nilCount++	
	}
}

func (b *boolOptionalStats) NullCount() *int64 {
	return &b.nilCount
}

func (b *boolOptionalStats) DistinctCount() *int64 {
	return nil
}

func (b *boolOptionalStats) Min() []byte {
	return nil
}

func (b *boolOptionalStats) Max() []byte {
	return nil
}
{{end}}`
