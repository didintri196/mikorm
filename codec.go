package mikorm

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const tagName = "mikorm"

var (
	durationType        = reflect.TypeOf(time.Duration(0))
	textMarshalerType   = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	stringMapType       = reflect.TypeOf(map[string]string(nil))
)

// field describes a struct field mapped to a RouterOS property.
type field struct {
	name     string
	index    []int
	readonly bool
}

var fieldCache sync.Map // map[reflect.Type][]field

func cachedFields(t reflect.Type) []field {
	if f, ok := fieldCache.Load(t); ok {
		return f.([]field)
	}
	fields := typeFields(t, nil)
	f, _ := fieldCache.LoadOrStore(t, fields)
	return f.([]field)
}

func typeFields(t reflect.Type, index []int) []field {
	var fields []field
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		tag, hasTag := sf.Tag.Lookup(tagName)
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		idx := append(index[:len(index):len(index)], i)

		if sf.Anonymous && name == "" && sf.Type.Kind() == reflect.Struct && !isTextType(sf.Type) {
			fields = append(fields, typeFields(sf.Type, idx)...)
			continue
		}
		if !sf.IsExported() {
			continue
		}
		if !hasTag || name == "" {
			name = kebabCase(sf.Name)
		}
		readonly := name == ".id"
		for _, o := range strings.Split(opts, ",") {
			if strings.TrimSpace(o) == "readonly" {
				readonly = true
			}
		}
		fields = append(fields, field{name: name, index: idx, readonly: readonly})
	}
	return fields
}

func isTextType(t reflect.Type) bool {
	return t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textUnmarshalerType)
}

// kebabCase converts a Go identifier to a RouterOS property name:
// "MACAddress" -> "mac-address", "RxByte" -> "rx-byte".
func kebabCase(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
					b.WriteByte('-')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		if r == '_' {
			r = '-'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// --- decoding ---------------------------------------------------------------

func checkSliceDest(dest any) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("mikorm: destination must be a non-nil pointer to a slice, got %T", dest)
	}
	return checkElemType(v.Elem().Type().Elem(), dest)
}

func checkRecordDest(dest any) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return fmt.Errorf("mikorm: destination must be a non-nil pointer to a struct or map[string]string, got %T", dest)
	}
	return checkElemType(v.Elem().Type(), dest)
}

func checkElemType(t reflect.Type, dest any) error {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct || t == stringMapType {
		return nil
	}
	return fmt.Errorf("mikorm: unsupported destination type %T", dest)
}

// decodeRecords stores records into dest, a pointer to a slice.
func decodeRecords(records []map[string]string, dest any) error {
	if err := checkSliceDest(dest); err != nil {
		return err
	}
	sv := reflect.ValueOf(dest).Elem()
	out := reflect.MakeSlice(sv.Type(), len(records), len(records))
	for i, rec := range records {
		if err := decodeInto(rec, out.Index(i)); err != nil {
			return err
		}
	}
	sv.Set(out)
	return nil
}

// decodeRecord stores rec into dest, a pointer to a struct or map.
func decodeRecord(rec map[string]string, dest any) error {
	if err := checkRecordDest(dest); err != nil {
		return err
	}
	return decodeInto(rec, reflect.ValueOf(dest).Elem())
}

func decodeInto(rec map[string]string, v reflect.Value) error {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	if v.Type() == stringMapType {
		m := make(map[string]string, len(rec))
		for k, val := range rec {
			m[k] = val
		}
		v.Set(reflect.ValueOf(m))
		return nil
	}
	for _, f := range cachedFields(v.Type()) {
		s, ok := rec[f.name]
		if !ok {
			continue
		}
		if err := setField(v.FieldByIndex(f.index), s); err != nil {
			return fmt.Errorf("mikorm: decode %q into %s: %w", f.name, v.Type(), err)
		}
	}
	return nil
}

