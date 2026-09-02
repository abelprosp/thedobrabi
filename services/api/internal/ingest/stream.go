package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/segmentio/kafka-go"
)

func (e *Engine) pingKafka(cfg SQLConfig) error {
	if cfg.Broker == "" || cfg.Topic == "" {
		return fmt.Errorf("broker e tópico Kafka obrigatórios")
	}
	conn, err := kafka.Dial("tcp", cfg.Broker)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.ReadPartitions(cfg.Topic)
	return err
}

func (e *Engine) readKafka(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	if cfg.Broker == "" || cfg.Topic == "" {
		return nil, nil, fmt.Errorf("broker e tópico Kafka obrigatórios")
	}
	limit := cfg.RowLimit()
	if limit > 5000 {
		limit = 5000
	}
	conn, err := kafka.DialLeader(ctx, "tcp", cfg.Broker, cfg.Topic, 0)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()
	offset, err := conn.ReadLastOffset()
	if err != nil {
		return nil, nil, err
	}
	start := offset - int64(limit)
	if start < 0 {
		start = 0
	}
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{cfg.Broker},
		Topic:     cfg.Topic,
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10e6,
	})
	defer r.Close()
	if err := r.SetOffset(start); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var maps []map[string]any
	for len(maps) < limit {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			break
		}
		row := map[string]any{
			"offset":    msg.Offset,
			"partition": msg.Partition,
			"key":       string(msg.Key),
			"value":     string(msg.Value),
			"time":      msg.Time.UTC().Format(time.RFC3339),
		}
		var obj map[string]any
		if json.Unmarshal(msg.Value, &obj) == nil {
			row["json"] = obj
			for k, v := range obj {
				if _, exists := row[k]; !exists {
					row[k] = v
				}
			}
		}
		maps = append(maps, flattenJSONMap(row))
	}
	if len(maps) == 0 {
		return nil, nil, fmt.Errorf("nenhuma mensagem no tópico %s", cfg.Topic)
	}
	return mapsToRows(maps)
}

func flattenJSONMap(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		switch t := v.(type) {
		case map[string]any:
			b, _ := json.Marshal(t)
			out[k] = string(b)
		default:
			out[k] = v
		}
	}
	return out
}

func mqttBroker(cfg SQLConfig) string {
	b := strings.TrimSpace(cfg.Broker)
	if b == "" {
		return "tcp://localhost:1883"
	}
	if strings.Contains(b, "://") {
		return b
	}
	return "tcp://" + b
}

func (e *Engine) pingMQTT(cfg SQLConfig) error {
	opts := mqtt.NewClientOptions().AddBroker(mqttBroker(cfg)).SetConnectTimeout(5 * time.Second)
	if cfg.User != "" {
		opts.SetUsername(cfg.User)
		opts.SetPassword(cfg.Password)
	}
	cli := mqtt.NewClient(opts)
	tok := cli.Connect()
	if !tok.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout a ligar MQTT")
	}
	if err := tok.Error(); err != nil {
		return err
	}
	cli.Disconnect(250)
	return nil
}

func (e *Engine) readMQTT(ctx context.Context, cfg SQLConfig) ([]string, [][]string, error) {
	topic := cfg.Topic
	if topic == "" {
		topic = "#"
	}
	opts := mqtt.NewClientOptions().AddBroker(mqttBroker(cfg)).SetConnectTimeout(5 * time.Second)
	if cfg.User != "" {
		opts.SetUsername(cfg.User)
		opts.SetPassword(cfg.Password)
	}
	cli := mqtt.NewClient(opts)
	tok := cli.Connect()
	if !tok.WaitTimeout(5 * time.Second) {
		return nil, nil, fmt.Errorf("timeout a ligar MQTT")
	}
	if err := tok.Error(); err != nil {
		return nil, nil, err
	}
	defer cli.Disconnect(250)

	ch := make(chan mqtt.Message, 256)
	sub := cli.Subscribe(topic, 0, func(_ mqtt.Client, msg mqtt.Message) {
		select {
		case ch <- msg:
		default:
		}
	})
	if !sub.WaitTimeout(3 * time.Second) {
		return nil, nil, fmt.Errorf("timeout a subscrever MQTT")
	}
	if err := sub.Error(); err != nil {
		return nil, nil, err
	}

	deadline := time.Now().Add(3 * time.Second)
	var maps []map[string]any
	for time.Now().Before(deadline) && len(maps) < cfg.RowLimit() {
		select {
		case msg := <-ch:
			row := map[string]any{
				"topic":   msg.Topic(),
				"payload": string(msg.Payload()),
				"time":    time.Now().UTC().Format(time.RFC3339),
			}
			var obj map[string]any
			if json.Unmarshal(msg.Payload(), &obj) == nil {
				for k, v := range obj {
					row[k] = v
				}
			}
			maps = append(maps, flattenJSONMap(row))
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			break
		}
	}
	if len(maps) == 0 {
		return nil, nil, fmt.Errorf("nenhuma mensagem MQTT em 3s no tópico %s", topic)
	}
	return mapsToRows(maps)
}
