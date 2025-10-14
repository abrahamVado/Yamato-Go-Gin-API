package i18n

import "testing"

//1.- Test that translating a known key returns the localized string.
func TestTranslatePreferredLocale(t *testing.T) {
	//2.- Initialize a translator with the Spanish locale.
	translator, err := New("es-MX")
	if err != nil {
		t.Fatalf("expected translator, got error: %v", err)
	}

	//3.- Verify that a known key resolves to the Spanish translation.
	got := translator.Translate("response.success")
	want := "Operación completada con éxito."
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

//4.- Test that missing keys fall back to the default locale.
func TestTranslateFallbackToDefaultLocale(t *testing.T) {
	//5.- Create a translator for Spanish, which lacks the pending response key.
	translator, err := New("es-MX")
	if err != nil {
		t.Fatalf("expected translator, got error: %v", err)
	}

	//6.- Ensure the fallback string from the English locale is used.
	got := translator.Translate("response.pending")
	want := "Your request is pending review."
	if got != want {
		t.Fatalf("expected fallback %q, got %q", want, got)
	}
}

//7.- Test that unsupported locales still allow translation via defaults.
func TestTranslateUnsupportedLocale(t *testing.T) {
	//8.- Request a translator for an unsupported locale code.
	translator, err := New("fr-FR")
	if err != nil {
		t.Fatalf("expected translator, got error: %v", err)
	}

	//9.- Confirm that default English translations are provided.
	got := translator.Translate("validation.required")
	want := "This field is required."
	if got != want {
		t.Fatalf("expected default %q, got %q", want, got)
	}
}

//10.- Test that missing keys return the key identifier when absent everywhere.
func TestTranslateMissingKey(t *testing.T) {
	//11.- Use the default locale to check handling of unknown keys.
	translator, err := New("")
	if err != nil {
		t.Fatalf("expected translator, got error: %v", err)
	}

	//12.- The missing key should be returned verbatim.
	got := translator.Translate("nonexistent.key")
	want := "nonexistent.key"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
