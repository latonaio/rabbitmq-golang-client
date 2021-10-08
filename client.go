package rabbitmq_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/streadway/amqp"
	"go.uber.org/multierr"
)

type RabbitmqClient struct {
	connection    *amqp.Connection
	channel       *amqp.Channel
	url           string
	receiveQueues map[string]struct{}
	sendQueues    map[string]struct{}
	counter       int
	tags          []string
	cancel        context.CancelFunc
	isClosed      bool
	iteratorCh    chan RabbitmqMessage
}

// rabbitmq に接続してクライアントを作る
func NewRabbitmqClient(url string, queueFrom, queueTo []string) (*RabbitmqClient, error) {
	// 接続
	conn, ch, err := connect(url)
	if err != nil {
		return nil, err
	}

	// queue の名前について重複を除外する
	receiveQueues := make(map[string]struct{})
	sendQueues := make(map[string]struct{})
	queues := make(map[string]struct{})

	for _, queue := range queueFrom {
		receiveQueues[queue] = struct{}{}
		queues[queue] = struct{}{}
	}
	for _, queue := range queueTo {
		sendQueues[queue] = struct{}{}
		queues[queue] = struct{}{}
	}

	for queue := range queues {
		_, err = ch.QueueInspect(queue)
		if err != nil {
			return nil, fmt.Errorf("queue does not exist: %w", err)
		}
	}

	client := &RabbitmqClient{
		connection:    conn,
		channel:       ch,
		url:           url,
		receiveQueues: receiveQueues,
		sendQueues:    sendQueues,
	}

	// 切断時の再接続用ルーチン
	go client.checkConnection()

	return client, nil
}

func connect(url string) (*amqp.Connection, *amqp.Channel, error) {
	// 接続
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("connection with rabbitmq error: %w", err)
	}

	// チャンネル生成
	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, fmt.Errorf("create channel error: %w", err)
	}

	// メッセージを同時に受信しない
	if err := ch.Qos(1, 0, false); err != nil {
		return nil, nil, fmt.Errorf("failed to set prefetch count: %w", err)
	}

	return conn, ch, nil
}

func (r *RabbitmqClient) checkConnection() {
	for {
		<-r.connection.NotifyClose(make(chan *amqp.Error))

		log.Println("[RabbitmqClient] disconnected")

		// 切断時
		for {
			if r.isClosed {
				return
			}

			// 5 秒後に再接続
			time.Sleep(5 * time.Second)

			conn, ch, err := connect(r.url)
			if err != nil {
				continue
			}

			r.connection = conn
			r.channel = ch

			if r.cancel == nil {
				break
			}

			// 受信中だった場合、再購読
			if err := r.stop(); err != nil {
				log.Printf("[RabbitmqClient] failed to stop on reconnecting: %v", err)
			}
			if _, err := r.Iterator(); err != nil {
				log.Printf("[RabbitmqClient] failed to iterate on reconnecting: %v", err)
				return
			}

			log.Println("[RabbitmqClient] reconnected")
			break
		}
	}
}

func (r *RabbitmqClient) Close() error {
	if err := r.Stop(); err != nil {
		return fmt.Errorf("failed to stop: %w", err)
	}

	if err := r.channel.Close(); err != nil {
		return fmt.Errorf("close channel error: %w", err)
	}

	// 実際の接続終了よりも先に接続終了フラグを立てる
	r.isClosed = true

	if err := r.connection.Close(); err != nil {
		return fmt.Errorf("close connection error: %w", err)
	}
	return nil
}

func (r *RabbitmqClient) newConsumerTag() string {
	r.counter += 1
	return "tag" + strconv.Itoa(r.counter)
}

