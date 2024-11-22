package main

import (
	"fmt"
	"os"
	"strconv"

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

	aCh, err := conn.Channel()
	if err != nil {
		gamelogic.Exit(err, 1)
	}

	gameState := gamelogic.NewGameState(uName)

	hp := pubsub.HandlerPause(gameState)
	hm := pubsub.HandlerMoves(gameState)
	hw := pubsub.HandlerWar(gameState)

	uPS := pubsub.UnmarshallerPlayingState()
	uM := pubsub.UnmarshallerMove()
	uW := pubsub.UnmarshallerWar()

	pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, routing.PauseKey+"."+uName, routing.PauseKey, int(amqp.Transient), hp, uPS)
	pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+uName, routing.ArmyMovesPrefix+".*", int(amqp.Transient), hm, uM)
	pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, routing.WarRecognitionsPrefix+".*", int(amqp.Persistent), hw, uW)

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
			m, err := gameState.CommandMove(s)
			if err != nil {
				fmt.Println(err)
				continue
			}
			err = pubsub.PublishJSON(aCh, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+uName, m)
			if err != nil {
				fmt.Println(fmt.Errorf("move failed: %v", err))
				continue
			}
			fmt.Println("move published")
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			if len(s) < 2 {
				fmt.Println("command 'spam' need a number to be supplied")
				continue
			}
			num, err := strconv.Atoi(s[1])
			if err != nil {
				fmt.Println(err)
				continue
			}
			for range num {
				logMessage := gamelogic.GetMaliciousLog()
				err = pubsub.PublishGob(aCh, routing.ExchangePerilTopic, routing.GameLogSlug+"."+uName, logMessage)
				if err != nil {
					fmt.Println(err)
				}
			}
		case "quit":
			gamelogic.PrintQuit()
			os.Exit(0)
		default:
			fmt.Println("command unknown")
		}
	}
}
