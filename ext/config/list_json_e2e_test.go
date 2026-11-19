package config

import "testing"

// Golden `config list --format json` output, encoded like clix.FormatData
// (encoding/json/v2). Config values are always strings, so durations,
// times, bytes and nil values cannot occur here.
func TestConfigE2E_ListJSONGolden(t *testing.T) {
	for _, tc := range []struct {
		name   string
		values map[string]string
		want   string
	}{
		{"keys are sorted at every level", map[string]string{"zeta": "1", "alpha.d": "4", "alpha.c": "3", "mid": "x"},
			"{\n  \"alpha\": {\n    \"c\": \"3\",\n    \"d\": \"4\"\n  },\n  \"mid\": \"x\",\n  \"zeta\": \"1\"\n}\n"},
		{"nested values", map[string]string{"a.b.c": "deep", "a.e": "shallow"},
			"{\n  \"a\": {\n    \"b\": {\n      \"c\": \"deep\"\n    },\n    \"e\": \"shallow\"\n  }\n}\n"},
		{"a key that is also a prefix", map[string]string{"a": "1", "a.b": "2"}, "{\n  \"a\": \"1\",\n  \"a.b\": \"2\"\n}\n"},
		{"invalid UTF-8 is replaced", map[string]string{"k": "bad\xff"}, "{\n  \"k\": \"bad\ufffd\"\n}\n"},
		{"HTML characters are not escaped", map[string]string{"url": "https://x.test/?a=1&b=<2>"}, "{\n  \"url\": \"https://x.test/?a=1&b=<2>\"\n}\n"},
		{"empty", map[string]string{}, "{}\n"},
	} {
		app, out, _ := e2eApp(t, "")
		for k, v := range tc.values {
			app.Config.Set(k, v)
		}
		if got := run(t, app, out, "config", "list", "--format", "json"); got != tc.want {
			t.Errorf("%s:\n%s\nwant:\n%s", tc.name, got, tc.want)
		}
	}
}