func (r *RabbitmqClient) Iterator() (<-chan RabbitmqMessage, error) {
	if r.cancel != nil {
		return nil, errors.New("already iterating")
	}

	if r.iteratorCh == nil {
		r.iteratorCh = make(chan RabbitmqMessage)
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	for rq := range r.receiveQueues {
		tag := r.newConsumerTag()
		r.tags = append(r.tags, tag)
		// queue からメッセージの受け取り
		msgs, err := r.channel.Consume(
			rq,    // queue
			tag,   // consumer
			false, // auto-ack
			false, // exclusive
			false, // no-local
			false, // no-wait
			nil,   // args
		)
		if err != nil {
			// consume に失敗したら受信を Stop() する
			r.Stop()
			return nil, fmt.Errorf("consume error %w", err)
		}

		// 複数の goroutine から1つのチャンネルにメッセージを投げる
		go func() {
			for {
				select {
				case m, ok := <-msgs:
					if !ok {
						continue
					}

					msg, err := NewRabbitmqMessage(m, r)
					if err != nil {
						log.Printf("[RabbitmqClient] failed to parse as json: %v", err)
						m.Nack(false, false)
						return
					}

					r.iteratorCh <- msg
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return r.iteratorCh, nil
}

func (r *RabbitmqClient) stop() error {
	var err error = nil
	// エラーをラップしながら for 文をすべて回す
	for _, tag := range r.tags {
		if perr := r.channel.Cancel(tag, false); perr != nil {
			err = multierr.Append(err, perr)
		}
	}
	r.tags = nil

	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}

	return err
}

func (r *RabbitmqClient) Stop() error {
	if r.iteratorCh == nil {
		return nil
	}

	if err := r.stop(); err != nil {
		return err
	}

	close(r.iteratorCh)
	r.iteratorCh = nil

	return nil
}

func decodeJSON(body []byte) (map[string]interface{}, error) {
	jsonData := map[string]interface{}{}
	if err := json.Unmarshal(body, &jsonData); err != nil {
		return nil, fmt.Errorf("JSON decode error: %w", err)
	}

	return jsonData, nil
}

func (r *RabbitmqClient) Send(sendQueue string, payload map[string]interface{}) error {
	q, err := r.channel.QueueInspect(sendQueue)
	if err != nil {
		return fmt.Errorf("queue does not exists: %w", err)
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	err = r.channel.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         []byte(jsonData),
		})
	if err != nil {
		return fmt.Errorf("failed to publish a message: %w", err)
	}
	return nil
}

func (r *RabbitmqClient) Success(tag uint64) error {
	if err := r.channel.Ack(tag, false); err != nil {
		return fmt.Errorf("success error: %w", err)
	}
	return nil
}

func (r *RabbitmqClient) Fail(tag uint64) error {
	if err := r.channel.Nack(tag, false, false); err != nil {
		return fmt.Errorf("fail error: %w", err)
	}
	return nil
}

func (r *RabbitmqClient) Requeue(tag uint64) error {
	if err := r.channel.Nack(tag, false, true); err != nil {
		return fmt.Errorf("requeue error: %w", err)
	}
	return nil
}

type RabbitmqMessage struct {
	message     amqp.Delivery
	data        map[string]interface{}
	client      *RabbitmqClient
	isResponded bool
}

func NewRabbitmqMessage(d amqp.Delivery, client *RabbitmqClient) (RabbitmqMessage, error) {
	jsonData, err := decodeJSON(d.Body)
	if err != nil {
		return RabbitmqMessage{}, fmt.Errorf("decode json error: %w", err)
	}

	return RabbitmqMessage{data: jsonData, client: client, message: d}, nil
}

func (rm *RabbitmqMessage) QueueName() string {
	return rm.message.RoutingKey
}

func (rm *RabbitmqMessage) Data() map[string]interface{} {
	return rm.data
}

func (r *RabbitmqMessage) Success() error {
	if err := r.client.Success(r.message.DeliveryTag); err != nil {
		return err
	}
	r.isResponded = true
	return nil
}

func (r *RabbitmqMessage) Fail() error {
	if err := r.client.Fail(r.message.DeliveryTag); err != nil {
		return err
	}
	r.isResponded = true
	return nil
}

func (r *RabbitmqMessage) Requeue() error {
	if err := r.client.Requeue(r.message.DeliveryTag); err != nil {
		return err
	}
	r.isResponded = true
	return nil
}

func (r *RabbitmqMessage) IsResponded() bool {
	return r.isResponded
}
