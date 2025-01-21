package redis_pulse_monitor

import "time"

type PulseMessageType string

const (
	PulseMessagePing  PulseMessageType = "ping"
	PulseMessagePong  PulseMessageType = "pong"
	PulseMessageError PulseMessageType = "error"
	PulseMessageInfo  PulseMessageType = "info"
)

type MonitorConfig struct {
	PulseTopic      string
	PulseTick       time.Duration
	MaxWaitResponse time.Duration
}

type AgentResult struct {
	Name     string    `json:"name"`
	Active   bool      `json:"active"`
	LastSeen time.Time `json:"last_seen"`
}

type Message struct {
	Sender  string           `json:"sender"`
	Type    PulseMessageType `json:"type"`
	Message string           `json:"message"`
}
