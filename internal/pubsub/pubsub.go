package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Ack = iota
	NackRequeue
	NackDiscard
)

const (
	JSON = iota
	GOB
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
	unmarshaller func([]byte, int) (T, error),
) error {
	err := subscribe(conn, exchange, queueName, key, simpleQueueType, handler, unmarshaller, JSON)
	if err != nil {
		fmt.Println(fmt.Errorf("subscribe failed: %v", err))
		return err
	}
	return nil
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	buf := new(bytes.Buffer)
	err := gob.NewEncoder(buf).Encode(val)
	if err != nil {
		return err
	}

	pub := amqp.Publishing{
		ContentType: "application/gob",
		Body:        buf.Bytes(),
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, pub)

	return err
}

func SubscribeGob[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T, *amqp.Channel) int,
	unmarshaller func([]byte, int) (T, error),
) error {
	err := subscribe(conn, exchange, queueName, key, simpleQueueType, handler, unmarshaller, GOB)
	if err != nil {
		fmt.Println(fmt.Errorf("subscribe failed: %v", err))
		return err
	}
	return nil
}

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int,
	handler func(T, *amqp.Channel) int,
	unmarshaller func([]byte, int) (T, error),
	dataType int,
) error {
	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}
	err = ch.Qos(10, 1000, true)
	if err != nil {
		return err
	}
	amqpDelivery, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	go func() {
		for d := range amqpDelivery {
			val, err := unmarshaller(d.Body, dataType)
			if err != nil {
				fmt.Println(fmt.Errorf("failure to unmarshal in Gob: %v", err))
			}

			ack := handler(val, ch)
			fmt.Printf("ack: %v", ack)
			switch ack {
			case Ack:
				d.Ack(false)
			case NackRequeue:
				d.Nack(false, true)
			case NackDiscard:
				d.Nack(false, false)
			}
		}
	}()
	return nil
}

func UnmarshallerPlayingState() func([]byte, int) (routing.PlayingState, error) {
	return func(arr []byte, dataType int) (routing.PlayingState, error) {
		var ps routing.PlayingState
		psp := &ps
		switch dataType {
		case JSON:
			err := json.Unmarshal(arr, psp)
			if err != nil {
				return ps, fmt.Errorf("error unmarshalling delivery body: %v", err)
			}
			return ps, nil

		case GOB:
			b := bytes.NewBuffer(arr)
			err := gob.NewDecoder(b).Decode(psp)
			if err != nil {
				return ps, fmt.Errorf("decoding failed: %v", err)
			}
			return ps, nil

		default:
			return ps, fmt.Errorf("given dataType is not supported: %q", dataType)
		}
	}
}

func UnmarshallerMove() func([]byte, int) (gamelogic.ArmyMove, error) {
	return func(arr []byte, dataType int) (gamelogic.ArmyMove, error) {
		var am gamelogic.ArmyMove
		amp := &am
		switch dataType {
		case JSON:
			err := json.Unmarshal(arr, amp)
			if err != nil {
				return am, fmt.Errorf("error unmarshalling delivery body: %v", err)
			}
			return am, nil

		case GOB:
			b := bytes.NewBuffer(arr)
			err := gob.NewDecoder(b).Decode(amp)
			if err != nil {
				return am, fmt.Errorf("decoding failed: %v", err)
			}
			return am, nil

		default:
			return am, fmt.Errorf("given dataType is not supported: %q", dataType)
		}
	}
}

func UnmarshallerWar() func([]byte, int) (gamelogic.RecognitionOfWar, error) {
	return func(arr []byte, dataType int) (gamelogic.RecognitionOfWar, error) {
		var row gamelogic.RecognitionOfWar
		rowp := &row
		switch dataType {
		case JSON:
			err := json.Unmarshal(arr, rowp)
			if err != nil {
				return row, fmt.Errorf("error unmarshalling delivery body: %v", err)
			}
			return row, nil

		case GOB:
			b := bytes.NewBuffer(arr)
			err := gob.NewDecoder(b).Decode(rowp)
			if err != nil {
				return row, fmt.Errorf("decoding failed: %v", err)
			}
			return row, nil

		default:
			return row, fmt.Errorf("given dataType is not supported: %q", dataType)
		}
	}
}

func UnmarshallerGameLog() func([]byte, int) (routing.GameLog, error) {
	return func(arr []byte, dataType int) (routing.GameLog, error) {
		var gl routing.GameLog
		glp := &gl
		switch dataType {
		case JSON:
			err := json.Unmarshal(arr, glp)
			if err != nil {
				return gl, fmt.Errorf("error unmarshalling delivery body: %v", err)
			}
			return gl, nil

		case GOB:
			b := bytes.NewBuffer(arr)
			err := gob.NewDecoder(b).Decode(glp)
			if err != nil {
				return gl, fmt.Errorf("decoding failed: %v", err)
			}
			return gl, nil

		default:
			return gl, fmt.Errorf("given dataType is not supported: %q", dataType)
		}
	}
}

func HandlerPause(gs *gamelogic.GameState) func(routing.PlayingState, *amqp.Channel) int {
	return func(ps routing.PlayingState, _ *amqp.Channel) int {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return Ack
	}
}

func HandlerMoves(gs *gamelogic.GameState) func(gamelogic.ArmyMove, *amqp.Channel) int {
	return func(move gamelogic.ArmyMove, aCh *amqp.Channel) int {
		defer fmt.Print("> ")
		mo := gs.HandleMove(move)
		switch mo {
		case gamelogic.MoveOutComeSafe:
			return Ack
		case gamelogic.MoveOutcomeMakeWar:
			row := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.Player,
			}
			err := PublishJSON(aCh, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix+"."+gs.GetUsername(), row)
			if err != nil {
				return NackRequeue
			}
			return Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return NackDiscard
		default:
			return NackDiscard
		}
	}
}

func HandlerWar(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar, *amqp.Channel) int {
	return func(row gamelogic.RecognitionOfWar, aCh *amqp.Channel) int {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(row)
		message := "%s won a war against %s"
		result := NackDiscard
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			result = NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			result = NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			message = fmt.Sprintf(message, winner, loser)
			gl := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     message,
				Username:    gs.GetUsername(),
			}
			err := PublishGob(aCh, routing.ExchangePerilTopic, routing.GameLogSlug+"."+row.Attacker.Username, gl)
			if err != nil {
				result = NackRequeue
			} else {
				result = Ack
			}
		case gamelogic.WarOutcomeYouWon:
			message = fmt.Sprintf(message, winner, loser)
			gl := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     message,
				Username:    gs.GetUsername(),
			}
			err := PublishGob(aCh, routing.ExchangePerilTopic, routing.GameLogSlug+"."+row.Attacker.Username, gl)
			if err != nil {
				result = NackRequeue
			} else {
				result = Ack
			}
		case gamelogic.WarOutcomeDraw:
			message = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			gl := routing.GameLog{
				CurrentTime: time.Now(),
				Message:     message,
				Username:    gs.GetUsername(),
			}
			err := PublishGob(aCh, routing.ExchangePerilTopic, routing.GameLogSlug+"."+row.Attacker.Username, gl)
			if err != nil {
				result = NackRequeue
			} else {
				result = Ack
			}
		default:
			fmt.Println("Error bad outcome of war")
			result = NackDiscard
		}
		return result
	}
}

func HandlerGameLog(_ *gamelogic.GameState) func(routing.GameLog, *amqp.Channel) int {
	return func(gl routing.GameLog, _ *amqp.Channel) int {
		defer fmt.Print("> ")
		gamelogic.WriteLog(gl)
		return Ack
	}
}
