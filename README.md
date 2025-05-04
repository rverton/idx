# idx - internal data explorer

idx is a tool for exploring and monitoring data from remote sources.

## Usage

First, initialize the example configuration file:
```bash
idx config init
```

After adjusting the configuration file, you can verify the configuration:
```bash
idx config verify
```

If the configuration is valid, encrypt the configuration:
```bash
idx config encrypt
```

This will encrypt the configuration file and move it to `config.ini.enc`.

Then, you can start the idx server:
```bash
idx daemon start
```

