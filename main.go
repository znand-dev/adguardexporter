package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Logging level
var logLevel string

func logDebug(format string, v ...interface{}) {
	if logLevel == "debug" {
		log.Printf("[DEBUG] "+format, v...)
	}
}
func logInfo(format string, v ...interface{}) {
	if logLevel == "debug" || logLevel == "info" {
		log.Printf("[INFO] "+format, v...)
	}
}
func logWarn(format string, v ...interface{}) {
	if logLevel == "debug" || logLevel == "info" || logLevel == "warn" {
		log.Printf("[WARN] "+format, v...)
	}
}

// Structs
type AdGuardStatus struct {
    Running           bool    `json:"running"`
    ProtectionEnabled bool    `json:"protection_enabled"`
    DNS struct {
        Enabled         bool     `json:"enabled"`
        Upstreams       []string `json:"upstream_dns"`
        ProcessingTime  float64  `json:"avg_processing_time"`
    } `json:"dns"`
    DHCP struct {
        Enabled bool `json:"enabled"`
        Leases  []struct {
            IP      string `json:"ip"`
            MAC     string `json:"mac"`
            Host    string `json:"hostname"`
            Expires string `json:"expires"`
        } `json:"leases"`
    } `json:"dhcp"`
}

type AdGuardStats struct {
    TimeUnits               string              `json:"time_units"`
    DNSQueries              []int               `json:"dns_queries"`
    BlockedFiltering        []int               `json:"blocked_filtering"`
    ReplacedSafebrowsing    []int               `json:"replaced_safebrowsing"`
    ReplacedSafesearch      []int               `json:"replaced_safesearch"`
    TopQueried              []map[string]int    `json:"top_queried_domains"`
    TopBlocked              []map[string]int    `json:"top_blocked_domains"`
    TopClients              []map[string]int    `json:"top_clients"`
    TopUpstreams            []map[string]int    `json:"top_upstreams_responses"`  // <== fix
    TopUpstreamsResponseTimes []map[string]float64 `json:"top_upstreams_avg_time"` // <== fix
}

var (
	adguardURL, adguardUsername, adguardPassword, exporterPort string
	scrapeInterval time.Duration

	// Prometheus metrics
	scrapeErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "adguard_scrape_errors_total",
		Help: "The number of errors scraping a target",
	})
	protectionEnabled = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_protection_enabled",
		Help: "Whether DNS filtering is enabled",
	})
	adguardRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_running",
		Help: "Whether adguard is running or not",
	})
	queries24h = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_queries",
		Help: "Total queries processed in the last 24 hours",
	})
	blockedFiltered = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_blocked_filtered",
		Help: "Total queries blocked from filter lists",
	})
	blockedSafesearch = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_blocked_safesearch",
		Help: "Total queries blocked by safesearch",
	})
	blockedSafebrowsing = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_blocked_safebrowsing",
		Help: "Total queries blocked by safebrowsing",
	})
	avgProcessingTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_avg_processing_time_seconds",
		Help: "The average query processing time in seconds",
	})
	topQueriedDomains = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_queried_domains",
		Help: "Top queried domains",
	}, []string{"domain"})
	topBlockedDomains = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_blocked_domains",
		Help: "Top blocked domains",
	}, []string{"domain"})
	topClients = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_clients",
		Help: "Top clients",
	}, []string{"client"})
	topUpstreams = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_upstreams",
		Help: "Top upstreams by response count",
	}, []string{"upstream"})
	topUpstreamsResponseTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_upstreams_avg_response_time_seconds",
		Help: "Top upstreams by average response time",
	}, []string{"upstream"})
	dhcpEnabled = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_dhcp_enabled",
		Help: "Whether DHCP is enabled",
	})
	dhcpLeases = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_dhcp_leases",
		Help: "DHCP lease count",
	})
)

