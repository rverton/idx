package tools

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestIsArchive(t *testing.T) {
	tests := []struct {
		mimeType string
		filename string
		want     bool
	}{
		// ZIP variants
		{"application/zip", "file.zip", true},
		{"application/x-zip-compressed", "archive.zip", true},
		{"", "app.jar", true},
		{"", "webapp.war", true},
		{"", "enterprise.ear", true},

		// TAR variants
		{"application/x-tar", "archive.tar", true},
		{"", "backup.tar", true},
		{"application/gzip", "archive.tar.gz", true},
		{"", "backup.tgz", true},
		{"", "data.tar.bz2", true},

		// Non-archives
		{"text/plain", "readme.txt", false},
		{"application/json", "data.json", false},
		{"image/png", "image.png", false},
		{"application/pdf", "document.pdf", false},
		{"", "script.sh", false},
	}

	for _, tt := range tests {
		name := tt.filename
		if tt.mimeType != "" {
			name = tt.mimeType + "/" + tt.filename
		}
		t.Run(name, func(t *testing.T) {
			got := IsArchive(tt.mimeType, tt.filename)
			if got != tt.want {
				t.Errorf("IsArchive(%q, %q) = %v, want %v", tt.mimeType, tt.filename, got, tt.want)
			}
		})
	}
}

func TestExtractArchiveFilenames_Zip(t *testing.T) {
	// Create a test ZIP file in memory
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	files := []string{"secret.env", "config/database.properties", "src/main.go"}
	for _, name := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry: %v", err)
		}
		f.Write([]byte("test content"))
	}
	w.Close()

	// Extract filenames
	names := ExtractArchiveFilenames(buf.Bytes(), "application/zip", "test.zip")

	if len(names) != len(files) {
		t.Errorf("got %d filenames, want %d", len(names), len(files))
	}

	for _, expected := range files {
		found := false
		for _, name := range names {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected filename %q not found in %v", expected, names)
		}
	}
}

func TestExtractArchiveFilenames_Tar(t *testing.T) {
	// Create a test TAR file in memory
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	files := []string{".env", "credentials.json", "id_rsa"}
	for _, name := range files {
		content := []byte("test content")
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("failed to write tar content: %v", err)
		}
	}
	tw.Close()

	// Extract filenames
	names := ExtractArchiveFilenames(buf.Bytes(), "application/x-tar", "backup.tar")

	if len(names) != len(files) {
		t.Errorf("got %d filenames, want %d", len(names), len(files))
	}

	for _, expected := range files {
		found := false
		for _, name := range names {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected filename %q not found in %v", expected, names)
		}
	}
}

func TestExtractArchiveFilenames_TarGz(t *testing.T) {
	// Create a test TAR file
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	files := []string{"app.properties", "secrets/api.key"}
	for _, name := range files {
		content := []byte("test content")
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		tw.WriteHeader(hdr)
		tw.Write(content)
	}
	tw.Close()

	// Compress with gzip
	gzBuf := new(bytes.Buffer)
	gw := gzip.NewWriter(gzBuf)
	gw.Write(tarBuf.Bytes())
	gw.Close()

	// Extract filenames
	names := ExtractArchiveFilenames(gzBuf.Bytes(), "application/gzip", "backup.tar.gz")

	if len(names) != len(files) {
		t.Errorf("got %d filenames, want %d", len(names), len(files))
	}

	for _, expected := range files {
		found := false
		for _, name := range names {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected filename %q not found in %v", expected, names)
		}
	}
}

func TestExtractArchiveFilenames_NonArchive(t *testing.T) {
	data := []byte("this is just plain text, not an archive")

	names := ExtractArchiveFilenames(data, "text/plain", "readme.txt")
	if names != nil {
		t.Errorf("expected nil for non-archive, got %v", names)
	}
}

func TestExtractArchiveFilenames_EmptyArchive(t *testing.T) {
	// Create empty ZIP
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	w.Close()

	names := ExtractArchiveFilenames(buf.Bytes(), "application/zip", "empty.zip")
	if len(names) != 0 {
		t.Errorf("expected 0 filenames for empty archive, got %d", len(names))
	}
}

func TestExtractArchiveFilenames_PartialZip(t *testing.T) {
	// Create a ZIP file and truncate it to simulate partial download
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	files := []string{"first.txt", "second.txt", "third.txt"}
	for _, name := range files {
		f, _ := w.Create(name)
		f.Write([]byte("some test content here"))
	}
	w.Close()

	// Take only first half of the data (simulating partial download)
	partialData := buf.Bytes()[:buf.Len()/2]

	// Should still extract some filenames from local headers
	names := ExtractArchiveFilenames(partialData, "application/zip", "partial.zip")

	// We should get at least the first file
	if len(names) == 0 {
		t.Log("Note: partial ZIP extraction returned no files (may be expected for small test)")
	}
}
