package importfile

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleXML = `<?xml version="1.0"?><Solution Name="demo"/>`

func writeTemp(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "import-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func zipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		fw, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipBytesOrdered(t *testing.T, names []string, bodies [][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i, name := range names {
		fw, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(bodies[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestResolveImportXMLZipRootSuccessIgnoresPrimaryData(t *testing.T) {
	data := zipBytes(t, map[string][]byte{
		"Solution.xml":    []byte(sampleXML),
		"PrimaryData.xml": []byte(`<?xml version="1.0"?><PrimaryData/>`),
	})
	path := writeTemp(t, data)
	got, format, err := ResolveImportXML(path, Limits{MaxXMLBytes: 1 << 20}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatZIP {
		t.Fatalf("format=%q", format)
	}
	if string(got) != sampleXML {
		t.Fatalf("got %q", got)
	}
}

func TestResolveImportXMLRawXML(t *testing.T) {
	path := writeTemp(t, []byte(sampleXML))
	got, format, err := ResolveImportXML(path, Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatXML {
		t.Fatalf("format=%q", format)
	}
	if string(got) != sampleXML {
		t.Fatalf("got %q", got)
	}
}

func TestResolveImportXMLMissingSolution(t *testing.T) {
	data := zipBytes(t, map[string][]byte{
		"PrimaryData.xml": []byte(`<PrimaryData/>`),
	})
	path := writeTemp(t, data)
	_, _, err := ResolveImportXML(path, Limits{}, nil)
	if !errors.Is(err, ErrSolutionXMLNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveImportXMLNestedSolution(t *testing.T) {
	data := zipBytes(t, map[string][]byte{
		"nested/Solution.xml": []byte(sampleXML),
	})
	path := writeTemp(t, data)
	_, _, err := ResolveImportXML(path, Limits{}, nil)
	if !errors.Is(err, ErrInvalidEntryPath) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveImportXMLPathTraversal(t *testing.T) {
	data := zipBytes(t, map[string][]byte{
		"../Solution.xml": []byte(sampleXML),
	})
	path := writeTemp(t, data)
	_, _, err := ResolveImportXML(path, Limits{}, nil)
	if !errors.Is(err, ErrInvalidEntryPath) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveImportXMLEncryptedFlag(t *testing.T) {
	raw := zipBytes(t, map[string][]byte{"Solution.xml": []byte(sampleXML)})
	path := writeTemp(t, setZipEncryptedFlag(raw))
	_, _, err := ResolveImportXML(path, Limits{}, nil)
	if !errors.Is(err, ErrEncrypted) {
		t.Fatalf("err=%v", err)
	}
}

func setZipEncryptedFlag(data []byte) []byte {
	out := bytes.Clone(data)
	for i := 0; i+10 <= len(out); i++ {
		if out[i] == 'P' && out[i+1] == 'K' && out[i+2] == 3 && out[i+3] == 4 {
			out[i+6] |= 1
		}
		if out[i] == 'P' && out[i+1] == 'K' && out[i+2] == 1 && out[i+3] == 2 {
			out[i+8] |= 1
		}
	}
	return out
}

func TestResolveImportXMLUncompressedSizeLimit(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 200)
	xml := append([]byte(`<?xml version="1.0"?><Solution>`), append(body, []byte(`</Solution>`)...)...)
	data := zipBytes(t, map[string][]byte{"Solution.xml": xml})
	path := writeTemp(t, data)
	_, _, err := ResolveImportXML(path, Limits{MaxXMLBytes: 50}, nil)
	if !errors.Is(err, ErrXMLTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveImportXMLRARSignature(t *testing.T) {
	rar := []byte("Rar!\x1a\x07\x00\x00")
	path := writeTemp(t, rar)
	_, _, err := ResolveImportXML(path, Limits{}, nil)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveImportXMLDuplicateSolution(t *testing.T) {
	data := zipBytesOrdered(t, []string{"Solution.xml", "Solution.xml"}, [][]byte{[]byte(sampleXML), []byte(sampleXML)})
	path := writeTemp(t, data)
	_, _, err := ResolveImportXML(path, Limits{}, nil)
	if !errors.Is(err, ErrInvalidEntryPath) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveImportXMLTooManyEntries(t *testing.T) {
	files := map[string][]byte{"Solution.xml": []byte(sampleXML)}
	for i := 0; i < 10; i++ {
		files[filepath.ToSlash(filepath.Join("extra", string(rune('a'+i))+".txt"))] = []byte("x")
	}
	data := zipBytes(t, files)
	path := writeTemp(t, data)
	_, _, err := ResolveImportXML(path, Limits{MaxEntries: 3}, nil)
	if !errors.Is(err, ErrTooManyEntries) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveImportXMLDetectsZIPByMagicNotName(t *testing.T) {
	data := zipBytes(t, map[string][]byte{"Solution.xml": []byte(sampleXML)})
	path := filepath.Join(t.TempDir(), "upload.rar")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, format, err := ResolveImportXML(path, Limits{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatZIP {
		t.Fatalf("format=%q", format)
	}
	if string(got) != sampleXML {
		t.Fatalf("got %q", got)
	}
}

func TestResolveImportXMLZipReportsProgress(t *testing.T) {
	xmlBody := []byte(`<?xml version="1.0"?><Solution Name="demo">` + strings.Repeat("x", 2000) + `</Solution>`)
	data := zipBytes(t, map[string][]byte{"Solution.xml": xmlBody})
	path := writeTemp(t, data)

	var lastProcessed, lastTotal int64
	calls := 0
	got, format, err := ResolveImportXML(path, Limits{}, func(processed, total int64) {
		calls++
		lastProcessed = processed
		lastTotal = total
	})
	if err != nil {
		t.Fatal(err)
	}
	if format != FormatZIP {
		t.Fatalf("format=%q", format)
	}
	if len(got) != len(xmlBody) {
		t.Fatalf("len=%d", len(got))
	}
	if calls < 1 {
		t.Fatal("expected progress callbacks")
	}
	if lastTotal < 1 || lastProcessed < 1 {
		t.Fatalf("processed=%d total=%d", lastProcessed, lastTotal)
	}
}
