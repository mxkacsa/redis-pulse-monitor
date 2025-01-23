package redis_pulse_monitor

import (
	"context"
	"encoding/json"
	"github.com/mxkacsa/eventbus"
	"github.com/redis/go-redis/v9"
	"time"
)

type Monitor struct {
	client         *redis.Client
	conf           MonitorConfig
	ctx            context.Context
	cancel         context.CancelFunc
	lastPing       time.Time
	agentNames     []string
	agentsWaitRoom map[string]time.Time
	results        map[string]AgentResult
	LostEvent      *eventbus.EventBus[[]AgentResult]
	FoundEvent     *eventbus.EventBus[[]AgentResult]
	ErrorEvent     *eventbus.EventBus[Message]
	InfoEvent      *eventbus.EventBus[Message]
}

// NewMonitor creates a new Monitor instance.
// Parameters:
// - client: A Redis client instance.
// - agentNames: A list of agent names to monitor.
// - conf: Optional configuration for the monitor.
// Returns a pointer to the created Monitor instance.
func NewMonitor(client *redis.Client, agentNames []string, conf ...MonitorConfig) *Monitor {
	if len(conf) == 0 {
		conf = append(conf, MonitorConfig{})
	}

	if conf[0].PulseTopic == "" {
		conf[0].PulseTopic = "pulse"
	}

	if conf[0].PulseTick == 0 {
		conf[0].PulseTick = 5 * time.Second
	}

	if conf[0].MaxWaitResponse == 0 || conf[0].MaxWaitResponse >= conf[0].PulseTick {
		conf[0].MaxWaitResponse = conf[0].PulseTick - 1*time.Second
		if conf[0].MaxWaitResponse < 0 {
			conf[0].MaxWaitResponse = conf[0].PulseTick
		}
	}

	results := make(map[string]AgentResult, len(agentNames))
	for _, agentName := range agentNames {
		results[agentName] = AgentResult{
			Name:     agentName,
			Active:   true,
			LastSeen: time.Time{},
		}
	}

	return &Monitor{
		client:         client,
		conf:           conf[0],
		results:        results,
		agentNames:     agentNames,
		agentsWaitRoom: make(map[string]time.Time),
		FoundEvent:     new(eventbus.EventBus[[]AgentResult]),
		LostEvent:      new(eventbus.EventBus[[]AgentResult]),
		ErrorEvent:     new(eventbus.EventBus[Message]),
		InfoEvent:      new(eventbus.EventBus[Message]),
	}
}

// Start begins the monitoring process by connecting to the Redis server and starting tasks.
// Returns an error if the connection fails.
func (m *Monitor) Start() error {
	m.ctx, m.cancel = context.WithCancel(context.Background())

	_, err := m.client.Ping(m.ctx).Result()
	if err != nil {
		return err
	}

	m.run()
	return nil
}

// Stop halts the monitoring process and closes associated tasks.
func (m *Monitor) Stop() {
	m.cancel()
}

// GetAgentNames returns a list of all monitored agent names.
// Returns a slice containing agent names.
func (m *Monitor) GetAgentNames() []string {
	return m.agentNames
}

// GetResultsMap returns a map of agent statuses, including their name, active status, and last seen time.
// Returns a map where the key is the agent's name, and the value is their status (AgentResult).
func (m *Monitor) GetResultsMap() map[string]AgentResult {
	return m.results
}

// GetResults returns a slice containing the status of all agents.
// Returns a slice of AgentResult for each monitored agent.
func (m *Monitor) GetResults() []AgentResult {
	results := make([]AgentResult, 0, len(m.results))
	for _, result := range m.results {
		results = append(results, result)
	}
	return results
}

// GetLostAgentResults returns a slice of agents that are no longer active.
// Returns a slice of AgentResult for agents who are inactive.
func (m *Monitor) GetLostAgentResults() []AgentResult {
	agents := make([]AgentResult, 0)
	for _, result := range m.results {
		if !result.Active {
			agents = append(agents, result)
		}
	}
	return agents
}

