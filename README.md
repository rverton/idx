# idx - internal data explorer

idx is a tool for monitoring internal data sources for secrets and sensitive
information.

- No external tools required, self-contained binary
- Single binary, single (encrypted at rest) config file

Most secret detection tools are focused on a single task: scanning a source for
secrets. They fall short when it comes to everything around like source credential
handling, scheduling, alerting, and managing findings. idx aims to fill that gap
by providing a complete solution for monitoring internal data sources for secrets
and sensitive information.

## Usage

First, initialize the example configuration file:

```bash
idx config init
```

After adjusting the configuration file, you can verify the configuration:

```bash
idx config verify
```

For production use, it is recommended to encrypt the configuration file. To use
encrypted configuration files, the password must be provided after starting the
daemon.

```bash
idx config encrypt
```

Then, you can start the idx server:
```bash
idx
```

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