func init() {
	err := godotenv.Load()
	if err != nil {
		logWarn("Could not load .env file: %v", err)
	}

	logLevel = os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	adguardURL = os.Getenv("ADGUARD_URL")
	adguardUsername = os.Getenv("ADGUARD_USERNAME")
	adguardPassword = os.Getenv("ADGUARD_PASSWORD")
	exporterPort = os.Getenv("EXPORTER_PORT")
	if exporterPort == "" {
		exporterPort = "9200"
	}

	interval := os.Getenv("SCRAPE_INTERVAL")
	if interval == "" {
		interval = "15s"
	}
	scrapeInterval, err = time.ParseDuration(interval)
	if err != nil {
		log.Fatalf("Invalid SCRAPE_INTERVAL: %v", err)
	}

	// Register metrics
	prometheus.MustRegister(scrapeErrors, protectionEnabled, adguardRunning,
		queries24h, blockedFiltered, blockedSafesearch, blockedSafebrowsing,
		avgProcessingTime, topQueriedDomains, topBlockedDomains, topClients,
		topUpstreams, topUpstreamsResponseTime, dhcpEnabled, dhcpLeases)
}

func fetchAdGuardData(endpoint string, target interface{}) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", adguardURL+endpoint, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(adguardUsername, adguardPassword)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func updateMetrics() {
	topQueriedDomains.Reset()
	topBlockedDomains.Reset()
	topClients.Reset()
	topUpstreams.Reset()
	topUpstreamsResponseTime.Reset()

	var status AdGuardStatus
	if err := fetchAdGuardData("/control/status", &status); err != nil {
		logWarn("Error fetching /control/status: %v", err)
		scrapeErrors.Inc()
		return
	}

	protectionEnabled.Set(boolToFloat(status.ProtectionEnabled))
	adguardRunning.Set(boolToFloat(status.Running))
	avgProcessingTime.Set(status.DNS.ProcessingTime / 1000)
	dhcpEnabled.Set(boolToFloat(status.DHCP.Enabled))
	dhcpLeases.Set(float64(len(status.DHCP.Leases)))

	var stats AdGuardStats
	if err := fetchAdGuardData("/control/stats", &stats); err != nil {
		logWarn("Error fetching /control/stats: %v", err)
		scrapeErrors.Inc()
		return
	}

	var totalQueries, totalBlocked, totalSafeSearch, totalSafeBrowsing int
	for _, v := range stats.DNSQueries {
		totalQueries += v
	}
	for _, v := range stats.BlockedFiltering {
		totalBlocked += v
	}
	for _, v := range stats.ReplacedSafesearch {
		totalSafeSearch += v
	}
	for _, v := range stats.ReplacedSafebrowsing {
		totalSafeBrowsing += v
	}

	queries24h.Set(float64(totalQueries))
	blockedFiltered.Set(float64(totalBlocked))
	blockedSafesearch.Set(float64(totalSafeSearch))
	blockedSafebrowsing.Set(float64(totalSafeBrowsing))

	for _, m := range stats.TopQueried {
		for k, v := range m {
			topQueriedDomains.WithLabelValues(k).Set(float64(v))
		}
	}
	for _, m := range stats.TopBlocked {
		for k, v := range m {
			topBlockedDomains.WithLabelValues(k).Set(float64(v))
		}
	}
	for _, m := range stats.TopClients {
		for k, v := range m {
			topClients.WithLabelValues(k).Set(float64(v))
		}
	}
	for _, m := range stats.TopUpstreams {
		for k, v := range m {
			topUpstreams.WithLabelValues(k).Set(float64(v))
		}
	}
	for _, m := range stats.TopUpstreamsResponseTimes {
		for k, v := range m {
			topUpstreamsResponseTime.WithLabelValues(k).Set(v)
		}
	}

	logDebug("Metrics updated: queries=%d, blocked=%d", totalQueries, totalBlocked)
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func main() {
	ticker := time.NewTicker(scrapeInterval)
	go func() {
		for range ticker.C {
			updateMetrics()
		}
	}()

	updateMetrics()
	http.Handle("/metrics", promhttp.Handler())
	logInfo("Starting AdGuard exporter on :%s", exporterPort)
	log.Fatal(http.ListenAndServe(":"+exporterPort, nil))
}

