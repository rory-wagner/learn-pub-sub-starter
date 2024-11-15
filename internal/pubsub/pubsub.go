package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Ack = iota
	NackRequeue
	NackDiscard
)

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}
	t := amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	}
	q, err := ch.QueueDeclare(
		queueName,
		simpleQueueType == int(amqp.Persistent),
		simpleQueueType == int(amqp.Transient),
		simpleQueueType == int(amqp.Transient),
		false,
		t,
	)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}
	err = ch.QueueBind(queueName, key, exchange, false, nil)
	if err != nil {
		return &amqp.Channel{}, amqp.Queue{}, err
	}
	return ch, q, nil
}

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	bytes, err := json.Marshal(val)
	if err != nil {
		return err
	}

	pub := amqp.Publishing{
		ContentType: "application/json",
		Body:        bytes,
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, pub)

	return err
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T, *amqp.Channel) int,
) error {
	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}
	amqpDelivery, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	go func() {
		for d := range amqpDelivery {
			var t T
			err = json.Unmarshal(d.Body, &t)
			if err != nil {
				fmt.Println(fmt.Errorf("error unmarshalling delivery body: %v", err))
			}
			ack := handler(t, ch)
			fmt.Printf("ack: %v", ack)
			switch ack {
			case Ack:
				d.Ack(false)
				fmt.Println("ACKING")
			case NackRequeue:
				d.Nack(false, true)
				fmt.Println("REQUEING")
			case NackDiscard:
				d.Nack(false, false)
				fmt.Println("DISCARDING")
			}
		}
	}()
	return nil
}
