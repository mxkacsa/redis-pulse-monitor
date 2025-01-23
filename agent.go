package redis_pulse_monitor

import (
	"context"
	"encoding/json"
	"github.com/mxkacsa/eventbus"
	"github.com/redis/go-redis/v9"
)

type Agent struct {
	Name       string
	client     *redis.Client
	pulseTopic string
	ctx        context.Context
	cancel     context.CancelFunc
	ErrorEvent *eventbus.EventBus[Message]
}

// NewAgent creates a new Agent instance.
// Parameters:
// - name: The name of the agent.
// - client: The Redis client instance used for communication.
// - pulseTopic: Optional, the topic name for pulse messages, defaults to "pulse" if not provided.
// Returns a pointer to the created Agent instance.
func NewAgent(name string, client *redis.Client, pulseTopic ...string) *Agent {
	if len(pulseTopic) == 0 {
		pulseTopic = append(pulseTopic, "pulse")
	}

	return &Agent{
		Name:       name,
		client:     client,
		pulseTopic: pulseTopic[0],
		ErrorEvent: new(eventbus.EventBus[Message]),
	}
}

// Start begins the agent's operation by connecting to the Redis server and listening for pulse messages.
// It returns an error if the connection or initialization fails.
func (a *Agent) Start() error {
	a.ctx, a.cancel = context.WithCancel(context.Background())
	_, err := a.client.Ping(a.ctx).Result()
	if err != nil {
		return err
	}

	a.run()
	return nil
}

// Stop halts the agent's operation and stops listening for pulse messages.
func (a *Agent) Stop() {
	a.cancel()
}

// run starts the agent's background process that listens to the Redis pulse topic and processes incoming messages.
// This method is automatically invoked when the agent is started.
func (a *Agent) run() {
	pubSub := a.client.Subscribe(a.ctx, a.pulseTopic)
	defer pubSub.Close()

	for {
		select {
		case <-a.ctx.Done():
			return
		case msg := <-pubSub.Channel():
			a.processMessage(msg.Payload)
		}
	}
}

// processMessage processes an incoming message from the pulse channel.
// It checks if the message is a ping, and if so, responds with a pong.
// Parameters:
// - payload: The message payload to process.
// If the message is of type "Ping", it will respond with a "Pong".
func (a *Agent) processMessage(payload string) {
	msg := Message{}
	err := json.Unmarshal([]byte(payload), &msg)
	if err != nil {
		return
	}

	if msg.Type != PulseMessagePing {
		return
	}

	err = a.sendPong(msg.Message)
	if err != nil {
		a.ErrorEvent.Publish(Message{
			Sender:  "self",
			Type:    PulseMessageError,
			Message: err.Error(),
		})
		return
	}
}

// sendPong sends a pong response back to the message that was received.
// Parameters:
// - message: The original message that prompted the pong response.
// Returns an error if the sending process fails.
func (a *Agent) sendPong(message string) error {
	return a.sendMessage(message, PulseMessagePong)
}

// SendInfo sends an informational message to the pulse topic.
// Parameters:
// - message: The message to send as an informational message.
// Returns an error if the sending process fails.
func (a *Agent) SendInfo(message string) error {
	return a.sendMessage(message, PulseMessageInfo)
}

// SendError sends an error message to the pulse topic.
// Parameters:
// - err: The error to send as an error message.
// Returns an error if the sending process fails.
func (a *Agent) SendError(err error) error {
	return a.sendMessage(err.Error(), PulseMessageError)
}

// sendMessage sends a message to the Redis pulse topic with a specified message type.
// Parameters:
// - message: The message to send.
// - msgType: The type of the message (Ping, Pong, Info, or Error).
// Returns an error if the sending process fails.
func (a *Agent) sendMessage(message string, msgType PulseMessageType) error {
	msg := Message{
		Sender:  a.Name,
		Type:    msgType,
		Message: message,
	}
	msgStr, _ := json.Marshal(msg)
	return a.client.Publish(a.ctx, a.pulseTopic, msgStr).Err()
}
