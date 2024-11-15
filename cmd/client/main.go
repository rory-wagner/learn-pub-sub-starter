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

	aCh, err := conn.Channel()
	if err != nil {
		gamelogic.Exit(err, 1)
	}

	gameState := gamelogic.NewGameState(uName)

	hp := handlerPause(gameState)
	hm := handlerMoves(gameState)
	hw := handlerWar(gameState)

	pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, routing.PauseKey+"."+uName, routing.PauseKey, int(amqp.Transient), hp)
	pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.ArmyMovesPrefix+"."+uName, routing.ArmyMovesPrefix+".*", int(amqp.Transient), hm)
	pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, routing.WarRecognitionsPrefix+".*", int(amqp.Persistent), hw)

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
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			os.Exit(0)
		default:
			fmt.Println("command unknown")
		}
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState, *amqp.Channel) int {
	return func(ps routing.PlayingState, _ *amqp.Channel) int {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMoves(gs *gamelogic.GameState) func(gamelogic.ArmyMove, *amqp.Channel) int {
	return func(move gamelogic.ArmyMove, aCh *amqp.Channel) int {
		defer fmt.Print("> ")
		mo := gs.HandleMove(move)
		switch mo {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			row := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.Player,
			}
			err := pubsub.PublishJSON(aCh, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix+"."+gs.GetUsername(), row)
			if err != nil {
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func handlerWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar, *amqp.Channel) int {
	return func(row gamelogic.RecognitionOfWar, _ *amqp.Channel) int {
		defer fmt.Print("> ")
		outcome, _, _ := gs.HandleWar(row)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			return pubsub.Ack
		default:
			fmt.Println("Error bad outcome of war")
			return pubsub.NackDiscard
		}
	}
}
