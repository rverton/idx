package smb

import (
	"bytes"
	"context"
	"fmt"
	"idx/detect"
	"idx/tools"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"slices"

	"github.com/jfjallid/go-smb/smb"
	"github.com/jfjallid/go-smb/smb/dcerpc"
	"github.com/jfjallid/go-smb/smb/dcerpc/mssrvs"
	"github.com/jfjallid/go-smb/spnego"
)

const maxFileSize = 1 * 1024 * 1024 // 1MB

var textExtensions = []string{
	".txt", ".log", ".md", ".json", ".xml", ".yml", ".yaml",
	".sh", ".bash",
	".sql", ".conf", ".cfg", ".ini", ".properties",
	".env", ".htaccess", ".htpasswd",
	".csv", ".tsv",
	".js", ".ts",
	".py", ".rb", ".pl", ".php", ".go", ".java", ".c", ".cpp", ".h",
	".rs", ".swift", ".kt", ".scala", ".clj",
	".ps1", ".psm1", ".psd1", ".bat", ".cmd",
	".doc", ".docx", ".rtf", ".xls", ".xlsx", ".pdf",
}

type Client struct {
	session  *smb.Connection
	hostname string
}

func NewClient(hostname string, port int, user, password, domain string) (*Client, error) {
	if hostname == "" {
		return nil, fmt.Errorf("hostname is required for SMB target")
	}

	if user == "" || password == "" {
		return nil, fmt.Errorf("ntlmUser and ntlmPassword are required for SMB target")
	}

	if port == 0 {
		port = 445
	}

	options := smb.Options{
		Host: hostname,
		Port: port,
		Initiator: &spnego.NTLMInitiator{
			User:     user,
			Password: password,
			Domain:   domain,
		},
	}

	session, err := smb.NewConnection(options)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SMB server %s:%d: %w", hostname, port, err)
	}

	return &Client{
		session:  session,
		hostname: hostname,
	}, nil
}

func (c *Client) Close() {
	if c.session != nil {
		c.session.Close()
	}
}

func (c *Client) VerifyConnection(ctx context.Context) error {
	shares, err := c.EnumerateShares()
	if err != nil {
		return fmt.Errorf("SMB connection verification failed: %w", err)
	}
	slog.Debug("SMB connection verified", "hostname", c.hostname, "shares", len(shares))
	return nil
}

func (c *Client) EnumerateShares() ([]string, error) {
	err := c.session.TreeConnect("IPC$")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IPC$: %w", err)
	}
	defer c.session.TreeDisconnect("IPC$")

	f, err := c.session.OpenFile("IPC$", mssrvs.MSRPCSrvSvcPipe)
	if err != nil {
		return nil, fmt.Errorf("failed to open SRVSVC pipe: %w", err)
	}
	defer f.CloseFile()

	bind, err := dcerpc.Bind(f, mssrvs.MSRPCUuidSrvSvc, mssrvs.MSRPCSrvSvcMajorVersion, mssrvs.MSRPCSrvSvcMinorVersion, dcerpc.MSRPCUuidNdr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind SRVSVC: %w", err)
	}

	rpccon := mssrvs.NewRPCCon(bind)
	shareInfos, err := rpccon.NetShareEnumAll(c.hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate shares: %w", err)
	}

	var shares []string
	for _, share := range shareInfos {
		// Skip IPC$ as it's only for RPC, not file access
		if strings.ToUpper(share.Name) == "IPC$" {
			continue
		}
		shares = append(shares, share.Name)
	}

	return shares, nil
}

func (c *Client) readFile(share, path string) ([]byte, error) {
	var buf bytes.Buffer
	limitedWriter := &limitedWriter{w: &buf, remaining: maxFileSize}

	err := c.session.RetrieveFile(share, path, 0, limitedWriter.Write)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type limitedWriter struct {
	w         io.Writer
	remaining int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.remaining <= 0 {
		return len(p), nil // Silently discard excess data
	}

	toWrite := p
	if int64(len(p)) > lw.remaining {
		toWrite = p[:lw.remaining]
	}

	n, err := lw.w.Write(toWrite)
	lw.remaining -= int64(n)
	return len(p), err
}

func isTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))

	if slices.Contains(textExtensions, ext) {
		return true
	}

	// Files without extension that are commonly text
	basename := strings.ToLower(filepath.Base(filename))
	textFiles := []string{
		"readme", "makefile", "dockerfile", "vagrantfile", ".env",
	}
	return slices.Contains(textFiles, basename)
}

// filetimeToUnix converts Windows FILETIME (100-nanosecond intervals since Jan 1, 1601) to Unix timestamp
func filetimeToUnix(ft uint64) int64 {
	// Windows epoch is Jan 1, 1601, Unix epoch is Jan 1, 1970
	// Difference is 116444736000000000 100-nanosecond intervals
	const epochDiff = 116444736000000000
	if ft < epochDiff {
		return 0
	}
	return int64((ft - epochDiff) / 10000000)
}

func pathDepth(path string) int {
	if path == "" {
		return 0
	}

	normalized := normalizePath(path)
	normalized = strings.Trim(normalized, "/")

	if normalized == "" {
		return 0
	}

	return strings.Count(normalized, "/") + 1
}

type walkOpts struct {
	share                string
	dir                  string
	currentDepth         int
	maxDepth             int
	folderCacheDepth     int
	folderRescanDuration time.Duration
	memory               detect.MemoryStore
	targetName           string
	hostname             string
}

