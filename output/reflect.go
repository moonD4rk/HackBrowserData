package output

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

// structCSVHeader extracts CSV column names from a struct's csv tags.
func structCSVHeader(v any) []string {
	t := reflect.TypeOf(v)
	headers := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name := tagName(t.Field(i), "csv")
		if name == "" {
			continue
		}
		headers = append(headers, name)
	}
	return headers
}

// structCSVRow converts a struct's field values to CSV string values,
// including only fields that have a csv tag.
func structCSVRow(v any) []string {
	val := reflect.ValueOf(v)
	t := val.Type()
	row := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if tagName(t.Field(i), "csv") == "" {
			continue
		}
		row = append(row, fieldToString(val.Field(i)))
	}
	return row
}

// tagName extracts the tag value for the given key from a struct field.
// Uses Lookup (not Get) to distinguish "no tag" from "empty tag".
// Returns "" if the tag is absent, empty, or "-".
func tagName(f reflect.StructField, key string) string {
	tag, ok := f.Tag.Lookup(key)
	if !ok || tag == "-" {
		return ""
	}
	if idx := strings.IndexByte(tag, ','); idx != -1 {
		tag = tag[:idx]
	}
	if tag == "" {
		return ""
	}
	return tag
}

func fieldToString(v reflect.Value) string {
	// Check time.Time before kind switch since it's a struct.
	if v.Type() == timeType {
		t, _ := v.Interface().(time.Time)
		return formatTime(t)
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return formatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func formatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
