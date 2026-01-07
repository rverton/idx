# Targets

This document lists all available targets and their key formats.

## Key Types

**Content Key**: A short, human-readable identifier for a piece of content. Used in findings to identify where a secret was detected. Designed to be stable and readable in reports.

**Memory Key**: An internal identifier used to track what has already been analyzed. Includes additional context (target name, timestamps) to support incremental scanning and detect when content has been updated. Stored in the database to avoid re-processing unchanged content.

## bitbucket-cloud

Analyzes git commit history from Bitbucket Cloud repositories.

| Field | Format |
|-------|--------|
| Content Key | `{repo}:{commitHash[:8]}:{filePath}` |
| Memory Key | `bitbucket-cloud/{targetName}/{repo}/{commitHash}` |

**Example:**
- Content Key: `myworkspace/myrepo:a1b2c3d4:src/config.go`
- Memory Key: `bitbucket-cloud/prod/myworkspace/myrepo/a1b2c3d4e5f6...`

## bitbucket-dc

Analyzes git commit history from Bitbucket Data Center repositories.

| Field | Format |
|-------|--------|
| Content Key | `{projectKey}/{repoSlug}:{commitHash[:8]}:{filePath}` |
| Memory Key | `bitbucket-dc/{targetName}/{projectKey}/{repoSlug}/{commitHash}` |

**Example:**
- Content Key: `PROJ/myrepo:a1b2c3d4:src/config.go`
- Memory Key: `bitbucket-dc/prod/PROJ/myrepo/a1b2c3d4e5f6...`

## confluence-dc

Analyzes page content (including version history) from Confluence Data Center.

| Field | Format |
|-------|--------|
| Content Key | `{spaceKey}:{pageID}:v{version}` |
| Memory Key | `confluence-dc/{targetName}/{spaceKey}/{pageID}/v{version}` |

**Example:**
- Content Key: `TEAM:12345678:v3`
- Memory Key: `confluence-dc/prod/TEAM/12345678/v3`

## jira-dc

Analyzes issue descriptions, comments, and attachments from Jira Data Center.

### Issue Description

| Field | Format |
|-------|--------|
| Content Key | `{projectKey}:{issueKey}:description` |
| Memory Key | `jira-dc/{targetName}/{projectKey}/{issueKey}/description/{updated}` |

**Example:**
- Content Key: `PROJ:PROJ-123:description`
- Memory Key: `jira-dc/prod/PROJ/PROJ-123/description/2024-01-15T10:30:00.000+0000`

### Issue Comment

| Field | Format |
|-------|--------|
| Content Key | `{projectKey}:{issueKey}:comment:{commentID}` |
| Memory Key | `jira-dc/{targetName}/{projectKey}/{issueKey}/comment/{commentID}/{updated}` |

**Example:**
- Content Key: `PROJ:PROJ-123:comment:10001`
- Memory Key: `jira-dc/prod/PROJ/PROJ-123/comment/10001/2024-01-15T10:30:00.000+0000`

### Issue Attachment

| Field | Format |
|-------|--------|
| Content Key | `{projectKey}:{issueKey}:attachment:{attachmentID}` |
| Memory Key | `jira-dc/{targetName}/{projectKey}/{issueKey}/attachment/{attachmentID}` |

**Example:**
- Content Key: `PROJ:PROJ-123:attachment:20001`
- Memory Key: `jira-dc/prod/PROJ/PROJ-123/attachment/20001`

Attachments are analyzed for:
- Filename patterns (all attachments)
- Text content (text/* mime types, limited to 1MB)
- Archived filenames (ZIP, TAR, TAR.GZ - filenames extracted and analyzed)

## smb

Analyzes files from SMB/CIFS fileshares using NTLM authentication.

### File (depth < folderCacheDepth)

| Field | Format |
|-------|--------|
| Content Key | `{share}:{filePath}` |
| Memory Key | `smb/{targetName}/{hostname}/{share}/{filePath}/{modifiedAt}` |

**Example:**
- Content Key: `Documents:reports/2024/annual.txt`
- Memory Key: `smb/prod/fileserver.local/Documents/reports/2024/annual.txt/1704067200`

### Folder (depth >= folderCacheDepth)

At folder depth >= `folderCacheDepth`, folders are tracked instead of individual files to reduce database size.

| Field | Format |
|-------|--------|
| Memory Key | `smb/{targetName}/{hostname}/{share}/folder:{folderPath}` |

**Example:**
- Memory Key: `smb/prod/fileserver.local/Documents/folder:reports/2024/quarterly`

Folders are re-scanned when `folderRescanDuration` has elapsed since the last scan.

### Configuration

| Option | Default | Description |
|--------|---------|-------------|
| `folderCacheDepth` | 2 | Depth threshold for folder-level caching (0 disables) |
| `folderRescanDuration` | 72h | Duration before re-scanning cached folders |

Files are analyzed for:
- Filename patterns (all files)
- Text content (common text extensions like .txt, .json, .xml, .yml, .conf, .env, etc., limited to 1MB)
- Archived filenames (ZIP, TAR, TAR.GZ - filenames extracted and analyzed)

Note: IPC$ is automatically excluded as it's used for RPC communication, not file access.
