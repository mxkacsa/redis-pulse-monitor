# Redis Pulse Monitor

The `redis_pulse_monitor` package provides functionality to monitor and manage Redis-based agents. Each agent sends periodic "pulse" messages to indicate its active state, and the monitor checks the agents' status by receiving these pulses. The monitor can detect lost or found agents and trigger events when these states change.

## Installation
To install, simply run:

```bash
go get github.com/mxkacsa/redis-pulse-monitor
```

# Usage

## Create and start a monitor
To use the monitor, you first need to create an instance of it with a Redis client and a list of agent names. Then, you can start the monitor.

```go
func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "pw",
	})

	pulseMonitor := redis_pulse_monitor.NewMonitor(rdb, []string{"agent1", "agent2"})
	err := pulseMonitor.FoundEvent.SubscribeFunc(OnFound)
	if err != nil {
		panic(err)
	}
	err = pulseMonitor.LostEvent.SubscribeFunc(OnLost)
	if err != nil {
		panic(err)
	}
	err = pulseMonitor.ErrorEvent.SubscribeFunc(OnError)
	if err != nil {
		panic(err)
	}
	err = pulseMonitor.InfoEvent.SubscribeFunc(OnInfo)
	if err != nil {
		panic(err)
	}

	log.Println("Starting pulse monitor")
	err = pulseMonitor.Start()
	if err != nil {
		panic(err)
	}
	defer pulseMonitor.Stop()
}

func OnInfo(message redis_pulse_monitor.Message) {
	println("Info: ", message.Message)
}

func OnFound(results []redis_pulse_monitor.AgentResult) {
	for _, result := range results {
		println("Agent found: ", result.Name)
	}
}

func OnLost(results []redis_pulse_monitor.AgentResult) {
	for _, result := range results {
		println("Agent lost: ", result.Name)
	}
}

func OnError(message redis_pulse_monitor.Message) {
	println("Error: ", message.Message)
}
```

## Create and start an agent
An agent represents a monitored entity that periodically sends and receives pulse messages. You can create an agent instance and start it.

```go
func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost6379",
		Password: "pw",
	})

	agent := redis_pulse_monitor.NewAgent("agent1", rdb)
	err := agent.ErrorEvent.SubscribeFunc(OnError)
	if err != nil {
		panic(err)
	}

	log.Println("Starting agent")

	go func() {
		time.Sleep(10 * time.Second)
		agent.SendError(errors.New("Sample went wrong"))
		time.Sleep(10 * time.Second)
		agent.SendInfo("Info message")
	}()

	err = agent.Start()
	if err != nil {
		panic(err)
	}

	defer agent.Stop()
}

func OnError(message redis_pulse_monitor.Message) {
	log.Println("Error: ", message.Message)
}
```

# Event Handling
The monitor and agents use an event-driven approach. Here are the available events:

- LostEvent: Triggered when an agent is no longer responding (ping timeout).
- FoundEvent: Triggered when an agent responds after being marked lost.
- ErrorEvent: Triggered when an error occurs, such as when a message cannot be processed or a connection issue arises.
- InfoEvent: Triggered when an informational message is received from an agent.