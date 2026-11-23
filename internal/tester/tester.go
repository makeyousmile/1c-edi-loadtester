package tester

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"loadtester/internal/generator"
)

type Runner struct {
	mu           sync.RWMutex
	cfg          TestConfig
	stats        TestStats
	latencies    []int64
	cancelFunc   context.CancelFunc
	subscribers  map[chan TestStats]struct{}
	subMu        sync.Mutex
	logLimit     int
	templateXML  string
}

func NewRunner(templateXML string) *Runner {
	return &Runner{
		templateXML: templateXML,
		subscribers: make(map[chan TestStats]struct{}),
		logLimit:    300,
	}
}

func (r *Runner) Subscribe() chan TestStats {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	ch := make(chan TestStats, 10)
	r.subscribers[ch] = struct{}{}
	return ch
}

func (r *Runner) Unsubscribe(ch chan TestStats) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	delete(r.subscribers, ch)
	close(ch)
}

func (r *Runner) broadcast(stats TestStats) {
	r.subMu.Lock()
	defer r.subMu.Unlock()
	for ch := range r.subscribers {
		select {
		case ch <- stats:
		default:
		}
	}
}

func (r *Runner) GetStats() TestStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s := r.stats
	if s.Running && s.StartTime != nil {
		s.ElapsedTimeMs = time.Since(*s.StartTime).Milliseconds()
		if s.ElapsedTimeMs > 0 {
			s.CurrentRPS = float64(s.TotalSent) / (float64(s.ElapsedTimeMs) / 1000.0)
		}
	}
	return s
}

func (r *Runner) Stop() {
	r.mu.Lock()
	if r.cancelFunc != nil {
		r.cancelFunc()
		r.cancelFunc = nil
	}
	r.stats.Running = false
	currentStats := r.stats
	r.mu.Unlock()

	r.broadcast(currentStats)
}

func (r *Runner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats.Running
}

type OneCStatusResponse struct {
	DeliveryNoteID       string `json:"DeliveryNoteID"`
	Date                 string `json:"Date"`
	Time                 string `json:"Time"`
	Operation            int    `json:"Operation"`
	OperationDescription string `json:"OperationDescription"`
	ProcessingStatus     string `json:"ProcessingStatus"`
	StatusDescription    string `json:"StatusDescription"`
}

func (r *Runner) Start(cfg TestConfig) error {
	r.mu.Lock()
	if r.stats.Running {
		r.mu.Unlock()
		return fmt.Errorf("тестирование уже запущено")
	}

	if cfg.Count <= 0 {
		cfg.Count = 1
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.Concurrency > cfg.Count {
		cfg.Concurrency = cfg.Count
	}
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 30
	}
	if cfg.EndpointURL == "" {
		cfg.EndpointURL = "http://localhost:8080/mock/1c/api/v1/edocuments"
	}

	r.cfg = cfg
	now := time.Now()
	r.stats = TestStats{
		TotalPlanned: cfg.Count,
		TotalSent:    0,
		Running:      true,
		StartTime:    &now,
		RecentLogs:   make([]ResponseLogItem, 0, r.logLimit),
	}
	r.latencies = make([]int64, 0, cfg.Count)

	ctx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel
	r.mu.Unlock()

	gen, err := generator.NewGenerator(generator.Config{
		TemplateXML:      r.templateXML,
		GLN:              cfg.GLN,
		Suffix:           cfg.Suffix,
		UpdateTimestamps: cfg.UpdateTimestamps,
	})
	if err != nil {
		r.Stop()
		return fmt.Errorf("ошибка инициализации генератора XML: %w", err)
	}

	go r.run(ctx, cfg, gen)

	return nil
}

func (r *Runner) run(ctx context.Context, cfg TestConfig, gen *generator.Generator) {
	defer func() {
		r.mu.Lock()
		r.stats.Running = false
		if r.stats.StartTime != nil {
			r.stats.ElapsedTimeMs = time.Since(*r.stats.StartTime).Milliseconds()
			if r.stats.ElapsedTimeMs > 0 {
				r.stats.CurrentRPS = float64(r.stats.TotalSent) / (float64(r.stats.ElapsedTimeMs) / 1000.0)
			}
		}
		stats := r.stats
		r.mu.Unlock()
		r.broadcast(stats)
	}()

	client := &http.Client{
		Timeout: time.Duration(cfg.TimeoutSec) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.Concurrency * 2,
			MaxIdleConnsPerHost: cfg.Concurrency * 2,
			IdleConnTimeout:     90 * time.Second,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		},
	}

	jobs := make(chan int, cfg.Count)
	for i := 0; i < cfg.Count; i++ {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	var ticker *time.Ticker
	var delayCh <-chan time.Time
	if cfg.DelayMs > 0 {
		ticker = time.NewTicker(time.Duration(cfg.DelayMs) * time.Millisecond)
		defer ticker.Stop()
		delayCh = ticker.C
	}

	for w := 0; w < cfg.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case idx, ok := <-jobs:
					if !ok {
						return
					}

					if delayCh != nil {
						select {
						case <-ctx.Done():
							return
						case <-delayCh:
						}
					}

					doc, err := gen.Generate(idx)
					if err != nil {
						r.recordResult(ResponseLogItem{
							Index:     idx + 1,
							Timestamp: time.Now(),
							Error:     fmt.Sprintf("Ошибка генерации XML: %v", err),
						}, 0)
						continue
					}

					r.executeRequest(ctx, client, cfg, idx, doc)
				}
			}
		}()
	}

	wg.Wait()
}

