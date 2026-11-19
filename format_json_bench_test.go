package clix

import (
	jsonv1 "encoding/json"
	"io"
	"testing"
	"time"
)

type benchRecord struct {
	ID       int               `json:"id"`
	Name     string            `json:"name"`
	URL      string            `json:"url"`
	Tags     []string          `json:"tags"`
	Labels   map[string]string `json:"labels"`
	Created  time.Time         `json:"created"`
	Ratio    float64           `json:"ratio"`
	Enabled  bool              `json:"enabled"`
	Children []benchChild      `json:"children,omitempty"`
}

type benchChild struct {
	Key   string `json:"key"`
	Value int    `json:"value"`
}

// benchPayload is a typical "list" result: 50 records with nested values.
func benchPayload() []benchRecord {
	records := make([]benchRecord, 50)
	for i := range records {
		records[i] = benchRecord{
			ID: i, Name: "record", URL: "https://x.test/items?id=1&page=2",
			Tags:    []string{"alpha", "beta", "gamma"},
			Labels:  map[string]string{"zone": "eu-west-1", "team": "core", "env": "prod"},
			Created: time.Date(2026, 11, 19, 8, 30, 0, i, time.UTC),
			Ratio:   float64(i) / 3, Enabled: i%2 == 0,
			Children: []benchChild{{"a", 1}, {"b", 2}},
		}
	}
	return records
}

// BenchmarkFormatJSON compares the encoding/json (v1) encoder FormatData
// used before with the encoding/json/v2 encoder it uses now.
func BenchmarkFormatJSON(b *testing.B) {
	data := benchPayload()
	b.Run("v1", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			enc := jsonv1.NewEncoder(io.Discard)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			if err := enc.Encode(data); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("v2", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := formatJSON(io.Discard, data); err != nil {
				b.Fatal(err)
			}
		}
	})
}
