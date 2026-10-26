package i18n

import "testing"

func TestParse(t *testing.T) {
	for in, want := range map[string]string{
		"es-MX": "es-MX", "zh_CN.UTF-8": "zh-CN", "es_ES@euro": "es-ES",
		"C": "en", "POSIX": "en", "C.UTF-8": "en", "zh-Hans": "zh-Hans",
	} {
		got, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		if got.String() != want {
			t.Errorf("Parse(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "garbage!!", "en_US.UTF-8@@"} {
		if got, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) = %q, want an error", bad, got)
		}
	}
}

func TestFromEnv(t *testing.T) {
	for name, tc := range map[string]struct {
		env  map[string]string
		want string
	}{
		"lang":            {map[string]string{"LANG": "es_ES.UTF-8"}, "es-ES"},
		"messages first":  {map[string]string{"LANG": "es_ES.UTF-8", "LC_MESSAGES": "zh_CN.UTF-8"}, "zh-CN"},
		"all wins":        {map[string]string{"LANG": "es_ES", "LC_ALL": "C"}, "en"},
		"gloss wins":      {map[string]string{"GLOSS_LANG": "zh-Hans", "LC_ALL": "es"}, "zh-Hans"},
		"garbage skipped": {map[string]string{"LANG": "garbage!!", "LC_MESSAGES": ""}, "en"},
		"garbage then ok": {map[string]string{"LC_ALL": "garbage!!", "LANG": "es_MX.UTF-8"}, "es-MX"},
		"gb18030":         {map[string]string{"LC_ALL": "zh_CN.GB18030"}, "zh-CN"},
		"euro modifier":   {map[string]string{"LANG": "en_US.UTF-8@euro"}, "en-US"},
		"c utf8":          {map[string]string{"LANG": "C.UTF-8"}, "en"},
		"empty":           {map[string]string{}, "en"},
	} {
		got := FromEnv(func(k string) string { return tc.env[k] })
		if got != tc.want {
			t.Errorf("%s: FromEnv = %q, want %q", name, got, tc.want)
		}
	}
}

func TestFromAcceptLanguage(t *testing.T) {
	for in, want := range map[string]string{
		"es-MX,es;q=0.9,en;q=0.8":      "es-MX",
		"zh-CN;q=0.1, es;q=0.9":        "es",
		"zh-CN":                        "zh-CN",
		"fr, de;q=0.5":                 "en",
		"*":                            "en",
		"xx;q=abc":                     "en",
		"":                             "en",
		"fr, es;q=0.5":                 "es",
		"zh-TW,zh-Hant;q=0.9":          "en",
		string(make([]byte, 4097)):     "en",
		"es" + pad(5000) + ",zh-Hans":  "en",
		"en-GB,en;q=0.9,es;q=0.1":      "en-GB",
		"<script>alert(1)</script>,es": "en",
	} {
		if got := FromAcceptLanguage(in); got != want {
			t.Errorf("FromAcceptLanguage(%.40q) = %q, want %q", in, got, want)
		}
	}
}

func pad(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

func TestMatch(t *testing.T) {
	for in, want := range map[string]string{
		"es-MX": "es", "es-419": "es", "es": "es",
		"zh-CN": "zh-Hans", "zh": "zh-Hans", "zh-SG": "zh-Hans", "zh-Hans": "zh-Hans",
		"zh-TW": "en", "zh-Hant": "en", "fr": "en", "de-DE": "en", "en-GB": "en", "": "en",
		"garbage!!": "en",
	} {
		if got := Match(in); got != want {
			t.Errorf("Match(%q) = %q, want %q", in, got, want)
		}
	}
}
