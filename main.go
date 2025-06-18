package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type AdGuardStats struct {
	NumDNSQueries       float64              `json:"num_dns_queries"`
	NumBlockedFiltering float64              `json:"num_blocked_filtering"`
	NumReplacedParental float64              `json:"num_replaced_parental"`
	AvgProcessingTime   float64              `json:"avg_processing_time"`
	TopQueriedDomains   []map[string]float64 `json:"top_queried_domains"`
	TopBlockedDomains   []map[string]float64 `json:"top_blocked_domains"`
	TopClients          []map[string]float64 `json:"top_clients"`
	TopUpstream         []map[string]float64 `json:"top_upstreams_responses"`
	TopUpstreamTime     []map[string]float64 `json:"top_upstreams_avg_time"`
}

type AdGuardStatus struct {
	Version                    string   `json:"version"`
	Language                   string   `json:"language"`
	DNSAddresses               []string `json:"dns_addresses"`
	DNSPort                    int      `json:"dns_port"`
	HTTPPort                   int      `json:"http_port"`
	ProtectionDisabledDuration int      `json:"protection_disabled_duration"`
	ProtectionEnabled          bool     `json:"protection_enabled"`
	DHCPAvailable              bool     `json:"dhcp_available"`
	Running                    bool     `json:"running"`
}

