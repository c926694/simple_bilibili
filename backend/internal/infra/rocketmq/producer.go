package rocketmq

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"

	"simple_bilibili/backend/internal/config"
)

const (
	timerDeliverMS = "TIMER_DELIVER_MS"
	// RocketMQ 5.3.4 的默认 timerMaxDelaySec 为 72 小时。
	maxDeliveryDelay = 72 * time.Hour
)

type Producer struct {
	client rocketmq.Producer
}

type producerSettings struct {
	sendTimeout time.Duration
	retries     int
}

type ProducerOption func(*producerSettings)

func WithSendTimeout(timeout time.Duration) ProducerOption {
	return func(settings *producerSettings) { settings.sendTimeout = timeout }
}

func WithProducerRetries(retries int) ProducerOption {
	return func(settings *producerSettings) { settings.retries = retries }
}

func NewProducer(ctx context.Context, cfg config.RocketMQ, groupName string, opts ...ProducerOption) (*Producer, error) {
	settings := producerSettings{sendTimeout: 3 * time.Second, retries: 3}
	for _, opt := range opts {
		opt(&settings)
	}
	if strings.TrimSpace(groupName) == "" || settings.sendTimeout <= 0 || settings.retries < 0 {
		return nil, errors.New("RocketMQ 生产者组名、发送超时或重试次数无效")
	}
	addresses, err := resolveNameServers(ctx, cfg.NameServers)
	if err != nil {
		return nil, err
	}
	client, err := rocketmq.NewProducer(
		producer.WithNameServer(addresses),
		producer.WithGroupName(groupName),
		producer.WithInstanceName("producer-"+rand.Text()),
		producer.WithSendMsgTimeout(settings.sendTimeout),
		producer.WithRetry(settings.retries),
	)
	if err != nil {
		return nil, fmt.Errorf("初始化 RocketMQ 生产者: %w", err)
	}
	if err := client.Start(); err != nil {
		return nil, fmt.Errorf("启动 RocketMQ 生产者: %w", err)
	}
	return &Producer{client: client}, nil
}

type sendSettings struct {
	tag        string
	keys       []string
	delay      *time.Duration
	deliverAt  *time.Time
	properties map[string]string
}

type SendOption func(*sendSettings)

func WithTag(tag string) SendOption {
	return func(settings *sendSettings) { settings.tag = tag }
}

func WithKeys(keys ...string) SendOption {
	return func(settings *sendSettings) { settings.keys = keys }
}

func WithDelay(delay time.Duration) SendOption {
	return func(settings *sendSettings) { settings.delay = &delay }
}

func WithDeliverAt(deliverAt time.Time) SendOption {
	return func(settings *sendSettings) { settings.deliverAt = &deliverAt }
}

func WithProperty(key, value string) SendOption {
	return func(settings *sendSettings) { settings.properties[key] = value }
}

func (p *Producer) PublishJSON(ctx context.Context, topic string, payload any, opts ...SendOption) (string, error) {
	settings, err := prepareSend(time.Now(), opts)
	if err != nil {
		return "", err
	}
	msg, err := settings.message(ctx, topic, payload)
	if err != nil {
		return "", err
	}
	return p.send(ctx, msg)
}

func (p *Producer) PublishBatchJSON(ctx context.Context, topic string, payloads []any, opts ...SendOption) error {
	if len(payloads) == 0 {
		return errors.New("RocketMQ 批量消息不能为空")
	}
	settings, err := prepareSend(time.Now(), opts)
	if err != nil {
		return err
	}
	messages := make([]*primitive.Message, 0, len(payloads))
	for i, payload := range payloads {
		message, err := settings.message(ctx, topic, payload)
		if err != nil {
			return fmt.Errorf("准备第 %d 条批量消息: %w", i+1, err)
		}
		messages = append(messages, message)
	}

	// SDK 批量信封不携带消息的定时属性，定时批次逐条同步投递。
	if settings.properties[timerDeliverMS] != "" {
		for i, message := range messages {
			if _, err := p.send(ctx, message); err != nil {
				return fmt.Errorf("发送第 %d 条定时消息: %w", i+1, err)
			}
		}
		return nil
	}
	_, err = p.send(ctx, messages...)
	return err
}

func (p *Producer) send(ctx context.Context, messages ...*primitive.Message) (string, error) {
	result, err := p.client.SendSync(ctx, messages...)
	if err != nil {
		return "", fmt.Errorf("发送 RocketMQ 消息至 %q: %w", messages[0].Topic, err)
	}
	if result.Status != primitive.SendOK {
		return "", fmt.Errorf("RocketMQ 未确认消息，Topic %q，状态 %v", messages[0].Topic, result.Status)
	}
	return result.MsgID, nil
}

func (p *Producer) Close() error {
	return p.client.Shutdown()
}

func prepareSend(now time.Time, opts []SendOption) (sendSettings, error) {
	settings := sendSettings{properties: make(map[string]string)}
	for _, opt := range opts {
		opt(&settings)
	}
	for key, value := range settings.properties {
		switch key {
		case timerDeliverMS, "TIMER_DELAY_MS", "TIMER_DELAY_SEC", primitive.PropertyDelayTimeLevel:
			return sendSettings{}, fmt.Errorf("RocketMQ 定时属性 %q 只能通过定时选项设置", key)
		}
		if key == "" || strings.ContainsAny(key, "\x01\x02") || strings.ContainsAny(value, "\x01\x02") {
			return sendSettings{}, fmt.Errorf("RocketMQ 消息属性 %q 无效", key)
		}
	}

	if settings.delay != nil && settings.deliverAt != nil {
		return sendSettings{}, errors.New("RocketMQ 相对延迟和绝对投递时间不能同时设置")
	}
	deliverAt := settings.deliverAt
	if settings.delay != nil {
		if *settings.delay <= 0 {
			return sendSettings{}, errors.New("RocketMQ 延迟时长必须为正数")
		}
		target := now.Add(*settings.delay)
		deliverAt = &target
	}
	if deliverAt != nil {
		if !deliverAt.After(now) || deliverAt.Sub(now) > maxDeliveryDelay {
			return sendSettings{}, fmt.Errorf("RocketMQ 投递时间必须晚于当前时间，且最多延迟 %s", maxDeliveryDelay)
		}
		milliseconds := max(deliverAt.UnixMilli(), now.UnixMilli()+1)
		settings.properties[timerDeliverMS] = strconv.FormatInt(milliseconds, 10)
	}
	return settings, nil
}

func (s sendSettings) message(ctx context.Context, topic string, payload any) (*primitive.Message, error) {
	if strings.TrimSpace(topic) == "" {
		return nil, errors.New("RocketMQ Topic 不能为空")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("编码 RocketMQ 消息 JSON: %w", err)
	}
	msg := primitive.NewMessage(topic, body)
	for key, value := range s.properties {
		msg.WithProperty(key, value)
	}
	msg.WithTag(s.tag).WithKeys(s.keys)
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		msg.WithProperty(TraceIDProperty, traceID)
	}
	return msg, nil
}
