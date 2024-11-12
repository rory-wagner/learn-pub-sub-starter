package main

import (
	"fmt"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	connStr := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(connStr)
	if err != nil {
		gamelogic.Exit(err, 1)
	}
	defer conn.Close()
	fmt.Println("connection succesful")

	ch, err := conn.Channel()
	if err != nil {
		gamelogic.Exit(err, 1)
	}

	err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
	if err != nil {
		gamelogic.Exit(err, 1)
	}

	pubsub.DeclareAndBind(conn, routing.ExchangePerilTopic, routing.GameLogSlug, routing.GameLogSlug+".*", int(amqp.Persistent))

	fmt.Println("Starting Peril server...")
	gamelogic.PrintServerHelp()
	for {
		s := gamelogic.GetInput()
		if len(s) == 0 {
			continue
		}
		switch s[0] {
		case "pause":
			fmt.Println("pausing")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
			if err != nil {
				gamelogic.Exit(err, 1)
			}
		case "resume":
			fmt.Println("resuming")
			err = pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
			if err != nil {
				fmt.Println(err)
				gamelogic.Exit(err, 1)
			}
		case "quit":
			fmt.Println("shutting down")
			gamelogic.Exit(nil, 0)
		case "help":
			gamelogic.PrintServerHelp()
		default:
			fmt.Println("command unknown")
		}
	}
}
