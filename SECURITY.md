# Security & responsible use

URLPassiveFinder is a **passive** reconnaissance tool: it queries public data sources
(Wayback Machine, CommonCrawl, crt.sh, OTX, URLScan, etc.) and does not touch the target
directly. Even so:

- Use it **only** against domains you are authorized to assess (your own assets, a bug-bounty
  program's in-scope targets, or an engagement with written authorization).
- API keys for optional providers are read from environment variables
  (`URLSCAN_API_KEY`, `VT_API_KEY`, `OTX_API_KEY`, `GITHUB_TOKEN`, …) — never commit them.
- Discovered URLs can be sensitive. Do not publish raw output that reveals a third party's
  attack surface.

Report issues in the tool via GitHub issues.
