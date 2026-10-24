package logread_test

// Shared helpers for the lifecycle and notification decoding tests.

import (
	"reflect"
	"strings"
	"testing"

	"mosaic-log-analyzer/internal/domain"
)

// blocksSet counts the non-nil field blocks on ev. Extend the list whenever a
// new block is added to domain.Event.
func blocksSet(ev domain.Event) int {
	n := 0
	for _, present := range []bool{ev.RunStart != nil, ev.RunEnd != nil, ev.SessionStart != nil,
		ev.SessionEnd != nil, ev.Notification != nil, ev.Turn != nil, ev.UsageRecord != nil,
		ev.InvocationStart != nil, ev.InvocationEnd != nil, ev.ToolCallStart != nil,
		ev.ToolCallEnd != nil, ev.Compaction != nil} {
		if present {
			n++
		}
	}
	return n
}

// collectStrings appends every string reachable from v (following pointers,
// structs, slices, maps and interfaces) to out.
func collectStrings(v reflect.Value, out *[]string) {
	switch v.Kind() {
	case reflect.String:
		*out = append(*out, v.String())
	case reflect.Ptr, reflect.Interface:
		if !v.IsNil() {
			collectStrings(v.Elem(), out)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			collectStrings(v.Field(i), out)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			collectStrings(v.Index(i), out)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			collectStrings(k, out)
			collectStrings(v.MapIndex(k), out)
		}
	}
}

// assertNoMarkers fails if any string anywhere inside ev (all blocks, nested
// pointers and the envelope) contains one of the markers.
func assertNoMarkers(t *testing.T, ev domain.Event, markers ...string) {
	t.Helper()
	var all []string
	collectStrings(reflect.ValueOf(ev), &all)
	for _, s := range all {
		for _, m := range markers {
			if strings.Contains(s, m) {
				t.Errorf("event %q leaks marker %q in decoded value %q", ev.Type, m, s)
			}
		}
	}
}
