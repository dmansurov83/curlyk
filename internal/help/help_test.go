package help

import (
	"testing"

	"github.com/user/curlyk/internal/i18n"
)

func TestEveryLineIsTranslatedInBothLocales(t *testing.T) {
	keys := make([]string, 0, len(TUIKeys)+len(GUIKeys)+len(VarLines)+2)
	for _, e := range TUIKeys {
		keys = append(keys, e.DescKey)
	}
	for _, e := range GUIKeys {
		keys = append(keys, e.DescKey)
	}
	for _, v := range VarLines {
		keys = append(keys, v.Key)
	}
	keys = append(keys, "help.title", "help.summary", "help.varTitle", "help.varHint")

	for _, lang := range []string{"ru", "en"} {
		for _, key := range keys {
			if !i18n.Has(lang, key) {
				t.Errorf("locale %q is missing key %q used by the help reference", lang, key)
			}
		}
	}
}

func TestKeyListsAreNotEmptyAndHaveKeys(t *testing.T) {
	for name, list := range map[string][]Entry{"TUIKeys": TUIKeys, "GUIKeys": GUIKeys} {
		if len(list) == 0 {
			t.Fatalf("%s is empty", name)
		}
		for _, e := range list {
			if e.Key == "" || e.DescKey == "" {
				t.Fatalf("%s has an entry with an empty key or description: %+v", name, e)
			}
		}
	}
}
