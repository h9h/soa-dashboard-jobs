package jsonutil

import "testing"

func TestMarshalDoesNotEscapeHTML(t *testing.T) {
	payload := map[string]string{"xml": "<root a=\"1\" & b='2'>"}

	got, err := Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want := `{"xml":"<root a=\"1\" & b='2'>"}`
	if string(got) != want {
		t.Errorf("Marshal =\n  %s\nerwartet\n  %s", got, want)
	}
}

func TestMarshalHasNoTrailingNewline(t *testing.T) {
	got, err := Marshal(map[string]string{"a": "b"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got[len(got)-1] == '\n' {
		t.Errorf("Marshal endet mit einem Zeilenumbruch: %q", got)
	}
}

func TestStripKeyPreservesOrder(t *testing.T) {
	input := []byte(`{"destination":"lauf","zeta":1,"alpha":"zwei","mitte":{"b":2,"a":1}}`)

	got, err := StripKey(input, "destination")
	if err != nil {
		t.Fatalf("StripKey: %v", err)
	}

	want := `{"zeta":1,"alpha":"zwei","mitte":{"b":2,"a":1}}`
	if string(got) != want {
		t.Errorf("StripKey =\n  %s\nerwartet\n  %s", got, want)
	}
}

func TestStripKeyCompactsWhitespaceButKeepsEscaping(t *testing.T) {
	input := []byte("{\n  \"destination\": \"lauf\",\n  \"xml\": \"<a>&</a>\"\n}")

	got, err := StripKey(input, "destination")
	if err != nil {
		t.Fatalf("StripKey: %v", err)
	}

	want := `{"xml":"<a>&</a>"}`
	if string(got) != want {
		t.Errorf("StripKey =\n  %s\nerwartet\n  %s", got, want)
	}
}

func TestStripKeyKeepsNumbersVerbatim(t *testing.T) {
	input := []byte(`{"timestamp":1758445200000,"quote":1.10,"exp":1e3}`)

	got, err := StripKey(input, "destination")
	if err != nil {
		t.Fatalf("StripKey: %v", err)
	}

	want := `{"timestamp":1758445200000,"quote":1.10,"exp":1e3}`
	if string(got) != want {
		t.Errorf("StripKey =\n  %s\nerwartet\n  %s", got, want)
	}
}

func TestStripKeyOnMissingKeyReturnsEverything(t *testing.T) {
	input := []byte(`{"a":1,"b":2}`)

	got, err := StripKey(input, "destination")
	if err != nil {
		t.Fatalf("StripKey: %v", err)
	}
	if string(got) != `{"a":1,"b":2}` {
		t.Errorf("StripKey = %s", got)
	}
}

func TestStripKeyEmptyResultIsEmptyObject(t *testing.T) {
	got, err := StripKey([]byte(`{"destination":"lauf"}`), "destination")
	if err != nil {
		t.Fatalf("StripKey: %v", err)
	}
	if string(got) != `{}` {
		t.Errorf("StripKey = %s, erwartet {}", got)
	}
}

func TestStripKeyRejectsNonObjects(t *testing.T) {
	for _, input := range []string{`[1,2]`, `"text"`, `null`, `{`} {
		if _, err := StripKey([]byte(input), "destination"); err == nil {
			t.Errorf("StripKey(%s) haette fehlschlagen muessen", input)
		}
	}
}
