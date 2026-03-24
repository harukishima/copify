# copify

A CLI tool to copy objects between S3-compatible storage services.

Works with AWS S3, MinIO, Backblaze B2, DigitalOcean Spaces, and any other S3-compatible provider.

## Features

- Stream objects directly from source to destination (no temp files)
- Multipart upload for large files
- Configurable concurrency for parallel transfers
- Prefix rewriting (e.g., copy `logs/` to `backup/logs/`)
- Dry-run mode to preview operations
- Real-time progress reporting (object count, bytes, speed, ETA)
- Graceful shutdown on interrupt
- YAML config file support (with flag overrides)

## Installation

```bash
go install github.com/harukishima/copify@latest
```

Or build from source:

```bash
git clone https://github.com/harukishima/copify.git
cd copify
go build -o copify .
```

## Usage

```bash
copify copy \
  --src-endpoint https://s3.amazonaws.com \
  --src-bucket my-source \
  --src-access-key AKIA... \
  --src-secret-key ... \
  --src-region us-east-1 \
  --src-prefix logs/2024/ \
  --dst-endpoint https://play.min.io \
  --dst-bucket my-dest \
  --dst-access-key ... \
  --dst-secret-key ... \
  --dst-region us-east-1 \
  --dst-prefix backup/logs/ \
  --concurrency 10 \
  --part-size 10
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `""` | Path to YAML config file |
| `--src-endpoint` | *(required)* | Source S3 endpoint URL |
| `--src-bucket` | *(required)* | Source bucket name |
| `--src-access-key` | *(required)* | Source access key |
| `--src-secret-key` | *(required)* | Source secret key |
| `--src-region` | `us-east-1` | Source region |
| `--src-prefix` | `""` | Source key prefix |
| `--dst-endpoint` | *(required)* | Destination S3 endpoint URL |
| `--dst-bucket` | *(required)* | Destination bucket name |
| `--dst-access-key` | *(required)* | Destination access key |
| `--dst-secret-key` | *(required)* | Destination secret key |
| `--dst-region` | `us-east-1` | Destination region |
| `--dst-prefix` | `""` | Destination key prefix |
| `--concurrency` | `5` | Number of parallel transfers |
| `--part-size` | `10` | Multipart part size in MiB |
| `--dry-run` | `false` | List what would be copied without copying |

### Config File

Instead of passing all flags, you can use a YAML config file:

```yaml
source:
  endpoint: https://s3.amazonaws.com
  bucket: my-source
  access_key: AKIA...
  secret_key: ...
  region: us-east-1
  prefix: logs/2024/

destination:
  endpoint: https://play.min.io
  bucket: my-dest
  access_key: ...
  secret_key: ...
  region: us-east-1
  prefix: backup/logs/

concurrency: 10
part_size_mib: 10
```

```bash
copify copy --config config.yaml
```

Flags override config file values when both are provided:

```bash
copify copy --config config.yaml --src-prefix other/prefix/ --dry-run
```

### Dry Run

Preview what would be copied:

```bash
copify copy --dry-run \
  --src-endpoint https://s3.amazonaws.com --src-bucket my-source --src-access-key ... --src-secret-key ... \
  --dst-endpoint https://play.min.io --dst-bucket my-dest --dst-access-key ... --dst-secret-key ...
```

## License

MIT
