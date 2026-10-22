package repository

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeDecodeGzipJSONRoundTrip(t *testing.T) {
	raw := json.RawMessage(`{"id":"sch-1","name":"Scheme","items":[1,2,3]}`)
	blob, encoding, err := EncodeGzipJSON(raw)
	if err != nil {
		t.Fatalf("EncodeGzipJSON: %v", err)
	}
	if encoding != EncodingGzipJSONV1 {
		t.Fatalf("encoding=%q want %q", encoding, EncodingGzipJSONV1)
	}
	if len(blob) == 0 {
		t.Fatal("blob empty")
	}
	decoded, err := DecodeStoredJSON(encoding, json.RawMessage(jsonbStub), blob)
	if err != nil {
		t.Fatalf("DecodeStoredJSON: %v", err)
	}
	if !bytes.Equal(decoded, raw) {
		t.Fatalf("round-trip mismatch: got %s want %s", decoded, raw)
	}
}

func TestDecodeStoredJSONLegacyJSONB(t *testing.T) {
	legacy := json.RawMessage(`{"legacy":true,"n":42}`)
	decoded, err := DecodeStoredJSON(EncodingJSONB, legacy, nil)
	if err != nil {
		t.Fatalf("DecodeStoredJSON jsonb: %v", err)
	}
	if !bytes.Equal(decoded, legacy) {
		t.Fatalf("legacy mismatch: got %s want %s", decoded, legacy)
	}
	decodedEmpty, err := DecodeStoredJSON("", nil, nil)
	if err != nil {
		t.Fatalf("DecodeStoredJSON empty encoding: %v", err)
	}
	if string(decodedEmpty) != jsonbStub {
		t.Fatalf("empty jsonb stub: %s", decodedEmpty)
	}
}

func TestDecodeStoredJSONGzipEmptyBlob(t *testing.T) {
	_, err := DecodeStoredJSON(EncodingGzipJSONV1, json.RawMessage(jsonbStub), nil)
	if err == nil {
		t.Fatal("expected empty blob error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEncodeGzipJSONEmptyUsesStub(t *testing.T) {
	blob, encoding, err := EncodeGzipJSON(nil)
	if err != nil {
		t.Fatalf("EncodeGzipJSON: %v", err)
	}
	if encoding != EncodingGzipJSONV1 {
		t.Fatalf("encoding=%q", encoding)
	}
	decoded, err := DecodeStoredJSON(encoding, json.RawMessage(jsonbStub), blob)
	if err != nil {
		t.Fatalf("DecodeStoredJSON: %v", err)
	}
	if string(decoded) != jsonbStub {
		t.Fatalf("decoded=%s want %s", decoded, jsonbStub)
	}
}

func TestDecodeStoredJSONRejectsUnknownEncoding(t *testing.T) {
	_, err := DecodeStoredJSON("unknown-v9", json.RawMessage(jsonbStub), []byte{1})
	if err == nil {
		t.Fatal("expected unsupported encoding error")
	}
}

func TestEncodeGzipJSONIsValidGzip(t *testing.T) {
	raw := []byte(`{"ok":true}`)
	blob, _, err := EncodeGzipJSON(raw)
	if err != nil {
		t.Fatalf("EncodeGzipJSON: %v", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer zr.Close()
	out := new(bytes.Buffer)
	if _, err := out.ReadFrom(zr); err != nil {
		t.Fatalf("gunzip: %v", err)
	}
	if !bytes.Equal(out.Bytes(), raw) {
		t.Fatalf("got %s want %s", out.Bytes(), raw)
	}
}
