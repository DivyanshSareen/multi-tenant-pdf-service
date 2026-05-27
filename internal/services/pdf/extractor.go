// Package pdf provides PDF text extraction utilities.
package pdf

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"unicode"

	"github.com/sirupsen/logrus"
)

// Extractor extracts plain text from PDF files.
type Extractor struct {
	log *logrus.Logger
}

// NewExtractor creates a new PDF Extractor.
func NewExtractor(log *logrus.Logger) *Extractor {
	return &Extractor{log: log}
}

// Extract pulls plain text from a PDF file and estimates the page count.
// It first tries pdftotext (poppler-utils); if that binary is not available it
// falls back to a raw byte scan for printable ASCII — useful in minimal containers.
func (e *Extractor) Extract(filePath string) (text string, pageCount int, err error) {
	text, pageCount, err = e.extractViaPdftotext(filePath)
	if err != nil {
		e.log.WithError(err).Warn("pdftotext unavailable, falling back to raw extraction")
		text, pageCount, err = e.extractRaw(filePath)
	}
	return
}

// extractViaPdftotext shells out to pdftotext (poppler-utils) which produces
// high-quality text with proper whitespace. The "-" argument sends output to stdout.
func (e *Extractor) extractViaPdftotext(filePath string) (string, int, error) {
	// First get the text
	textCmd := exec.Command("pdftotext", filePath, "-")
	var textOut, textErr bytes.Buffer
	textCmd.Stdout = &textOut
	textCmd.Stderr = &textErr

	if err := textCmd.Run(); err != nil {
		return "", 0, fmt.Errorf("pdftotext: %w (stderr: %s)", err, textErr.String())
	}

	text := textOut.String()
	pageCount := estimatePageCount(text)

	// Try to get accurate page count via pdfinfo if available.
	if infoCount, err := getPageCountViaPdfinfo(filePath); err == nil && infoCount > 0 {
		pageCount = infoCount
	}

	e.log.WithFields(logrus.Fields{
		"file":       filePath,
		"char_count": len(text),
		"pages":      pageCount,
	}).Info("extracted text via pdftotext")

	return text, pageCount, nil
}

// getPageCountViaPdfinfo calls pdfinfo and parses the "Pages:" line.
func getPageCountViaPdfinfo(filePath string) (int, error) {
	cmd := exec.Command("pdfinfo", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, err
	}

	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				var n int
				fmt.Sscanf(parts[1], "%d", &n)
				return n, nil
			}
		}
	}
	return 0, fmt.Errorf("pages field not found in pdfinfo output")
}

// extractRaw reads the file bytes and collects printable ASCII runs as a last resort.
// This loses formatting but preserves readable text from simple PDFs.
func (e *Extractor) extractRaw(filePath string) (string, int, error) {
	data, err := exec.Command("cat", filePath).Output()
	if err != nil {
		return "", 0, fmt.Errorf("reading file %q: %w", filePath, err)
	}

	var sb strings.Builder
	for _, b := range data {
		r := rune(b)
		if unicode.IsPrint(r) || r == '\n' || r == '\t' {
			sb.WriteRune(r)
		}
	}

	text := sb.String()
	// Collapse long runs of whitespace that result from binary sections.
	text = strings.Join(strings.Fields(text), " ")

	pageCount := estimatePageCount(text)
	e.log.WithFields(logrus.Fields{
		"file":  filePath,
		"pages": pageCount,
	}).Warn("used raw fallback extraction — text quality may be poor")

	return text, pageCount, nil
}

// estimatePageCount counts form-feed characters (\f) as page breaks, defaulting to 1.
func estimatePageCount(text string) int {
	count := strings.Count(text, "\f") + 1
	if count < 1 {
		return 1
	}
	return count
}
