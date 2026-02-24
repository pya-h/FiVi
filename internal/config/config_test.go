package config

import (
	"reflect"
	"testing"

	"fivi/internal/sources"
)

func TestDefaultSourcesUsesRegistry(t *testing.T) {
	got := DefaultSources()
	want := sources.DefaultURLs()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default sources do not match registry\n got=%v\nwant=%v", got, want)
	}
}
