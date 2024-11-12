package main

import (
	"fmt"
	"os"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	connStr := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(connStr)
	if err != nil {
		// do something
		fmt.Println(err)
		return
	}
	defer conn.Close()
	fmt.Println("connection succesful")

	fmt.Println("Starting Peril client...")

	uName, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Println(fmt.Errorf("expected name, got: %v", err))
		return
	}

	// pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, routing.PauseKey+"."+uName, routing.PauseKey, int(amqp.Transient))

	gameState := gamelogic.NewGameState(uName)
	hp := handlerPause(gameState)

	pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, routing.PauseKey+"."+uName, routing.PauseKey, int(amqp.Transient), hp)

	for {
		s := gamelogic.GetInput()
		if len(s) == 0 {
			continue
		}
		switch s[0] {
		case "spawn":
			err = gameState.CommandSpawn(s)
			if err != nil {
				fmt.Println(err)
			}
		case "move":
			_, err = gameState.CommandMove(s)
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println("successful move")
			}
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			os.Exit(0)
		default:
			fmt.Println("command unknown")
		}
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(ps routing.PlayingState) {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
	}
}
