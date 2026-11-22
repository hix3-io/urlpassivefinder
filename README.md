# URLPassiveFinder

[![Release](https://img.shields.io/github/v/release/hix3-io/urlpassivefinder)](https://github.com/hix3-io/urlpassivefinder/releases/latest)
[![CI](https://github.com/hix3-io/urlpassivefinder/actions/workflows/ci.yml/badge.svg)](https://github.com/hix3-io/urlpassivefinder/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.22-blue)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

High-performance passive URL discovery tool that combines the best techniques from gau, waybackurls, waymore, urlfinder, and sigurlfind3r.

## Features

- 🚀 **Fast** - Concurrent fetching with worker pools
- 🔍 **Comprehensive** - 10 data sources (Wayback, CommonCrawl, OTX, URLScan, VirusTotal, GitHub, crt.sh, Shodan, SecurityTrails, Censys)
- 💾 **Memory Efficient** - Streaming architecture, no bulk loading
- 🎯 **Smart Filtering** - Extension, MIME type, and scope filtering
- 🔄 **Resilient** - Per-source error handling and rate limiting
- 📊 **Multiple Output Formats** - Text, JSON, JSONL

## Installation

```bash
go install github.com/hix3-io/urlpassivefinder@latest
```

## Quick Start

```bash
# Single domain
urlpassivefinder -d example.com

# Multiple domains from file
urlpassivefinder -l domains.txt -o results.txt

# Specific providers only
urlpassivefinder -d example.com -p wayback,commoncrawl

# JSON output with filtering
urlpassivefinder -d example.com --json --filter js,css,png
```
## Demo 

<img width="800" height="443" alt="ezgif-79dd2d5fbd88db69" src="https://github.com/user-attachments/assets/f5cbd142-adeb-4220-9e86-906caa2e34e3" />


## Configuration

Create `~/.urlpassivefinder/config.yaml`:

```yaml
providers:
  wayback:
    enabled: true
    rate_limit: 10
  urlscan:
    api_key: "your-key-here"
    
filters:
  extensions: [.jpg, .png, .css, .js]
```

API keys can also (or only) be provided through environment variables —
no config file required:

| Environment variable | Provider |
|----------------------|----------|
| `URLSCAN_API_KEY` | URLScan |
| `VT_API_KEY` | VirusTotal |
| `GITHUB_TOKEN` | GitHub |
| `OTX_API_KEY` | AlienVault OTX |
| `SHODAN_API_KEY` | Shodan |
| `SECURITYTRAILS_API_KEY` | SecurityTrails |
| `CENSYS_API_ID` / `CENSYS_API_SECRET` | Censys |

## Providers

| Provider | API Key | Default Rate Limit |
|----------|---------|-------------------|
| Wayback Machine | No | 10 req/s |
| Common Crawl | No | 5 req/s |
| AlienVault OTX | Optional | 10 req/s |
| URLScan | Optional | 2 req/s |
| VirusTotal | Required | 4 req/min |
| GitHub | Optional | 30 req/min |
| crt.sh | No | 10 req/s |
| Shodan | Required | — |
| SecurityTrails | Required | — |
| Censys | Required (ID + secret) | — |

## Performance

- **Memory**: < 100MB for 1M URLs
- **Speed**: > 50k URLs/second processing
- **Concurrency**: 10 workers by default (configurable)

## Documentation

- [Passive URL discovery analysis](docs/PASSIVE_URL_DISCOVERY_ANALYSIS.md) — methodology and data-source evaluation
- [Techniques](docs/BEST_TECHNIQUES_URL_DISCOVERY.md) — the best techniques harvested from the ecosystem
- [Tool comparison](docs/URL_COMPARISON.md) — how this tool compares to gau, waymore and friends

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

This tool combines the best techniques from:
- [gau](https://github.com/lc/gau) by @lc
- [waybackurls](https://github.com/tomnomnom/waybackurls) by @tomnomnom  
- [waymore](https://github.com/xnl-h4ck3r/waymore) by @xnl-h4ck3r
- [urlfinder](https://github.com/projectdiscovery/urlfinder) by @projectdiscovery
- [sigurlfind3r](https://github.com/signedsecurity/sigurlfind3r) by @signedsecurity
