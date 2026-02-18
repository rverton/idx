# idx - internal data explorer

idx is a tool for monitoring internal data sources for secrets and sensitive
information.

- No external tools required, self-contained binary
- Single binary, single (encrypted at rest) config file
- Local sqlite database for storing runs and findings

### Differentiation to other tools

Most secret detection tools are focused on a single task: scanning a source for secrets. They fall
short when it comes to everything around like source handling, scheduling, alerting, and managing
findings. idx aims to fill that gap by providing a complete solution for monitoring internal data
sources for secrets and sensitive information.

### Supported Targets

- Bitbucket Cloud
    - Repositories
- Bitbucket Data Center
    - Repositories
- Confluence Data Center
    - Pages and all versions
- Jira Data Center
    - Issues
    - Comments
    - Attachments (content, filenames and archive filenames)
- SMB file shares
    - Share enumeration
    - Recursive share file scan (filename and 5MB file content limit)

## Usage

```bash
idx [FLAGS] <subcommand>
```

### Commands

#### Configuration Management

```bash
idx config init
idx config verify
idx config targets-list
idx config encrypt
idx config decrypt
```

#### Start exploration run

```bash
idx run
idx run --repeat 30m
```

Runs the exploration against all configured targets. Results are stored in a local sqlite database.
With `--repeat <duration>`, idx waits for the given Go duration and starts the next run in a loop.

#### List Runs

```bash
idx list-runs
```

Lists all exploration runs from the database.

## Development

### Target Callbacks

Each target's `Explore` function receives two callbacks that decouple the target implementation from the core analysis and persistence logic.

#### Analyze Callback

```go
analyze func(content detect.Content)
```

Called for each piece of content discovered during exploration. The callback receives a `detect.Content` struct containing:

- `Key` - Unique identifier for the content (e.g., `repo:commithash:filepath`)
- `Data` - Raw content bytes to analyze
- `Location` - Breadcrumb trail for the content (e.g., `["bitbucket-cloud", "repo", "commit", "file"]`)

The core implementation runs detection rules against the content and logs findings.

#### Memory Callback

```go
type MemoryStore struct {
    Has func(key string) bool
    Set func(key string)
}
```

Provides persistent memory to avoid re-analyzing content across runs. Each target type defines its own key format appropriate for its content granularity (e.g., commits for git-based targets, file paths for file shares, etc.).

Usage pattern in target implementations:

```go
if memory.Has(key) {
    return nil  // skip already analyzed
}
// ... analyze content ...
memory.Set(key)
```

Both callbacks are created by the core `Explore` function and passed to each target, keeping targets stateless and testable.

### Integration testing

- For the supported Atlassian targets a 30d trial license can be generated from their website.
- Use `make testhelper/smb` to start a local SMB server with test data for development and testing.
