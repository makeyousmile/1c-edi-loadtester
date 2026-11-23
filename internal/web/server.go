package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"loadtester/internal/generator"
	"loadtester/internal/mock1c"
	"loadtester/internal/tester"
)

//go:embed static/*
var staticFS embed.FS

type Server struct {
	runner      *tester.Runner
	mock        *mock1c.MockServer
	templateXML string
	mux         *http.ServeMux
}

func NewServer(runner *tester.Runner, mock *mock1c.MockServer, templateXML string) *Server {
	s := &Server{
		runner:      runner,
		mock:        mock,
		templateXML: templateXML,
		mux:         http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// Static files
	subFS, err := fs.Sub(staticFS, "static")
	if err == nil {
		s.mux.Handle("/", http.FileServer(http.FS(subFS)))
	}

	// API routes
	s.mux.HandleFunc("/api/test/start", s.handleStart)
	s.mux.HandleFunc("/api/test/stop", s.handleStop)
	s.mux.HandleFunc("/api/test/status", s.handleStatus)
	s.mux.HandleFunc("/api/test/events", s.handleEvents)
	s.mux.HandleFunc("/api/test/preview", s.handlePreview)

	// Mock 1C endpoint
	s.mux.HandleFunc("/mock/1c/api/v1/edocuments", s.handleMock1C)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for ease of integration
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg tester.TestConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, fmt.Sprintf("Неверный формат JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Resolve endpoint if host is omitted or relative
	if strings.HasPrefix(cfg.EndpointURL, "/") {
		cfg.EndpointURL = "http://" + r.Host + cfg.EndpointURL
	}

	if err := s.runner.Start(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Тестирование успешно запущено",
	})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	s.runner.Stop()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Тестирование остановлено",
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	stats := s.runner.GetStats()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.runner.Subscribe()
	defer s.runner.Unsubscribe(ch)

	// Send initial status immediately
	initial := s.runner.GetStats()
	data, _ := json.Marshal(initial)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case stats, open := <-ch:
			if !open {
				return
			}
			bytesData, err := json.Marshal(stats)
			if err == nil {
				_, _ = fmt.Fprintf(w, "data: %s\n\n", bytesData)
				flusher.Flush()
			}
		}
	}
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GLN                string `json:"gln"`
		Suffix             string `json:"suffix"`
		BaseMessageID      string `json:"baseMessageID"`
		BaseDocumentID     string `json:"baseDocumentID"`
		BaseDeliveryNoteID string `json:"baseDeliveryNoteID"`
		UpdateTimestamps   bool   `json:"updateTimestamps"`
		Count              int    `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	gen, err := generator.NewGenerator(generator.Config{
		TemplateXML:        s.templateXML,
		BaseMessageID:      req.BaseMessageID,
		BaseDocumentID:     req.BaseDocumentID,
		BaseDeliveryNoteID: req.BaseDeliveryNoteID,
		GLN:                req.GLN,
		Suffix:             req.Suffix,
		UpdateTimestamps:   req.UpdateTimestamps,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	previewCount := req.Count
	if previewCount <= 0 {
		previewCount = 2
	}
	if previewCount > 5 {
		previewCount = 5
	}

	docs := make([]*generator.GeneratedDoc, 0, previewCount)
	for i := 0; i < previewCount; i++ {
		doc, err := gen.Generate(i)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		docs = append(docs, doc)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(docs)
}

func (s *Server) handleMock1C(w http.ResponseWriter, r *http.Request) {
	// Update mock settings from query params if passed
	if latencyStr := r.URL.Query().Get("latency"); latencyStr != "" {
		if l, err := strconv.ParseInt(latencyStr, 10, 64); err == nil {
			s.mock.SimulatedLatencyMs = l
		}
	}
	if errRateStr := r.URL.Query().Get("errorRate"); errRateStr != "" {
		if er, err := strconv.ParseInt(errRateStr, 10, 64); err == nil {
			s.mock.SimulateErrorRate = er
		}
	}

	s.mock.ServeHTTP(w, r)
}
