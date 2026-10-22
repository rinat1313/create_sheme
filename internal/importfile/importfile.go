package importfile

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	solutionXMLName    = "Solution.xml"
	defaultMaxEntries  = 256
	defaultMaxXMLBytes = int64(1 << 30)
	sniffSize          = 64
)

const (
	FormatXML = "xml"
	FormatZIP = "zip"
)

var (
	ErrSolutionXMLNotFound = errors.New("Solution.xml not found in archive")
	ErrInvalidEntryPath    = errors.New("invalid archive entry path")
	ErrEncrypted           = errors.New("encrypted archives are not supported")
	ErrTooManyEntries      = errors.New("archive has too many entries")
	ErrXMLTooLarge         = errors.New("extracted Solution.xml exceeds size limit")
	ErrUnsupportedFormat   = errors.New("unsupported import format")
)

type Limits struct {
	MaxXMLBytes int64
	MaxEntries  int
}

type ProgressFunc func(processed, total int64)

func NormalizeLimits(limits Limits) Limits {
	if limits.MaxXMLBytes < 1 {
		limits.MaxXMLBytes = defaultMaxXMLBytes
	}
	if limits.MaxEntries < 1 {
		limits.MaxEntries = defaultMaxEntries
	}
	return limits
}

func ResolveImportXML(filePath string, limits Limits, onProgress ProgressFunc) (data []byte, format string, err error) {
	start := time.Now()
	limits = NormalizeLimits(limits)
	defer func() {
		slog.Info("ResolveImportXML done",
			"path", filePath,
			"format", format,
			"xml_bytes", len(data),
			"duration_ms", time.Since(start).Milliseconds(),
			"err", err,
		)
	}()
	slog.Debug("ResolveImportXML start", "path", filePath)

	format, err = sniffFileFormat(filePath)
	if err != nil {
		return nil, format, err
	}
	switch format {
	case FormatXML:
		data, err = readXMLFile(filePath, limits.MaxXMLBytes)
		return data, format, err
	case FormatZIP:
		data, err = extractSolutionXMLFromZIP(filePath, limits, onProgress)
		return data, format, err
	default:
		return nil, format, ErrUnsupportedFormat
	}
}

func SniffFormat(filePath string) (format string, err error) {
	return sniffFileFormat(filePath)
}

func sniffFileFormat(filePath string) (format string, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, sniffSize)
	n, readErr := io.ReadFull(f, buf)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		return "", readErr
	}
	buf = buf[:n]
	if isZIPMagic(buf) {
		return FormatZIP, nil
	}
	if isRARMagic(buf) {
		return "", ErrUnsupportedFormat
	}
	if looksLikeXML(buf) {
		return FormatXML, nil
	}
	return "", ErrUnsupportedFormat
}

func isZIPMagic(buf []byte) bool {
	if len(buf) < 4 {
		return false
	}
	if buf[0] != 'P' || buf[1] != 'K' {
		return false
	}
	return (buf[2] == 0x03 && buf[3] == 0x04) ||
		(buf[2] == 0x05 && buf[3] == 0x06) ||
		(buf[2] == 0x07 && buf[3] == 0x08)
}

func isRARMagic(buf []byte) bool {
	return len(buf) >= 4 && buf[0] == 'R' && buf[1] == 'a' && buf[2] == 'r' && buf[3] == '!'
}

func looksLikeXML(buf []byte) bool {
	if len(buf) >= 3 && buf[0] == 0xEF && buf[1] == 0xBB && buf[2] == 0xBF {
		buf = buf[3:]
	}
	i := 0
	for i < len(buf) && unicode.IsSpace(rune(buf[i])) {
		i++
	}
	return i < len(buf) && buf[i] == '<'
}

func readXMLFile(filePath string, maxBytes int64) ([]byte, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxBytes {
		return nil, ErrXMLTooLarge
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readLimited(f, maxBytes, 0, nil)
}

func extractSolutionXMLFromZIP(zipPath string, limits Limits, onProgress ProgressFunc) ([]byte, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if len(r.File) > limits.MaxEntries {
		return nil, ErrTooManyEntries
	}

	var solution *zip.File
	for _, f := range r.File {
		if f.Flags&0x1 != 0 {
			return nil, ErrEncrypted
		}
		cleaned, pathErr := normalizeArchiveEntry(f.Name)
		if pathErr != nil {
			return nil, pathErr
		}
		if cleaned == "" {
			continue
		}
		if path.Base(cleaned) == solutionXMLName && cleaned != solutionXMLName {
			return nil, ErrInvalidEntryPath
		}
		if cleaned != solutionXMLName {
			continue
		}
		if solution != nil {
			return nil, ErrInvalidEntryPath
		}
		solution = f
	}
	if solution == nil {
		return nil, ErrSolutionXMLNotFound
	}
	if solution.UncompressedSize64 > uint64(limits.MaxXMLBytes) {
		return nil, ErrXMLTooLarge
	}
	rc, err := solution.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	total := int64(solution.UncompressedSize64)
	if total < 1 {
		total = 1
	}
	if onProgress != nil {
		onProgress(0, total)
	}
	return readLimited(rc, limits.MaxXMLBytes, total, onProgress)
}

func normalizeArchiveEntry(name string) (string, error) {
	n := filepath.ToSlash(name)
	n = strings.TrimSpace(n)
	if n == "" || strings.HasSuffix(n, "/") {
		return "", nil
	}
	if path.IsAbs(n) || strings.HasPrefix(n, "/") {
		return "", ErrInvalidEntryPath
	}
	for _, part := range strings.Split(n, "/") {
		if part == ".." {
			return "", ErrInvalidEntryPath
		}
	}
	cleaned := path.Clean(n)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", ErrInvalidEntryPath
	}
	return cleaned, nil
}

func readLimited(r io.Reader, maxBytes, total int64, onProgress ProgressFunc) ([]byte, error) {
	var buf bytes.Buffer
	cr := &countingReader{r: io.LimitReader(r, maxBytes+1), total: total, onProgress: onProgress}
	written, err := buf.ReadFrom(cr)
	if err != nil {
		return nil, err
	}
	if written > maxBytes {
		return nil, ErrXMLTooLarge
	}
	if onProgress != nil && total > 0 {
		onProgress(written, total)
	}
	return buf.Bytes(), nil
}

type countingReader struct {
	r          io.Reader
	n          int64
	total      int64
	onProgress ProgressFunc
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.n += int64(n)
		if c.onProgress != nil && c.total > 0 {
			processed := c.n
			if processed > c.total {
				processed = c.total
			}
			c.onProgress(processed, c.total)
		}
	}
	return n, err
}
