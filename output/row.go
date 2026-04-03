package output

import (
	"encoding/json"
	"reflect"
)

// row wraps any entry with browser/profile context for output.
type row struct {
	Browser string
	Profile string
	entry   any
}

func (r row) csvHeader() []string {
	return append([]string{"browser", "profile"}, structCSVHeader(r.entry)...)
}

func (r row) csvRow() []string {
	return append([]string{r.Browser, r.Profile}, structCSVRow(r.entry)...)
}

// MarshalJSON produces flat JSON with browser/profile followed by the entry's fields.
// Uses reflect.StructOf to dynamically build a struct that json.Marshal handles natively,
// avoiding manual JSON string concatenation.
func (r row) MarshalJSON() ([]byte, error) {
	ev := reflect.ValueOf(r.entry)
	et := ev.Type()

	fields := make([]reflect.StructField, 0, et.NumField()+2)
	fields = append(fields,
		reflect.StructField{Name: "Browser", Type: reflect.TypeOf(""), Tag: `json:"browser"`},
		reflect.StructField{Name: "Profile", Type: reflect.TypeOf(""), Tag: `json:"profile"`},
	)
	for i := 0; i < et.NumField(); i++ {
		fields = append(fields, et.Field(i))
	}

	flat := reflect.New(reflect.StructOf(fields)).Elem()
	flat.Field(0).SetString(r.Browser)
	flat.Field(1).SetString(r.Profile)
	for i := 0; i < et.NumField(); i++ {
		flat.Field(i + 2).Set(ev.Field(i))
	}

	return json.Marshal(flat.Interface())
}
