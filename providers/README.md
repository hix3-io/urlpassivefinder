# URLPassiveFinder Providers

## 📁 Structure

Each provider is in its own file for better maintainability:

```
providers/
├── provider.go         # Provider interface definition
├── wayback.go         # Wayback Machine CDX API
├── crtsh.go           # Certificate Transparency logs
├── commoncrawl.go     # CommonCrawl index search
├── otx.go             # AlienVault OTX threat intel
├── urlscan.go         # URLScan.io search
├── threatcrowd.go     # ThreatCrowd intelligence
├── shodan.go          # Shodan search engine
├── securitytrails.go  # SecurityTrails DNS history
├── censys.go          # Censys Internet scan data
├── virustotal.go      # VirusTotal API
└── github.go          # GitHub code search

```

## 🔧 Provider Status

### ✅ No API Key Required (6)
- **Wayback Machine** - Historical web archive
- **crt.sh** - SSL Certificate Transparency
- **CommonCrawl** - Web crawl data
- **AlienVault OTX** - Threat intelligence
- **URLScan.io** - URL scanning service
- **ThreatCrowd** - Threat intelligence (strict rate limits)

### 🔑 API Key Required (5)
- **Shodan** - Internet device search
- **SecurityTrails** - DNS history & subdomains
- **Censys** - Internet scan data
- **VirusTotal** - Malware intelligence
- **GitHub** - Code search for URLs

## 📊 Rate Limits

| Provider | Default Rate | Notes |
|----------|-------------|-------|
| Wayback | 10 req/s | Very permissive |
| crt.sh | 5 req/s | Certificate queries |
| CommonCrawl | 5 req/s | Index search |
| OTX | 10 req/s | Public API |
| URLScan | 2 req/s | Can be increased with API key |
| ThreatCrowd | 1 req/10s | Very strict! |
| Shodan | 1 req/s | Requires API key |
| SecurityTrails | 1 req/s | Requires API key |
| Censys | 1 req/s | Requires credentials |
| VirusTotal | 4 req/s | Requires API key |
| GitHub | 30 req/s | With authentication |

## 🚀 Usage

### Basic (no API keys)
```bash
./urlpassivefinder -d example.com -p wayback,crtsh,otx,commoncrawl,urlscan,threatcrowd
```

### With API Keys
```bash
export SHODAN_API_KEY="your-key"
export SECURITYTRAILS_API_KEY="your-key"
export CENSYS_API_ID="your-id"
export CENSYS_API_SECRET="your-secret"
export VIRUSTOTAL_API_KEY="your-key"
export GITHUB_TOKEN="your-token"

./urlpassivefinder -d example.com -p all
```

## 🔄 Adding New Providers

1. Create new file `providers/newprovider.go`
2. Implement the `Provider` interface:
```go
type Provider interface {
    Name() string
    Fetch(ctx context.Context, domain string, results chan<- string) error
}
```
3. Add to main.go provider map
4. Update config defaults if needed

## 📈 Data Volume Comparison

| Provider | Avg URLs/Domain | Quality | Speed |
|----------|----------------|---------|-------|
| Wayback | 1000-50000 | Medium | Fast |
| CommonCrawl | 500-10000 | High | Medium |
| crt.sh | 10-100 | High (subdomains) | Fast |
| OTX | 100-5000 | High | Fast |
| URLScan | 100-1000 | Very High | Fast |
| ThreatCrowd | 10-500 | High | Slow |
| Shodan | 50-500 | Very High | Fast |
| SecurityTrails | 100-1000 | Very High | Fast |
| Censys | 50-500 | Very High | Medium |
| VirusTotal | 100-5000 | High | Fast |
| GitHub | 10-100 | Medium | Fast |

## 🎯 Best Practices

1. **Start with free providers** - wayback, crtsh, otx
2. **Add API providers for depth** - shodan, securitytrails  
3. **Use rate limiting** - Respect provider limits
4. **Stream results** - Don't store everything in memory
5. **Handle failures gracefully** - Continue if one provider fails