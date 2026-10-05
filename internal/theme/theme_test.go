package theme

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuiltinsDistinct(t *testing.T) {
	if reflect.DeepEqual(Darkula(), Default()) {
		t.Error("darkula must differ from default")
	}
	if reflect.DeepEqual(Light(), Default()) {
		t.Error("light must differ from default")
	}
	if reflect.DeepEqual(Darkula(), Light()) {
		t.Error("darkula must differ from light")
	}
}

func TestBuiltinsComplete(t *testing.T) {
	for name, s := range map[string]Scheme{"default": Default(), "darkula": Darkula(), "light": Light()} {
		typ := reflect.TypeOf(s)
		for i := 0; i < typ.NumField(); i++ {
			val := reflect.ValueOf(s).Field(i).String()
			if val == "" {
				t.Errorf("%s: field %q is empty", name, typ.Field(i).Name)
			}
		}
	}
}

func TestByName(t *testing.T) {
	if _, ok := ByName(DefaultName); !ok {
		t.Error("default not found")
	}
	if _, ok := ByName(DarkulaName); !ok {
		t.Error("darkula not found")
	}
	if _, ok := ByName(LightName); !ok {
		t.Error("light not found")
	}
	if _, ok := ByName("nope"); ok {
		t.Error("unknown builtin must not resolve")
	}
}

func TestByNameOrFileFallback(t *testing.T) {
	s, ok := ByNameOrFile("")
	if !ok {
		t.Error("empty name should fall back to default ok")
	}
	if s != Default() {
		t.Error("empty name should return default")
	}
	// unknown non-existent custom file falls back to default
	s, ok = ByNameOrFile("does-not-exist")
	if ok {
		t.Error("unknown custom name should report ok=false")
	}
	if s != Default() {
		t.Error("unknown custom name should fall back to default")
	}
}

func TestLoadFilePartialFallsBack(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "p.theme")
	// only override method; everything else should inherit from default
	content := "method: 167\ncomment: 100\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Method != "167" {
		t.Errorf("method not overridden: got %q", s.Method)
	}
	if s.Comment != "100" {
		t.Errorf("comment not overridden: got %q", s.Comment)
	}
	// inherited from default
	if s.URL != Default().URL {
		t.Errorf("URL should inherit default %q, got %q", Default().URL, s.URL)
	}
	if s.StatusBg != Default().StatusBg {
		t.Errorf("StatusBg should inherit default %q, got %q", Default().StatusBg, s.StatusBg)
	}
}

func TestLoadFileMissing(t *testing.T) {
	if _, err := LoadFile(filepath.Join(t.TempDir(), "nope.theme")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadFileInvalidYaml(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "b.theme")
	if err := os.WriteFile(p, []byte("method: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(p); err == nil {
		t.Error("expected error for malformed yaml")
	}
}

func TestListFilesInDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.theme"), []byte("method: 1"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.theme"), []byte("method: 2"), 0o644)
	os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0o644)
	os.Mkdir(filepath.Join(dir, "subdir.theme"), 0o755)

	names := ListFilesInDir(dir)
	if len(names) != 2 {
		t.Fatalf("expected 2 theme files, got %v", names)
	}
	if names[0] != "a" || names[1] != "b" {
		t.Errorf("unexpected sorted names: %v", names)
	}
}

func TestListFilesMissingDir(t *testing.T) {
	if got := ListFilesInDir(filepath.Join(t.TempDir(), "nope")); len(got) != 0 {
		t.Errorf("missing dir should yield empty list, got %v", got)
	}
}

func TestWriteTemplateCreatesParseableFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "themes", "custom.theme")
	if err := WriteTemplate("custom", WriteTemplateOptions{Base: Darkula()}, out); err != nil {
		t.Fatal(err)
	}
	// loaded file should equal Darkula (since every field is written out)
	s, err := LoadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, Darkula()) {
		t.Error("template file should round-trip to the base scheme")
	}
}

func TestWriteTemplateRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "existing.theme")
	if err := os.WriteFile(out, []byte("method: 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteTemplate("existing", WriteTemplateOptions{Base: Default()}, out); err == nil {
		t.Error("expected refusal to overwrite without Overwrite flag")
	}
	if err := WriteTemplate("existing", WriteTemplateOptions{Base: Default(), Overwrite: true}, out); err != nil {
		t.Errorf("overwrite should succeed, got %v", err)
	}
	// still a valid theme afterwards
	if _, err := LoadFile(out); err != nil {
		t.Errorf("overwritten file should parse: %v", err)
	}
}
