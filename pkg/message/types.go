package message

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// 错误定义
var (
	ErrInvalidChecksum = errors.New("消息校验和不匹配")
	ErrInvalidFormat   = errors.New("消息格式无效")
)

// MessageHeader 消息头部信息
type MessageHeader struct {
	MessageID   string `json:"messageId"`
	Timestamp   int64  `json:"timestamp"`
	Version     string `json:"version"`
	Producer    string `json:"producer"`
	ContentType string `json:"contentType"`
}

// MessageMetadata 消息元数据
type MessageMetadata struct {
	Provider       string `json:"provider"`
	DataType       string `json:"dataType"`
	BatchSize      int    `json:"batchSize"`
	Market         string `json:"market,omitempty"`
	TradingSession string `json:"tradingSession,omitempty"`
}

// MessageFormat 标准消息格式
type MessageFormat struct {
	Header   MessageHeader   `json:"header"`
	Metadata MessageMetadata `json:"metadata"`
	Payload  interface{}     `json:"payload"`
	Checksum string          `json:"checksum"`
}

// StockData 股票数据结构
type StockData struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"changePercent"`
	Volume        int64   `json:"volume"`
	Timestamp     string  `json:"timestamp"`
}

// IndexData 指数数据结构
type IndexData struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Value         float64 `json:"value"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"changePercent"`
	Timestamp     string  `json:"timestamp"`
}

// HistoricalDataPoint 历史数据点
type HistoricalDataPoint struct {
	Symbol   string    `json:"symbol"`
	Date     time.Time `json:"date"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   int64     `json:"volume"`
	Turnover float64   `json:"turnover,omitempty"`
}

// NewMessageFormat 创建新的消息格式
func NewMessageFormat(producer, provider, dataType string, payload interface{}) *MessageFormat {
	header := MessageHeader{
		MessageID:   uuid.New().String(),
		Timestamp:   time.Now().Unix(),
		Version:     "1.0",
		Producer:    producer,
		ContentType: "application/json",
	}

	var batchSize int
	switch p := payload.(type) {
	case []StockData:
		batchSize = len(p)
	case []IndexData:
		batchSize = len(p)
	case []HistoricalDataPoint:
		batchSize = len(p)
	default:
		batchSize = 1
	}

	metadata := MessageMetadata{
		Provider:  provider,
		DataType:  dataType,
		BatchSize: batchSize,
	}

	msg := &MessageFormat{
		Header:   header,
		Metadata: metadata,
		Payload:  payload,
	}

	// 计算校验和
	msg.Checksum = msg.calculateChecksum()

	return msg
}

// calculateChecksum 计算消息校验和
func (m *MessageFormat) calculateChecksum() string {
	// 创建消息副本，排除 checksum 字段
	temp := MessageFormat{
		Header:   m.Header,
		Metadata: m.Metadata,
		Payload:  m.Payload,
	}

	data, err := json.Marshal(temp)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

// Validate 验证消息完整性
func (m *MessageFormat) Validate() error {
	expectedChecksum := m.calculateChecksum()
	if m.Checksum != expectedChecksum {
		return ErrInvalidChecksum
	}
	return nil
}

// ToJSON 将消息转换为 JSON 字符串
func (m *MessageFormat) ToJSON() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON 从 JSON 字符串解析消息
func FromJSON(jsonStr string) (*MessageFormat, error) {
	var msg MessageFormat
	if err := json.Unmarshal([]byte(jsonStr), &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// GetStreamName 根据数据类型获取 Redis Stream 名称
func GetStreamName(dataType string) string {
	switch dataType {
	case "stock_realtime":
		return "stream:stock:realtime"
	case "index_realtime":
		return "stream:index:realtime"
	case "historical":
		return "stream:historical"
	default:
		return "stream:unknown"
	}
}

// SetMarketInfo 设置市场信息
func (m *MessageFormat) SetMarketInfo(market, tradingSession string) {
	m.Metadata.Market = market
	m.Metadata.TradingSession = tradingSession
	// 重新计算校验和
	m.Checksum = m.calculateChecksum()
}
