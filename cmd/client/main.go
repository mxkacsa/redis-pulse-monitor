package main

import (
	"errors"
	"github.com/mxkacsa/redis-pulse-monitor"
	"github.com/redis/go-redis/v9"
	"log"
	"time"
)

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "141.147.32.48:3131",
		Password: "random1234-ASD-BEA",
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
