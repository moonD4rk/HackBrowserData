package export

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

func structCSVHeader(value any) []string {
	structType := reflect.Indirect(reflect.ValueOf(value)).Type()
	headers := make([]string, 0, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		name := tagName(structType.Field(i), "csv")
		if name == "" {
			continue
		}
		headers = append(headers, name)
	}
	return headers
}

func structCSVRow(value any) []string {
	structValue := reflect.Indirect(reflect.ValueOf(value))
	structType := structValue.Type()
	row := make([]string, 0, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		if tagName(structType.Field(i), "csv") == "" {
			continue
		}
		row = append(row, fieldToString(structValue.Field(i)))
	}
	return row
}

func tagName(field reflect.StructField, key string) string {
	tag, ok := field.Tag.Lookup(key)
	if !ok || tag == "-" {
		return ""
	}
	if index := strings.IndexByte(tag, ','); index != -1 {
		tag = tag[:index]
	}
	if tag == "" {
		return ""
	}
	return tag
}

func fieldToString(value reflect.Value) string {
	if value.Type() == timeType {
		timestamp, _ := value.Interface().(time.Time)
		return formatTime(timestamp)
	}
	switch value.Kind() {
	case reflect.String:
		return value.String()
	case reflect.Bool:
		return formatBool(value.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)
	default:
		return fmt.Sprintf("%v", value.Interface())
	}
}

func formatTime(timestamp time.Time) string {
	if timestamp.IsZero() {
		return ""
	}
	return timestamp.Format(time.RFC3339)
}

func formatBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
