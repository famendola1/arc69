package arc69

import (
	"fmt"
	"testing"
)

func checkProperty(name, want string, meta *Metadata, t *testing.T) {
	got, err := meta.Property(name)
	if err != nil {
		t.Errorf("Property(%q) failed with error: %s, want success", name, err)
		return
	}

	if got != want {
		t.Errorf("Property(%q) = %s, want %s", name, got, want)
		return
	}
}

func TestMetadataPropertySuccess(t *testing.T) {
	meta := &Metadata{
		Properties: map[string]interface{}{
			"a": "aa",
			"b": map[string]interface{}{"bb": "bbb"},
			"c": map[string]interface{}{"cc": map[string]interface{}{"ccc": "cccc"}},
		},
	}

	checkProperty("a", "aa", meta, t)
	checkProperty("b.bb", "bbb", meta, t)
	checkProperty("c.cc.ccc", "cccc", meta, t)
}

func TestMetadataPropertyNotFound(t *testing.T) {
	meta := &Metadata{
		Properties: map[string]interface{}{"a": "aa"},
	}

	_, got := meta.Property("b")
	want := fmt.Errorf("unable to get property b: property b is not valid")

	if got.Error() != want.Error() {
		t.Errorf("got error: %s, want error: %s", got, want)
	}

	_, got = meta.Property("a.aa")
	want = fmt.Errorf("unable to get property a.aa: property a is not a map")

	if got.Error() != want.Error() {
		t.Errorf("got error: %s, want error: %s", got, want)
	}

	_, got = meta.Property("")
	want = fmt.Errorf("no path provided")
	if got.Error() != want.Error() {
		t.Errorf("got error: %s, want error: %s", got, want)
	}
}

func TestMetadataIsValid(t *testing.T) {
	validMeta := &Metadata{Standard: "arc69"}
	invalidMeta := &Metadata{Standard: "arc68"}

	if validMeta.IsValid() != true {
		t.Errorf("IsValid(%+v) = false, want true", *validMeta)
	}

	if invalidMeta.IsValid() != false {
		t.Errorf("IsValid(%+v) = true, want false", *invalidMeta)
	}
}
