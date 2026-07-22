package extensions

import "testing"

func TestNormalizeCatalogSourceURL(t *testing.T) {
	got, err := NormalizeCatalogSourceURL("https://github.com/qlpqlp/dogego/tree/main/DogeGo/extensions/catalog/")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://raw.githubusercontent.com/qlpqlp/dogego/main/DogeGo/extensions/catalog/catalog.json"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	got2, err := NormalizeCatalogSourceURL(DefaultCatalogURL)
	if err != nil || got2 != DefaultCatalogURL {
		t.Fatalf("raw json: got %q err %v", got2, err)
	}
}