func (r *Runner) executeRequest(ctx context.Context, client *http.Client, cfg TestConfig, idx int, doc *generator.GeneratedDoc) {
	reqBody := []byte(doc.XML)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.EndpointURL, bytes.NewReader(reqBody))
	if err != nil {
		r.recordResult(ResponseLogItem{
			Index:             idx + 1,
			Timestamp:         time.Now(),
			DeliveryNoteID:    doc.DeliveryNoteID,
			MessageID:         doc.MessageID,
			Error:             fmt.Sprintf("Ошибка создания HTTP запроса: %v", err),
			RequestXMLSnippet: snippet(doc.XML, 500),
		}, 0)
		return
	}

	httpReq.Header.Set("Content-Type", "application/xml; charset=utf-8")
	httpReq.Header.Set("Accept", "application/json")

	// Authentication
	if cfg.AuthType == "basic" && cfg.Username != "" {
		httpReq.SetBasicAuth(cfg.Username, cfg.Password)
	} else if cfg.AuthType == "bearer" && cfg.BearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.BearerToken)
	}

	for k, v := range cfg.CustomHeaders {
		httpReq.Header.Set(k, v)
	}

	startTime := time.Now()
	resp, err := client.Do(httpReq)
	latency := time.Since(startTime).Milliseconds()

	if err != nil {
		r.recordResult(ResponseLogItem{
			Index:             idx + 1,
			Timestamp:         startTime,
			DeliveryNoteID:    doc.DeliveryNoteID,
			MessageID:         doc.MessageID,
			LatencyMs:         latency,
			StatusCode:        0,
			Error:             fmt.Sprintf("Ошибка соединения: %v", err),
			RequestXMLSnippet: snippet(doc.XML, 500),
		}, latency)
		return
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	respStr := string(respBytes)

	var statusResp OneCStatusResponse
	_ = json.Unmarshal(respBytes, &statusResp)

	dnID := doc.DeliveryNoteID
	if statusResp.DeliveryNoteID != "" {
		dnID = statusResp.DeliveryNoteID
	}

	logItem := ResponseLogItem{
		Index:                idx + 1,
		Timestamp:            startTime,
		DeliveryNoteID:       dnID,
		MessageID:            doc.MessageID,
		StatusCode:           resp.StatusCode,
		LatencyMs:            latency,
		Operation:            statusResp.Operation,
		OperationDescription: statusResp.OperationDescription,
		ProcessingStatus:     statusResp.ProcessingStatus,
		StatusDescription:    statusResp.StatusDescription,
		RequestXMLSnippet:    doc.XML,
		ResponseBody:         respStr,
	}

	r.recordResult(logItem, latency)
}

func (r *Runner) recordResult(item ResponseLogItem, latencyMs int64) {
	r.mu.Lock()
	r.stats.TotalSent++

	switch item.StatusCode {
	case http.StatusCreated:
		r.stats.Success201++
	case http.StatusBadRequest:
		r.stats.BadRequest400++
	default:
		r.stats.OtherErrors++
	}

	if latencyMs > 0 {
		r.latencies = append(r.latencies, latencyMs)
		if r.stats.MinLatencyMs == 0 || latencyMs < r.stats.MinLatencyMs {
			r.stats.MinLatencyMs = latencyMs
		}
		if latencyMs > r.stats.MaxLatencyMs {
			r.stats.MaxLatencyMs = latencyMs
		}

		var sum int64
		for _, l := range r.latencies {
			sum += l
		}
		r.stats.AvgLatencyMs = sum / int64(len(r.latencies))

		// P95 calculation
		sorted := make([]int64, len(r.latencies))
		copy(sorted, r.latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		p95Idx := int(float64(len(sorted)) * 0.95)
		if p95Idx >= len(sorted) {
			p95Idx = len(sorted) - 1
		}
		r.stats.P95LatencyMs = sorted[p95Idx]
	}

	// Update logs (keep newest at beginning)
	if len(r.stats.RecentLogs) >= r.logLimit {
		r.stats.RecentLogs = append([]ResponseLogItem{item}, r.stats.RecentLogs[:r.logLimit-1]...)
	} else {
		r.stats.RecentLogs = append([]ResponseLogItem{item}, r.stats.RecentLogs...)
	}

	if r.stats.StartTime != nil {
		r.stats.ElapsedTimeMs = time.Since(*r.stats.StartTime).Milliseconds()
		if r.stats.ElapsedTimeMs > 0 {
			r.stats.CurrentRPS = float64(r.stats.TotalSent) / (float64(r.stats.ElapsedTimeMs) / 1000.0)
		}
	}

	statsCopy := r.stats
	r.mu.Unlock()

	r.broadcast(statsCopy)
}

func snippet(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