type AdGuardQueryLog struct {
	Data []struct {
		Question struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"question"`
		Answer   []string `json:"answer"`
		Reason   string   `json:"reason"`
		Client   string   `json:"client"`
		Elapsed  float64  `json:"elapsedMs"`
		Upstream string   `json:"upstream"`
	} `json:"data"`
}

var (
	// Stats & Status
	dnsQueries = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_dns_queries_total", Help: "Total DNS queries received",
	})
	blockedFiltering = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_blocked_filtering_total", Help: "Total DNS queries blocked",
	})
	replacedParental = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_replaced_parental", Help: "Total parental-replaced queries",
	})
	avgProcessingTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_avg_processing_time", Help: "Avg DNS processing time (ms)",
	})
	statusProtectionEnabled = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_protection_enabled", Help: "Protection enabled (1/0)",
	})
	statusRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_running", Help: "AdGuard service running (1/0)",
	})
	statusDHCPAvailable = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_dhcp_available", Help: "DHCP available (1/0)",
	})
	statusDisabledDuration = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_protection_disabled_duration_seconds",
		Help: "Time since protection disabled (s)",
	})
	versionInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_version_info", Help: "AdGuard version info",
	}, []string{"version"})

	// Top stats
	topQueriedDomains = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_queried_domain_total", Help: "Top queried domains",
	}, []string{"domain"})
	topBlockedDomains = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_blocked_domain_total", Help: "Top blocked domains",
	}, []string{"domain"})
	topClients = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_client_total", Help: "Top client IPs",
	}, []string{"client"})
	topUpstreams = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_upstream_total", Help: "Top upstream servers",
	}, []string{"upstream"})
	topUpstreamTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_upstream_avg_response_time_seconds",
		Help: "Avg response time per upstream (s)",
	}, []string{"upstream"})

	// Querylog
	queryCountByReason = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "adguard_query_reason_total", Help: "Total queries by reason",
	}, []string{"reason"})
	queryCountByType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "adguard_query_type_total", Help: "Total queries by DNS type",
	}, []string{"type"})
	queryHistogramByClient = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "adguard_query_elapsed_ms",
		Help:    "Query duration by client in ms",
		Buckets: prometheus.LinearBuckets(1, 5, 10),
	}, []string{"client"})
        	queryCountByUpstream = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "adguard_query_upstream_total",
		Help: "Total queries per upstream DNS server",
	}, []string{"upstream"})

	queryCountByDomain = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "adguard_query_domain_total",
		Help: "Total queries per domain",
	}, []string{"domain"})

	queryCountClientReason = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "adguard_query_client_reason_total",
		Help: "Total queries by client and reason",
	}, []string{"client", "reason"})

)

func init() {
	_ = godotenv.Load()
	prometheus.MustRegister(
		dnsQueries, blockedFiltering, replacedParental, avgProcessingTime,
		statusProtectionEnabled, statusRunning, statusDHCPAvailable, statusDisabledDuration, versionInfo,
		topQueriedDomains, topBlockedDomains, topClients, topUpstreams, topUpstreamTime,
		queryCountByReason, queryCountByType, queryHistogramByClient,
	)
        prometheus.MustRegister(queryCountByUpstream)
	prometheus.MustRegister(queryCountByDomain)
	prometheus.MustRegister(queryCountClientReason)

}

func logLevel(level, msg string) {
	logLevelEnv := os.Getenv("LOG_LEVEL")
	allowed := map[string]int{"ERROR": 1, "WARN": 2, "INFO": 3, "DEBUG": 4}
	current := allowed[logLevelEnv]
	if current == 0 {
		current = 3
	}
	if allowed[level] <= current {
		log.Printf("[%s] %s", level, msg)
	}
}

func fetchStats() (*AdGuardStats, error) {
	host := os.Getenv("ADGUARD_HOST")
	user := os.Getenv("ADGUARD_USER")
	pass := os.Getenv("ADGUARD_PASS")
	url := host + "/control/stats"
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(user, pass)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var stats AdGuardStats
	json.Unmarshal(body, &stats)
	return &stats, nil
}

func fetchStatus() (*AdGuardStatus, error) {
	host := os.Getenv("ADGUARD_HOST")
	user := os.Getenv("ADGUARD_USER")
	pass := os.Getenv("ADGUARD_PASS")
	url := host + "/control/status"
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(user, pass)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var status AdGuardStatus
	json.Unmarshal(body, &status)
	return &status, nil
}

func fetchQueryLog() (*AdGuardQueryLog, error) {
	host := os.Getenv("ADGUARD_HOST")
	user := os.Getenv("ADGUARD_USER")
	pass := os.Getenv("ADGUARD_PASS")
	url := host + "/control/querylog"
	req, _ := http.NewRequest("GET", url, nil)
	req.SetBasicAuth(user, pass)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var logData AdGuardQueryLog
	json.Unmarshal(body, &logData)
	return &logData, nil
}

func updateQueryLogMetrics() {
	logData, err := fetchQueryLog()
	if err != nil {
		logLevel("ERROR", fmt.Sprintf("Failed to fetch querylog: %v", err))
		return
	}
	for _, q := range logData.Data {
		queryCountByReason.WithLabelValues(q.Reason).Inc()
                queryCountByType.WithLabelValues(q.Question.Type).Inc()
		queryHistogramByClient.WithLabelValues(q.Client).Observe(q.Elapsed)
                queryCountByUpstream.WithLabelValues(q.Upstream).Inc()
		queryCountByDomain.WithLabelValues(q.Question.Name).Inc()
		queryCountClientReason.WithLabelValues(q.Client, q.Reason).Inc()
	}
	logLevel("DEBUG", fmt.Sprintf("Processed %d querylog entries", len(logData.Data)))
}

func updateMetrics() {
	stats, err := fetchStats()
	if err == nil {
		dnsQueries.Set(stats.NumDNSQueries)
		blockedFiltering.Set(stats.NumBlockedFiltering)
		replacedParental.Set(stats.NumReplacedParental)
		avgProcessingTime.Set(stats.AvgProcessingTime)

		topQueriedDomains.Reset()
		for _, m := range stats.TopQueriedDomains {
			for domain, val := range m {
				topQueriedDomains.WithLabelValues(domain).Set(val)
			}
		}
		topBlockedDomains.Reset()
		for _, m := range stats.TopBlockedDomains {
			for domain, val := range m {
				topBlockedDomains.WithLabelValues(domain).Set(val)
			}
		}
		topClients.Reset()
		for _, m := range stats.TopClients {
			for client, val := range m {
				topClients.WithLabelValues(client).Set(val)
			}
		}
		topUpstreams.Reset()
		for _, m := range stats.TopUpstream {
			for up, val := range m {
				topUpstreams.WithLabelValues(up).Set(val)
			}
		}
		topUpstreamTime.Reset()
		for _, m := range stats.TopUpstreamTime {
			for up, val := range m {
				topUpstreamTime.WithLabelValues(up).Set(val)
			}
		}
		logLevel("DEBUG", fmt.Sprintf("Fetched stats: %+v", stats))
	}

	status, err := fetchStatus()
	if err == nil {
		statusProtectionEnabled.Set(boolToFloat(status.ProtectionEnabled))
		statusRunning.Set(boolToFloat(status.Running))
		statusDHCPAvailable.Set(boolToFloat(status.DHCPAvailable))
		statusDisabledDuration.Set(float64(status.ProtectionDisabledDuration))
		versionInfo.Reset()
		versionInfo.WithLabelValues(status.Version).Set(1)
		logLevel("DEBUG", fmt.Sprintf("Fetched status: %+v", status))
	}

	updateQueryLogMetrics()
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func main() {
	scrapeIntervalStr := os.Getenv("SCRAPE_INTERVAL")
	port := os.Getenv("EXPORTER_PORT")
	if port == "" {
		port = "9617"
	}
	interval, err := strconv.Atoi(scrapeIntervalStr)
	if err != nil || interval < 1 {
		interval = 30
	}

	go func() {
		for {
			updateMetrics()
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	logLevel("INFO", fmt.Sprintf("Starting exporter at :%s", port))
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
