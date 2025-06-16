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

// AdGuardStatus represents the response from /control/status
type AdGuardStatus struct {
	DNS struct {
		Enabled          bool     `json:"enabled"`
		Upstreams       []string `json:"upstream_dns"`
		ProcessingTime  float64  `json:"avg_processing_time"`
	} `json:"dns"`
	Protection struct {
		Enabled     bool `json:"enabled"`
		Queries    int  `json:"total_queries"`
		Blocked     int  `json:"blocked_filtering"`
		ReplacedSafebrowsing int `json:"blocked_safebrowsing"`
		ReplacedSafesearch   int `json:"blocked_safesearch"`
	} `json:"protection"`
	DHCP struct {
		Enabled bool `json:"enabled"`
		Leases  []struct {
			IP      string `json:"ip"`
			MAC     string `json:"mac"`
			Host    string `json:"hostname"`
			Expires string `json:"expires"`
		} `json:"leases"`
	} `json:"dhcp"`
	Running bool `json:"running"`
}

// AdGuardStats represents the response from /control/stats
type AdGuardStats struct {
	TimeUnits string `json:"time_units"`
	DNSQueries []int `json:"dns_queries"`
	BlockedFiltering []int `json:"blocked_filtering"`
	ReplacedSafebrowsing []int `json:"replaced_safebrowsing"`
	ReplacedSafesearch []int `json:"replaced_safesearch"`
	TopQueried []map[string]int `json:"top_queried_domains"`
	TopBlocked []map[string]int `json:"top_blocked_domains"`
	TopClients []map[string]int `json:"top_clients"`
	TopUpstreams []map[string]int `json:"top_upstreams"`
	TopUpstreamsResponseTimes []map[string]float64 `json:"top_upstreams_avg_response_time"`
}

var (
	adguardURL      string
	adguardUsername string
	adguardPassword string
	exporterPort    string
	scrapeInterval  time.Duration

	// Metrics
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
		Help: "Total queries that have been blocked from filter lists",
	})
	blockedSafesearch = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_blocked_safesearch",
		Help: "Total queries that have been blocked due to safesearch",
	})
	blockedSafebrowsing = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_blocked_safebrowsing",
		Help: "Total queries that have been blocked due to safebrowsing",
	})
	avgProcessingTime = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_avg_processing_time_seconds",
		Help: "The average query processing time in seconds",
	})
	topQueriedDomains = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_queried_domains",
		Help: "The number of queries for the top domains",
	}, []string{"domain"})
	topBlockedDomains = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_blocked_domains",
		Help: "The number of blocked queries for the top domains",
	}, []string{"domain"})
	topClients = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_clients",
		Help: "The number of queries for the top clients",
	}, []string{"client"})
	topUpstreams = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_upstreams",
		Help: "The number of responses for the top upstream servers",
	}, []string{"upstream"})
	topUpstreamsResponseTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "adguard_top_upstreams_avg_response_time_seconds",
		Help: "The average response time for each of the top upstream servers",
	}, []string{"upstream"})
	dhcpEnabled = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_dhcp_enabled",
		Help: "Whether dhcp is enabled",
	})
	dhcpLeases = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "adguard_dhcp_leases",
		Help: "The dhcp leases count",
	})
)

func init() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	adguardURL = os.Getenv("ADGUARD_URL")
	adguardUsername = os.Getenv("ADGUARD_USERNAME")
	adguardPassword = os.Getenv("ADGUARD_PASSWORD")
	exporterPort = os.Getenv("EXPORTER_PORT")
	if exporterPort == "" {
		exporterPort = "9100"
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
	prometheus.MustRegister(scrapeErrors)
	prometheus.MustRegister(protectionEnabled)
	prometheus.MustRegister(adguardRunning)
	prometheus.MustRegister(queries24h)
	prometheus.MustRegister(blockedFiltered)
	prometheus.MustRegister(blockedSafesearch)
	prometheus.MustRegister(blockedSafebrowsing)
	prometheus.MustRegister(avgProcessingTime)
	prometheus.MustRegister(topQueriedDomains)
	prometheus.MustRegister(topBlockedDomains)
	prometheus.MustRegister(topClients)
	prometheus.MustRegister(topUpstreams)
	prometheus.MustRegister(topUpstreamsResponseTime)
	prometheus.MustRegister(dhcpEnabled)
	prometheus.MustRegister(dhcpLeases)
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
		return fmt.Errorf("API request failed with status code %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func updateMetrics() {
	// Reset all metrics with labels
	topQueriedDomains.Reset()
	topBlockedDomains.Reset()
	topClients.Reset()
	topUpstreams.Reset()
	topUpstreamsResponseTime.Reset()

	// Fetch status data
	var status AdGuardStatus
	if err := fetchAdGuardData("/control/status", &status); err != nil {
		log.Printf("Error fetching status: %v", err)
		scrapeErrors.Inc()
		return
	}

	// Update status metrics
	protectionEnabled.Set(boolToFloat(status.Protection.Enabled))
	adguardRunning.Set(boolToFloat(status.Running))
	avgProcessingTime.Set(status.DNS.ProcessingTime / 1000) // Convert ms to seconds
	dhcpEnabled.Set(boolToFloat(status.DHCP.Enabled))
	dhcpLeases.Set(float64(len(status.DHCP.Leases)))

	// Fetch stats data
	var stats AdGuardStats
	if err := fetchAdGuardData("/control/stats", &stats); err != nil {
		log.Printf("Error fetching stats: %v", err)
		scrapeErrors.Inc()
		return
	}

	// Calculate 24h totals
	var queries24hTotal, blocked24hTotal, safesearch24hTotal, safebrowsing24hTotal int
	for _, q := range stats.DNSQueries {
		queries24hTotal += q
	}
	for _, b := range stats.BlockedFiltering {
		blocked24hTotal += b
	}
	for _, s := range stats.ReplacedSafesearch {
		safesearch24hTotal += s
	}
	for _, s := range stats.ReplacedSafebrowsing {
		safebrowsing24hTotal += s
	}

	// Update stats metrics
	queries24h.Set(float64(queries24hTotal))
	blockedFiltered.Set(float64(blocked24hTotal))
	blockedSafesearch.Set(float64(safesearch24hTotal))
	blockedSafebrowsing.Set(float64(safebrowsing24hTotal))

	// Update top domains metrics
	for _, domain := range stats.TopQueried {
		for d, count := range domain {
			topQueriedDomains.WithLabelValues(d).Set(float64(count))
		}
	}

	for _, domain := range stats.TopBlocked {
		for d, count := range domain {
			topBlockedDomains.WithLabelValues(d).Set(float64(count))
		}
	}

	// Update top clients metrics
	for _, client := range stats.TopClients {
		for c, count := range client {
			topClients.WithLabelValues(c).Set(float64(count))
		}
	}

	// Update upstream metrics
	for _, upstream := range stats.TopUpstreams {
		for u, count := range upstream {
			topUpstreams.WithLabelValues(u).Set(float64(count))
		}
	}

	for _, upstream := range stats.TopUpstreamsResponseTimes {
		for u, time := range upstream {
			topUpstreamsResponseTime.WithLabelValues(u).Set(time)
		}
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func main() {
	// Update metrics at interval
	ticker := time.NewTicker(scrapeInterval)
	go func() {
		for range ticker.C {
			updateMetrics()
		}
	}()

	// Initial metrics update
	updateMetrics()

	// Expose the registered metrics via HTTP
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Starting AdGuard exporter on :%s", exporterPort)
	log.Fatal(http.ListenAndServe(":"+exporterPort, nil))
}
