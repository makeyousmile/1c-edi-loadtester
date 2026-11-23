package mock1c

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var (
	reDeliveryNoteID = regexp.MustCompile(`(?s)<DeliveryNoteID>(.*?)</DeliveryNoteID>`)
	reMessageID      = regexp.MustCompile(`(?s)<MessageID>(.*?)</MessageID>`)
)

type StatusResponse struct {
	DeliveryNoteID       string `json:"DeliveryNoteID"`
	Date                 string `json:"Date"`
	Time                 string `json:"Time"`
	Operation            int    `json:"Operation"`
	OperationDescription string `json:"OperationDescription"`
	ProcessingStatus     string `json:"ProcessingStatus"`
	StatusDescription    string `json:"StatusDescription"`
}

type MockServer struct {
	SimulatedLatencyMs int64
	SimulateErrorRate  int64 // percentage 0-100
	RequestCount       int64
}

func NewMockServer() *MockServer {
	return &MockServer{
		SimulatedLatencyMs: 10,
		SimulateErrorRate:  0,
	}
}

func (m *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&m.RequestCount, 1)

	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if m.SimulatedLatencyMs > 0 {
		time.Sleep(time.Duration(m.SimulatedLatencyMs) * time.Millisecond)
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("15:04:05")

	xmlStr := string(body)
	var dnID string
	mDN := reDeliveryNoteID.FindStringSubmatch(xmlStr)
	if len(mDN) >= 2 {
		dnID = strings.TrimSpace(mDN[1])
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Structural validation
	if !strings.Contains(xmlStr, "<BLRWBL>") || !strings.Contains(xmlStr, "</BLRWBL>") {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(StatusResponse{
			DeliveryNoteID:       dnID,
			Date:                 dateStr,
			Time:                 timeStr,
			Operation:            5,
			OperationDescription: "Загрузка электронного документа в 1С",
			ProcessingStatus:     "2",
			StatusDescription:    "Ошибка структуры: отсутствует корневой тег BLRWBL",
		})
		return
	}

	// Check simulated random error
	if m.SimulateErrorRate > 0 && (atomic.LoadInt64(&m.RequestCount)%100) < m.SimulateErrorRate {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(StatusResponse{
			DeliveryNoteID:       dnID,
			Date:                 dateStr,
			Time:                 timeStr,
			Operation:            5,
			OperationDescription: "Загрузка электронного документа в 1С",
			ProcessingStatus:     "2",
			StatusDescription:    "Имитация ошибки структуры: неверный формат данных",
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(StatusResponse{
		DeliveryNoteID:       dnID,
		Date:                 dateStr,
		Time:                 timeStr,
		Operation:            5,
		OperationDescription: "Загрузка электронного документа в 1С",
		ProcessingStatus:     "1",
		StatusDescription:    "ОК",
	})
}