// GetFoundAgentResults returns a slice of agents that are still active.
// Returns a slice of AgentResult for agents who are still active.
func (m *Monitor) GetFoundAgentResults() []AgentResult {
	agents := make([]AgentResult, 0)
	for _, result := range m.results {
		if result.Active {
			agents = append(agents, result)
		}
	}
	return agents
}

// run starts the monitoring loop, listens to Redis channels, and checks agent statuses periodically.
// This method runs in the background and is automatically invoked when the monitor starts.
func (m *Monitor) run() {
	pubSub := m.client.Subscribe(m.ctx, m.conf.PulseTopic)
	defer pubSub.Close()

	ticker := time.NewTicker(m.conf.PulseTick)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			go func() {
				m.pingAgents()
				time.Sleep(m.conf.MaxWaitResponse)
				m.checkAgents()
			}()
		case msg := <-pubSub.Channel():
			m.processMessage(msg.Payload)
		}
	}
}

// pingAgents sends a "ping" message to all agents requesting a response.
// It broadcasts the message to all agents to check if they are active.
func (m *Monitor) pingAgents() {
	m.lastPing = time.Now().UTC()
	msg := Message{
		Sender:  "ping",
		Type:    PulseMessagePing,
		Message: m.lastPing.String(),
	}
	msgStr, err := json.Marshal(msg)
	if err != nil {
		m.ErrorEvent.PublishAsync(Message{
			Sender:  "self",
			Type:    PulseMessageError,
			Message: err.Error(),
		})
		return
	}

	m.agentsWaitRoom = make(map[string]time.Time)
	m.client.Publish(m.ctx, m.conf.PulseTopic, msgStr)
}

// processMessage processes incoming messages from agents, handling different types such as errors, responses, and info.
// Parameters:
// - payload: The message payload from an agent.
// If the message type is an error, it will publish the error to the ErrorEvent.
func (m *Monitor) processMessage(payload string) {
	msg := Message{}
	err := json.Unmarshal([]byte(payload), &msg)
	if err != nil {
		m.ErrorEvent.PublishAsync(Message{
			Sender:  "unknown",
			Type:    PulseMessageError,
			Message: payload,
		})
		return
	}

	if !m.isValidAgentName(msg.Sender) {
		return
	}

	if msg.Type == PulseMessageError {
		m.ErrorEvent.PublishAsync(msg)
		return
	}

	if msg.Type == PulseMessagePing {
		return
	}

	if msg.Type == PulseMessageInfo {
		m.InfoEvent.PublishAsync(msg)
		return
	}

	if msg.Type != PulseMessagePong {
		return
	}

	if msg.Message != m.lastPing.String() {
		return
	}

	m.agentsWaitRoom[msg.Sender] = time.Now().UTC()
}

// checkAgents checks the status of all agents and updates their active status based on responses.
// It will mark agents as lost if no response is received within the expected time window.
func (m *Monitor) checkAgents() {
	losts := make([]AgentResult, 0)
	founds := make([]AgentResult, 0)

	for _, agentName := range m.agentNames {
		if _, ok := m.agentsWaitRoom[agentName]; !ok {
			if !m.results[agentName].Active {
				continue
			}

			result := m.results[agentName]
			result.Active = false
			m.results[agentName] = result
			losts = append(losts, result)
			continue
		}

		changed := false
		if !m.results[agentName].Active {
			changed = true
		}

		result := m.results[agentName]
		result.Active = true
		result.LastSeen = m.agentsWaitRoom[agentName]
		m.results[agentName] = result
		if changed {
			founds = append(founds, result)
		}
	}

	if len(losts) > 0 {
		m.LostEvent.PublishAsync(losts)
	}

	if len(founds) > 0 {
		m.FoundEvent.PublishAsync(founds)
	}
}

// isValidAgentName checks if the provided agent name exists in the list of monitored agents.
// Parameters:
// - name: The name of the agent to check.
// Returns true if the agent name is valid, false otherwise.
func (m *Monitor) isValidAgentName(name string) bool {
	for _, agentName := range m.agentNames {
		if agentName == name {
			return true
		}
	}
	return false
}