func normalizePath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func (c *Client) walkFiles(opts walkOpts, fn func(file smb.SharedFile, depth int)) error {
	normalizedDir := normalizePath(opts.dir)

	if opts.folderCacheDepth > 0 && opts.currentDepth >= opts.folderCacheDepth {
		folderKey := fmt.Sprintf("smb/%s/%s/%s/folder:%s", opts.targetName, opts.hostname, opts.share, normalizedDir)

		if ts, exists := opts.memory.GetTimestamp(folderKey); exists {
			age := time.Since(time.Unix(ts, 0))
			if age < opts.folderRescanDuration {
				slog.Debug("skipping cached folder", "path", normalizedDir, "age", age, "threshold", opts.folderRescanDuration)
				return nil
			}
		}
	}

	files, err := c.session.ListDirectory(opts.share, opts.dir, "*")
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.Name == "." || file.Name == ".." {
			continue
		}

		fn(file, opts.currentDepth)

		if file.IsDir && !file.IsJunction && opts.currentDepth < opts.maxDepth {
			subOpts := opts
			subOpts.dir = file.FullPath
			subOpts.currentDepth = opts.currentDepth + 1

			if err := c.walkFiles(subOpts, fn); err != nil {
				slog.Debug("failed to list subdirectory", "share", opts.share, "path", file.FullPath, "error", err)
				continue
			}
		}
	}

	if opts.folderCacheDepth > 0 && opts.currentDepth >= opts.folderCacheDepth {
		folderKey := fmt.Sprintf("smb/%s/%s/%s/folder:%s", opts.targetName, opts.hostname, opts.share, normalizedDir)
		opts.memory.Set(folderKey)
	}

	return nil
}

func Explore(
	ctx context.Context,
	analyze func(content detect.Content),
	analyzeFilename func(filename, contentKey string, location []string),
	memory detect.MemoryStore,
	name string,
	hostname string,
	port int,
	user, password, domain string,
	maxRecursiveDepth int,
	folderCacheDepth int,
	folderRescanDuration time.Duration,
) error {
	client, err := NewClient(hostname, port, user, password, domain)
	if err != nil {
		return fmt.Errorf("smb: %w", err)
	}
	defer client.Close()

	shares, err := client.EnumerateShares()
	if err != nil {
		return fmt.Errorf("smb: failed to enumerate shares: %w", err)
	}

	slog.Info("smb shares enumerated", "target", name, "hostname", hostname, "count", len(shares))

	for _, share := range shares {
		err := client.session.TreeConnect(share)
		if err != nil {
			slog.Error("smb: failed to connect to share", "share", share, "error", err)
			continue
		}

		opts := walkOpts{
			share:                share,
			dir:                  "",
			currentDepth:         0,
			maxDepth:             maxRecursiveDepth,
			folderCacheDepth:     folderCacheDepth,
			folderRescanDuration: folderRescanDuration,
			memory:               memory,
			targetName:           name,
			hostname:             hostname,
		}

		var fileCount int

		err = client.walkFiles(opts, func(file smb.SharedFile, depth int) {
			if file.IsDir {
				return
			}

			fileCount++

			useFolderCache := folderCacheDepth > 0 && depth >= folderCacheDepth

			if !useFolderCache {
				modifiedAt := filetimeToUnix(file.LastWriteTime)
				memoryKey := fmt.Sprintf("smb/%s/%s/%s/%s/%d", name, hostname, share, normalizePath(file.FullPath), modifiedAt)

				if memory.Has(memoryKey) {
					return
				}
			}

			location := []string{
				"smb",
				hostname,
				share,
				file.FullPath,
			}
			contentKey := fmt.Sprintf("%s:%s", share, file.FullPath)

			analyzeFilename(file.Name, contentKey, location)

			if tools.IsArchive("", file.Name) {
				data, err := client.readFile(share, file.FullPath)
				if err != nil {
					slog.Debug("smb: failed to download archive", "share", share, "path", file.FullPath, "error", err)
				} else {
					archivedFiles := tools.ExtractArchiveFilenames(data, "", file.Name)
					slog.Debug("smb: extracted filenames from archive", "share", share, "path", file.FullPath, "count", len(archivedFiles))

					for _, archivedFile := range archivedFiles {
						archivedLocation := []string{
							"smb",
							hostname,
							share,
							file.FullPath,
							archivedFile,
						}
						analyzeFilename(archivedFile, contentKey, archivedLocation)
					}
				}
			} else if isTextFile(file.Name) && file.Size <= maxFileSize {
				data, err := client.readFile(share, file.FullPath)
				if err != nil {
					slog.Debug("smb: failed to read file", "share", share, "path", file.FullPath, "error", err)
				} else {
					content := detect.Content{
						Key:      contentKey,
						Data:     data,
						Location: location,
					}
					analyze(content)
					slog.Debug("analyzed file content", "share", share, "path", file.FullPath)
				}
			}

			if !useFolderCache {
				modifiedAt := filetimeToUnix(file.LastWriteTime)
				memoryKey := fmt.Sprintf("smb/%s/%s/%s/%s/%d", name, hostname, share, normalizePath(file.FullPath), modifiedAt)
				memory.Set(memoryKey)
			}
		})

		if err != nil {
			slog.Error("smb: failed to walk files", "share", share, "error", err)
			client.session.TreeDisconnect(share)
			continue
		}

		slog.Info("smb files processed", "target", name, "share", share, "count", fileCount)

		client.session.TreeDisconnect(share)
	}

	return nil
}
