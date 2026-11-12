package service

import "testing"

func TestNormalizeTreeFilter(t *testing.T) {
	got, err := NormalizeTreeFilter("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "all" || got.NodeXMLIDs == nil || len(got.NodeXMLIDs) != 0 {
		t.Fatalf("default: %+v", got)
	}
	got, err = NormalizeTreeFilter("schemes", []string{"a", "", "a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "schemes" || len(got.NodeXMLIDs) != 2 || got.NodeXMLIDs[0] != "a" || got.NodeXMLIDs[1] != "b" {
		t.Fatalf("dedupe: %+v", got)
	}
	if _, err = NormalizeTreeFilter("exclude", nil); err != ErrInvalidTreeFilterMode {
		t.Fatalf("want invalid mode, got %v", err)
	}
}
