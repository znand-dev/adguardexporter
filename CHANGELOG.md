# Changelog

## v1.1.0 - 2025-06-18
### ✨ Added
- `adguard_replaced_parental`: total queries replaced due to parental control
- `adguard_safe_search`: total queries forced into Safe Search (YouTube, Google, Bing)
- `adguard_safe_browsing`: total Safe Browsing protections triggered
- `adguard_top_upstreams`: top DNS upstreams based on query count
- `adguard_top_clients`: top client IPs querying AdGuard
- `adguard_top_queried_domains`: most frequently queried domains
- `adguard_upstream_avg_response_time_seconds`: average DNS upstream response time

### 🛠 Fixed
- Fixed: missing `upstream` and `client` labels due to empty maps or parsing errors
- Fixed: more reliable parsing of nested JSON from `/control/stats` and `/control/status`
- Fixed: exporter now exposes partial metrics even if part of the JSON fails

### 🔄 Refactored
- Cleaned up metric and label names to follow Prometheus naming conventions
- Fixed Dockerfile lint warning: unified casing of `FROM ... AS` keyword for best practices
