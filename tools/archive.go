package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"strings"
)

// ExtractArchiveFilenames attempts to extract filenames from archive data.
// It supports ZIP, TAR, and TAR.GZ formats.
// Returns a list of filenames found in the archive, or nil if not an archive or parsing fails.
func ExtractArchiveFilenames(data []byte, mimeType, filename string) []string {
	// Try to detect archive type from mime type or filename
	lowerFilename := strings.ToLower(filename)
	lowerMime := strings.ToLower(mimeType)

	switch {
	// Check gzip BEFORE zip since "application/gzip" contains "zip"
	case strings.Contains(lowerMime, "gzip") ||
		strings.HasSuffix(lowerFilename, ".tar.gz") ||
		strings.HasSuffix(lowerFilename, ".tgz"):
		return extractTarGzFilenames(data)

	case strings.Contains(lowerMime, "zip") ||
		strings.HasSuffix(lowerFilename, ".zip") ||
		strings.HasSuffix(lowerFilename, ".jar") ||
		strings.HasSuffix(lowerFilename, ".war") ||
		strings.HasSuffix(lowerFilename, ".ear"):
		return extractZipFilenames(data)

	case strings.Contains(lowerMime, "x-tar") ||
		strings.HasSuffix(lowerFilename, ".tar"):
		return extractTarFilenames(data)

	default:
		return nil
	}
}

// extractZipFilenames reads ZIP local file headers to extract filenames.
// ZIP central directory is at the end, but local headers precede each file.
func extractZipFilenames(data []byte) []string {
	// First try standard zip reader if we have enough data
	if names := tryStandardZipReader(data); names != nil {
		return names
	}

	// Fall back to parsing local file headers manually
	return parseZipLocalHeaders(data)
}

func tryStandardZipReader(data []byte) []string {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}

	var names []string
	for _, f := range reader.File {
		if f.Name != "" {
			names = append(names, f.Name)
		}
	}
	return names
}

// parseZipLocalHeaders parses ZIP local file headers to extract filenames.
// Local file header signature: 0x04034b50
func parseZipLocalHeaders(data []byte) []string {
	var names []string
	offset := 0

	for offset+30 <= len(data) {
		// Check for local file header signature (little-endian: 50 4b 03 04)
		if data[offset] != 0x50 || data[offset+1] != 0x4b ||
			data[offset+2] != 0x03 || data[offset+3] != 0x04 {
			break
		}

		// Read filename length (offset 26-27, little-endian)
		if offset+28 > len(data) {
			break
		}
		filenameLen := int(binary.LittleEndian.Uint16(data[offset+26 : offset+28]))

		// Read extra field length (offset 28-29, little-endian)
		if offset+30 > len(data) {
			break
		}
		extraLen := int(binary.LittleEndian.Uint16(data[offset+28 : offset+30]))

		// Read compressed size (offset 18-21, little-endian)
		compressedSize := int(binary.LittleEndian.Uint32(data[offset+18 : offset+22]))

		// Extract filename
		filenameStart := offset + 30
		filenameEnd := filenameStart + filenameLen
		if filenameEnd > len(data) {
			break
		}

		filename := string(data[filenameStart:filenameEnd])
		if filename != "" && !strings.HasSuffix(filename, "/") {
			names = append(names, filename)
		}

		// Move to next header
		offset = filenameEnd + extraLen + compressedSize
		if offset <= 0 || offset >= len(data) {
			break
		}
	}

	return names
}

// extractTarFilenames reads TAR headers to extract filenames.
func extractTarFilenames(data []byte) []string {
	reader := tar.NewReader(bytes.NewReader(data))
	var names []string

	for {
		header, err := reader.Next()
		if err != nil {
			break
		}

		if header.Name != "" && header.Typeflag != tar.TypeDir {
			names = append(names, header.Name)
		}
	}

	return names
}

// extractTarGzFilenames decompresses gzip and then reads TAR headers.
func extractTarGzFilenames(data []byte) []string {
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	var names []string

	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}

		if header.Name != "" && header.Typeflag != tar.TypeDir {
			names = append(names, header.Name)
		}
	}

	return names
}

// IsArchive checks if the given mime type or filename indicates an archive.
func IsArchive(mimeType, filename string) bool {
	lowerFilename := strings.ToLower(filename)
	lowerMime := strings.ToLower(mimeType)

	archiveMimes := []string{
		"application/zip",
		"application/x-zip",
		"application/x-zip-compressed",
		"application/x-tar",
		"application/gzip",
		"application/x-gzip",
		"application/x-compressed-tar",
	}

	for _, m := range archiveMimes {
		if strings.Contains(lowerMime, m) {
			return true
		}
	}

	archiveExtensions := []string{
		".zip", ".jar", ".war", ".ear",
		".tar", ".tar.gz", ".tgz",
		".tar.bz2", ".tbz2",
	}

	for _, ext := range archiveExtensions {
		if strings.HasSuffix(lowerFilename, ext) {
			return true
		}
	}

	return false
}