func setField(v reflect.Value, s string) error {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return setField(v.Elem(), s)
	}
	if v.CanAddr() && v.Addr().Type().Implements(textUnmarshalerType) {
		return v.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s))
	}
	if v.Type() == durationType {
		d, err := ParseDuration(s)
		if err != nil {
			return err
		}
		v.SetInt(int64(d))
		return nil
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		b, err := parseBool(s)
		if err != nil {
			return err
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if s == "" {
			v.SetInt(0)
			return nil
		}
		n, err := strconv.ParseInt(s, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if s == "" {
			v.SetUint(0)
			return nil
		}
		n, err := strconv.ParseUint(s, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetUint(n)
	case reflect.Float32, reflect.Float64:
		if s == "" {
			v.SetFloat(0)
			return nil
		}
		n, err := strconv.ParseFloat(s, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetFloat(n)
	case reflect.Slice:
		if v.Type().Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported type %s", v.Type())
		}
		var parts []string
		if s != "" {
			parts = strings.Split(s, ",")
		}
		out := reflect.MakeSlice(v.Type(), len(parts), len(parts))
		for i, p := range parts {
			out.Index(i).SetString(p)
		}
		v.Set(out)
	default:
		return fmt.Errorf("unsupported type %s", v.Type())
	}
	return nil
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "yes":
		return true, nil
	case "false", "no", "":
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean %q", s)
}

// --- encoding ---------------------------------------------------------------

type pair struct {
	key, value string
}

// encodeValues turns a struct, pointer to struct or map into property
// pairs. Zero struct fields are skipped; read-only fields are skipped when
// write is true.
func encodeValues(values any, write bool) ([]pair, error) {
	if values == nil {
		return nil, nil
	}
	v := reflect.ValueOf(values)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return nil, fmt.Errorf("mikorm: map key must be a string, got %s", v.Type())
		}
		pairs := make([]pair, 0, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			s, err := formatValue(iter.Value())
			if err != nil {
				return nil, fmt.Errorf("mikorm: encode %q: %w", iter.Key().String(), err)
			}
			pairs = append(pairs, pair{iter.Key().String(), s})
		}
		// Map iteration order is random; keep commands deterministic.
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].key < pairs[j].key })
		return pairs, nil

	case reflect.Struct:
		var pairs []pair
		for _, f := range cachedFields(v.Type()) {
			if write && f.readonly {
				continue
			}
			fv, ok := fieldByIndex(v, f.index)
			if !ok {
				continue
			}
			// Nil pointers are skipped; non-nil pointers are always sent so
			// zero values can be set explicitly.
			if fv.Kind() == reflect.Pointer {
				if fv.IsNil() {
					continue
				}
			} else if fv.IsZero() {
				continue
			}
			s, err := formatValue(fv)
			if err != nil {
				return nil, fmt.Errorf("mikorm: encode %q: %w", f.name, err)
			}
			pairs = append(pairs, pair{f.name, s})
		}
		return pairs, nil
	}
	return nil, fmt.Errorf("mikorm: values must be a struct or map, got %T", values)
}

// fieldByIndex is like reflect.Value.FieldByIndex but reports false instead
// of panicking on a nil embedded pointer.
func fieldByIndex(v reflect.Value, index []int) (reflect.Value, bool) {
	for i, x := range index {
		if i > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}, false
			}
			v = v.Elem()
		}
		v = v.Field(x)
	}
	return v, true
}

func formatAny(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	return formatValue(reflect.ValueOf(value))
}

func formatValue(v reflect.Value) (string, error) {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return "", nil
		}
		v = v.Elem()
	}
	if v.Type().Implements(textMarshalerType) {
		b, err := v.Interface().(encoding.TextMarshaler).MarshalText()
		return string(b), err
	}
	if v.CanAddr() && v.Addr().Type().Implements(textMarshalerType) {
		b, err := v.Addr().Interface().(encoding.TextMarshaler).MarshalText()
		return string(b), err
	}
	if v.Type() == durationType {
		return FormatDuration(time.Duration(v.Int())), nil
	}

	switch v.Kind() {
	case reflect.String:
		return v.String(), nil
	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, v.Type().Bits()), nil
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.String {
			parts := make([]string, v.Len())
			for i := range parts {
				parts[i] = v.Index(i).String()
			}
			return strings.Join(parts, ","), nil
		}
	}
	return "", errors.New("unsupported type " + v.Type().String())
}
