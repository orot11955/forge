package i18n

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed messages.*.yaml
var messageFiles embed.FS

type Lang string

const (
	LangEn Lang = "en"
	LangKo Lang = "ko"
)

func IsValid(l string) bool {
	return l == string(LangEn) || l == string(LangKo)
}

type Translator struct {
	Lang     Lang
	messages map[Lang]map[string]string
}

func New(lang Lang) (*Translator, error) {
	t := &Translator{Lang: lang, messages: map[Lang]map[string]string{}}
	for _, l := range []Lang{LangEn, LangKo} {
		raw, err := messageFiles.ReadFile("messages." + string(l) + ".yaml")
		if err != nil {
			return nil, fmt.Errorf("read i18n %s: %w", l, err)
		}
		var nested map[string]any
		if err := yaml.Unmarshal(raw, &nested); err != nil {
			return nil, fmt.Errorf("parse i18n %s: %w", l, err)
		}
		flat := map[string]string{}
		flatten("", nested, flat)
		t.messages[l] = flat
	}
	return t, nil
}

func flatten(prefix string, in map[string]any, out map[string]string) {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch x := v.(type) {
		case map[string]any:
			flatten(key, x, out)
		case string:
			out[key] = x
		default:
			out[key] = fmt.Sprintf("%v", x)
		}
	}
}

func (t *Translator) T(key string, args ...any) string {
	if msg, ok := t.messages[t.Lang][key]; ok {
		if len(args) == 0 {
			return msg
		}
		return fmt.Sprintf(msg, args...)
	}
	if msg, ok := t.messages[LangEn][key]; ok {
		if len(args) == 0 {
			return msg
		}
		return fmt.Sprintf(msg, args...)
	}
	return key
}

func ResolveLang(flag, env, cfg string) Lang {
	for _, candidate := range []string{flag, env, cfg} {
		c := strings.TrimSpace(candidate)
		if c == "" {
			continue
		}
		if IsValid(c) {
			return Lang(c)
		}
	}
	return LangEn
}
