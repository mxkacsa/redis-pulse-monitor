package main

import (
	"github.com/mxkacsa/redis-pulse-monitor"
	"github.com/redis/go-redis/v9"
	"log"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "141.147.32.48:3131",
		Password: "random1234-ASD-BEA",
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
