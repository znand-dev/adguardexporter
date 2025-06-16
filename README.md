# 🛡️ AdGuard Exporter for Prometheus

A lightweight Prometheus exporter written in Go that exposes detailed metrics from your AdGuard Home instance — including DNS statistics, blocked domains, upstreams, and client data.

---

## 🚀 Features

- 🔒 Authenticated access to AdGuard Home API
- 📊 Rich metrics: total queries, blocked queries, upstream stats, per-client stats
- 🧠 Supports `/control/status` & `/control/stats` endpoints
- 🔁 Customizable scrape interval
- 📦 Lightweight single binary or Docker container

---

## 🏗️ Built With

- [Go (Golang)](https://golang.org/)
- [Prometheus Client Library](https://github.com/prometheus/client_golang)
- [AdGuard Home](https://github.com/AdguardTeam/AdGuardHome)

---

## ⚙️ Environment Variables

| Variable         | Description                            | Required | Example                      |
|------------------|----------------------------------------|----------|------------------------------|
| `ADGUARD_URL`     | URL to your AdGuard Home API          | ✅       | `http://192.168.1.1:3000`    |
| `ADGUARD_USERNAME`| AdGuard Home username                 | ✅       | `admin`                      |
| `ADGUARD_PASSWORD`| AdGuard Home password                 | ✅       | `secretpassword`             |
| `EXPORTER_PORT`   | Port to expose metrics (default: 9617)| ❌       | `9200`                       |
| `SCRAPE_INTERVAL` | How often to scrape (default: 15s)    | ❌       | `30s`                        |

---

## 🐳 Run via Docker

### ▶️ Quick Start (Inline ENV)

```bash
docker run -d \
  --name adguard_exporter \
  --restart unless-stopped \
  -p 9200:9200 \
  -e ADGUARD_URL=http://172.31.20.12 \
  -e ADGUARD_USERNAME=admin \
  -e ADGUARD_PASSWORD=dt2030jaya \
  -e EXPORTER_PORT=9200 \
  -e SCRAPE_INTERVAL=15s \
  znanddev/adguard-exporter:latest
```

---

## 📦 Run via Docker Compose

### 1. Clone This Repo

```yaml
version: '3.8'

services:
  adguard-exporter:
    image: znanddev/adguard-exporter:latest
    container_name: adguard_exporter
    restart: unless-stopped
    ports:
      - "9200:9200"
    environment:
      - ADGUARD_URL=http://192.168.18.1
      - ADGUARD_USERNAME=admin
      - ADGUARD_PASSWORD=admin
      - EXPORTER_PORT=9200
      - SCRAPE_INTERVAL=15s
```

### 2. Run with Compose

```bash
docker-compose up -d
```

---

## 🧪 Metrics Endpoint

Once running, your exporter will be available at:

```
http://<host>:9617/metrics
```

✅ Ready to scrape by Prometheus!

---

## 📈 Example Prometheus Job

```yaml
- job_name: 'adguard-exporter'
  scrape_interval: 15s
  static_configs:
    - targets: ['adguard-exporter:9617']
```

---

## 📃 License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.

---

## 🙌 Credits

Inspired by:
- [HenryWhitaker3/adguard-exporter](https://github.com/HenryWhitaker3/AdGuardHome-exporter)
- The AdGuard team for their awesome API

---

## ✨ Screenshots

<details>
<summary>Grafana Dashboard Preview</summary>

> _Soon..._ You can contribute a Grafana JSON 😎

</details>

---

## 💬 Feedback

Pull Requests, Issues, and Suggestions are always welcome!

**Made with ❤️ by [znanddev](https://github.com/znand-dev)**
