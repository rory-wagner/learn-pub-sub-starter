package main

import (
	"fmt"
	"os"
	"os/signal"

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

	pubsub.DeclareAndBind(conn, routing.ExchangePerilDirect, routing.PauseKey+"."+uName, routing.PauseKey, int(amqp.Transient))

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	println("shutting down")
}
