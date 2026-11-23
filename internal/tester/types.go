package tester

import (
	"time"
)

type TestConfig struct {
	EndpointURL      string            `json:"endpointUrl"`
	Count            int               `json:"count"`
	Concurrency      int               `json:"concurrency"`
	GLN              string            `json:"gln"`
	Suffix           string            `json:"suffix"`
	DelayMs          int               `json:"delayMs"`
	TimeoutSec       int               `json:"timeoutSec"`
	UpdateTimestamps bool              `json:"updateTimestamps"`
	AuthType         string            `json:"authType"` // "none", "basic", "bearer"
	Username         string            `json:"username"`
	Password         string            `json:"password"`
	BearerToken      string            `json:"bearerToken"`
	CustomHeaders    map[string]string `json:"customHeaders,omitempty"`
}

type ResponseLogItem struct {
	Index                int       `json:"index"`
	Timestamp            time.Time `json:"timestamp"`
	DeliveryNoteID       string    `json:"deliveryNoteID"`
	MessageID            string    `json:"messageID"`
	StatusCode           int       `json:"statusCode"`
	LatencyMs            int64     `json:"latencyMs"`
	Operation            int       `json:"operation"`
	OperationDescription string    `json:"operationDescription"`
	ProcessingStatus     string    `json:"processingStatus"`
	StatusDescription    string    `json:"statusDescription"`
	Error                string    `json:"error,omitempty"`
	RequestXMLSnippet    string    `json:"requestXmlSnippet,omitempty"`
	ResponseBody         string    `json:"responseBody,omitempty"`
}

type TestStats struct {
	TotalPlanned  int               `json:"totalPlanned"`
	TotalSent     int               `json:"totalSent"`
	Success201    int               `json:"success201"`
	BadRequest400 int               `json:"badRequest400"`
	OtherErrors   int               `json:"otherErrors"`
	Running       bool              `json:"running"`
	StartTime     *time.Time        `json:"startTime,omitempty"`
	ElapsedTimeMs int64             `json:"elapsedTimeMs"`
	CurrentRPS    float64           `json:"currentRps"`
	MinLatencyMs  int64             `json:"minLatencyMs"`
	MaxLatencyMs  int64             `json:"maxLatencyMs"`
	AvgLatencyMs  int64             `json:"avgLatencyMs"`
	P95LatencyMs  int64             `json:"p95LatencyMs"`
	RecentLogs    []ResponseLogItem `json:"recentLogs"`
}
