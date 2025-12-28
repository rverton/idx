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
