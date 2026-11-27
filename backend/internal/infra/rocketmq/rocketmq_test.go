package rocketmq

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/admin"
	"github.com/apache/rocketmq-client-go/v2/primitive"

	"simple_bilibili/backend/internal/config"
)

func TestSendOptions(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	for _, tc := range []struct {
		name string
		opts []SendOption
		want time.Duration
		ok   bool
	}{
		{"即时", nil, 0, true},
		{"相对延迟", []SendOption{WithDelay(2 * time.Second)}, 2 * time.Second, true},
		{"绝对时间", []SendOption{WithDeliverAt(now.Add(time.Minute))}, time.Minute, true},
		{"最大窗口", []SendOption{WithDelay(72 * time.Hour)}, 72 * time.Hour, true},
		{"超过窗口", []SendOption{WithDelay(72*time.Hour + time.Millisecond)}, 0, false},
		{"零时长", []SendOption{WithDelay(0)}, 0, false},
		{"负时长", []SendOption{WithDelay(-time.Second)}, 0, false},
		{"已到期", []SendOption{WithDeliverAt(now)}, 0, false},
		{"两种延迟同时设置", []SendOption{WithDelay(time.Second), WithDeliverAt(now.Add(time.Second))}, 0, false},
		{"直接写入定时属性", []SendOption{WithProperty("TIMER_DELIVER_MS", "1")}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings, err := prepareSend(now, tc.opts)
			if (err == nil) != tc.ok {
				t.Fatalf("设置定时选项返回 %v", err)
			}
			if !tc.ok {
				return
			}
			want := ""
			if tc.want != 0 {
				want = strconv.FormatInt(now.Add(tc.want).UnixMilli(), 10)
			}
			if got := settings.properties["TIMER_DELIVER_MS"]; got != want {
				t.Fatalf("投递时间为 %q，期望 %q", got, want)
			}
		})
	}
}

func TestDecodeJSONAndPermanentError(t *testing.T) {
	var decoded map[string]string
	if err := (Message{Body: []byte(`{"name":"测试"}`)}).DecodeJSON(&decoded); err != nil || decoded["name"] != "测试" {
		t.Fatalf("解码结果不符: %v，%v", decoded, err)
	}
	err := (Message{Body: []byte("{")}).DecodeJSON(&decoded)
	if err == nil || IsPermanentError(err) || IsPermanentError(nil) || PermanentError(nil) != nil {
		t.Fatalf("普通错误或空值分类不符: %v", err)
	}
	marked := PermanentError(err)
	wrapped := fmt.Errorf("消息无法处理: %w", marked)
	if !IsPermanentError(wrapped) || !errors.Is(wrapped, err) || PermanentError(marked) != marked {
		t.Fatal("永久性错误标记或错误链不符")
	}
}

func TestResolveNameServers(t *testing.T) {
	addresses, err := resolveNameServers(t.Context(), []string{"localhost:9876", "127.0.0.1:9877"})
	if err != nil {
		t.Fatal(err)
	}
	var lookedUp bool
	for _, address := range addresses {
		host, port, err := net.SplitHostPort(address)
		if err != nil || net.ParseIP(host) == nil {
			t.Fatalf("解析结果应包含 IP 地址和端口，收到 %q", address)
		}
		lookedUp = lookedUp || port == "9876"
	}
	if !lookedUp || addresses[len(addresses)-1] != "127.0.0.1:9877" {
		t.Fatalf("NameServer 解析结果不符: %v", addresses)
	}
}

func TestPushHandlerTimeout(t *testing.T) {
	c := &PushConsumer{handleTimeout: 10 * time.Millisecond}
	err := c.handle(t.Context(), func(ctx context.Context, _ Message) error {
		<-ctx.Done()
		return nil
	}, Message{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("处理超时返回 %v", err)
	}
}

func TestRocketMQIntegration(t *testing.T) {
	if os.Getenv("BILIBILI_ROCKETMQ_INTEGRATION") != "1" {
		t.Skip("设置 BILIBILI_ROCKETMQ_INTEGRATION=1，以连接本地 RocketMQ 5.3.4")
	}
	addr := os.Getenv("BILIBILI_ROCKETMQ_TEST_ADDR")
	if addr == "" {
		addr = "127.0.0.1:9876"
	}
	broker := os.Getenv("BILIBILI_ROCKETMQ_TEST_BROKER")
	if broker == "" {
		broker = "127.0.0.1:10911"
	}
	cfg := config.RocketMQ{NameServers: []string{addr}}
	names, err := resolveNameServers(t.Context(), cfg.NameServers)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := admin.NewAdmin(admin.WithResolver(primitive.NewPassthroughResolver(names)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
	})
	suffix := strings.ToLower(rand.Text())
	producer, err := NewProducer(t.Context(), cfg, "sdk_test_producer_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := producer.Close(); err != nil {
			t.Error(err)
		}
	})

	topic := func(t *testing.T, name string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
		defer cancel()
		if err := manager.CreateTopic(ctx, admin.WithTopicCreate(name), admin.WithBrokerAddrCreate(broker),
			admin.WithReadQueueNums(1), admin.WithWriteQueueNums(1)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if err := manager.DeleteTopic(ctx, admin.WithTopicDelete(name), admin.WithBrokerAddrDelete(broker), admin.WithNameSrvAddr(names)); err != nil {
				t.Error(err)
			}
		})
		for {
			queues, err := manager.FetchPublishMessageQueues(ctx, name)
			if err == nil && len(queues) == 1 {
				return name
			}
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				t.Fatalf("未发现测试 Topic %s 的路由: %v", name, err)
			case <-timer.C:
			}
		}
	}

	t.Run("推送、延迟与重试", func(t *testing.T) {
		group := "sdk_test_push_" + suffix
		topic(t, "%RETRY%"+group)
		verifyPushIntegration(t, cfg, producer, topic(t, "sdk_test_push_"+suffix), group)
	})
	t.Run("批量拉取、重试与位点", func(t *testing.T) {
		verifyPullIntegration(t, cfg, producer, topic(t, "sdk_test_pull_"+suffix), "sdk_test_pull_"+suffix)
	})
}

func verifyPushIntegration(t *testing.T, cfg config.RocketMQ, producer *Producer, topic, group string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	push, err := NewPushConsumer(ctx, cfg, group, WithMaxReconsumeTimes(3), WithHandleTimeout(5*time.Second),
		WithMessageModel(Clustering), WithConsumeFromWhere(ConsumeFromFirstOffset))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := push.Close(); err != nil {
			t.Error(err)
		}
	})
	type received struct {
		kind  string
		msg   Message
		trace string
		at    time.Time
	}
	events := make(chan received, 24)
	if err := push.Subscribe(topic, "sdk", func(ctx context.Context, msg Message) error {
		var data struct{ Kind string }
		if err := msg.DecodeJSON(&data); err != nil {
			return PermanentError(err)
		}
		events <- received{data.Kind, msg, TraceIDFromContext(ctx), time.Now()}
		switch data.Kind {
		case "permanent":
			return PermanentError(errors.New("此消息不可重试"))
		case "transient":
			if msg.ReconsumeTimes == 0 {
				return errors.New("此次处理暂时失败")
			}
		case "panic":
			if msg.ReconsumeTimes == 0 {
				panic("触发推消息异常恢复")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := push.Start(); err != nil {
		t.Fatal(err)
	}

	ctx = WithTraceID(ctx, "sdk-push-trace")
	read := func() received {
		t.Helper()
		select {
		case event := <-events:
			if event.msg.Topic != topic || event.msg.Tag != "sdk" || event.trace != "sdk-push-trace" ||
				event.msg.Properties["source"] != "sdk-test" || !reflect.DeepEqual(event.msg.Keys, []string{event.kind, "test"}) {
				t.Fatalf("推消息元数据不符: %+v", event)
			}
			return event
		case <-ctx.Done():
			t.Fatal("等待推消息超时")
			return received{}
		}
	}
	publish := func(kind string, extra ...SendOption) string {
		t.Helper()
		opts := []SendOption{WithTag("sdk"), WithKeys(kind, "test"), WithProperty("source", "sdk-test")}
		id, err := producer.PublishJSON(ctx, topic, map[string]string{"Kind": kind}, append(opts, extra...)...)
		if err != nil || id == "" {
			t.Fatalf("发送 %s 返回 ID %q，错误 %v", kind, id, err)
		}
		return id
	}
	id := publish("immediate")
	if event := read(); event.kind != "immediate" || event.msg.MsgID != id {
		t.Fatalf("即时消息不符: %+v", event)
	}
	start := time.Now()
	publish("delayed", WithDelay(2*time.Second))
	publish("permanent")
	publish("transient")
	publish("panic")

	count := make(map[string]int)
	var transientRetried, panicRetried bool
	for count["delayed"] == 0 || count["permanent"] == 0 || !transientRetried || !panicRetried {
		event := read()
		count[event.kind]++
		switch event.kind {
		case "delayed":
			if event.at.Sub(start) < 2*time.Second {
				t.Fatalf("延迟消息在 %v 后提前到达", event.at.Sub(start))
			}
		case "transient":
			transientRetried = event.msg.ReconsumeTimes > 0
		case "panic":
			panicRetried = event.msg.ReconsumeTimes > 0
		}
	}
	if count["permanent"] != 1 || count["transient"] != 2 || count["panic"] != 2 {
		t.Fatalf("推消息重投次数不符: %v", count)
	}
}

func verifyPullIntegration(t *testing.T, cfg config.RocketMQ, producer *Producer, topic, group string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	ctx = WithTraceID(ctx, "sdk-pull-trace")
	if err := producer.PublishBatchJSON(ctx, topic, []any{1, 2, 3}, WithTag("sdk"), WithProperty("source", "batch")); err != nil {
		t.Fatal(err)
	}
	batchConfig := PullBatchConfig{BatchSize: 3, PollTimeout: 300 * time.Millisecond, RetryBackoff: 250 * time.Millisecond}
	verifyBatch := func(ctx context.Context, batch []Message, wanted []int) error {
		values := make([]int, len(batch))
		for i, msg := range batch {
			if msg.Topic != topic || msg.Tag != "sdk" || msg.Properties[TraceIDProperty] != "sdk-pull-trace" {
				return PermanentError(fmt.Errorf("拉取消息元数据不符: %+v", msg))
			}
			if err := msg.DecodeJSON(&values[i]); err != nil {
				return PermanentError(err)
			}
		}
		if !reflect.DeepEqual(values, wanted) || TraceIDFromContext(ctx) != "sdk-pull-trace" {
			return PermanentError(fmt.Errorf("拉取批次为 %v，期望 %v", values, wanted))
		}
		return nil
	}

	uncommittedCtx, stopUncommitted := context.WithCancel(ctx)
	defer stopUncommitted()
	uncommitted, err := NewPullConsumer(uncommittedCtx, cfg, group, WithPullConsumeFromWhere(ConsumeFromFirstOffset))
	if err != nil {
		t.Fatal(err)
	}
	defer uncommitted.Close()
	var interrupted bool
	if err := uncommitted.RunBatch(uncommittedCtx, topic, "sdk", batchConfig, func(ctx context.Context, batch []Message) error {
		if err := verifyBatch(ctx, batch, []int{1, 2, 3}); err != nil {
			return err
		}
		interrupted = true
		stopUncommitted()
		return errors.New("取消前此批次尚未处理成功")
	}); err != nil || !interrupted {
		t.Fatalf("批次处理失败后取消循环，返回 %v", err)
	}

	attempts := 0
	var firstAttempt time.Time
	retryCtx, stopRetry := context.WithCancel(ctx)
	pull, err := NewPullConsumer(ctx, cfg, group, WithPullConsumeFromWhere(ConsumeFromFirstOffset))
	if err != nil {
		t.Fatal(err)
	}
	defer pull.Close()
	err = pull.RunBatch(retryCtx, topic, "sdk", batchConfig, func(ctx context.Context, batch []Message) error {
		if err := verifyBatch(ctx, batch, []int{1, 2, 3}); err != nil {
			return err
		}
		attempts++
		if attempts == 1 {
			firstAttempt = time.Now()
			return errors.New("批量处理暂时失败")
		}
		if time.Since(firstAttempt) < batchConfig.RetryBackoff {
			return PermanentError(errors.New("未等待设定的重试间隔"))
		}
		stopRetry()
		return nil
	})
	stopRetry()
	if err != nil || attempts != 2 {
		t.Fatalf("批量重试了 %d 次，返回 %v", attempts, err)
	}

	if _, err := producer.PublishJSON(ctx, topic, 4, WithTag("sdk")); err != nil {
		t.Fatal(err)
	}
	var next int
	nextCtx, stopNext := context.WithCancel(ctx)
	defer stopNext()
	newPull, err := NewPullConsumer(nextCtx, cfg, group, WithPullConsumeFromWhere(ConsumeFromFirstOffset))
	if err != nil {
		t.Fatal(err)
	}
	defer newPull.Close()
	if err := newPull.RunBatch(nextCtx, topic, "sdk", batchConfig, func(ctx context.Context, batch []Message) error {
		if err := verifyBatch(ctx, batch, []int{4}); err != nil {
			return err
		}
		next += len(batch)
		stopNext()
		return nil
	}); err != nil || next != 1 {
		t.Fatalf("同组重启处理 %d 条新消息，返回 %v", next, err)
	}
}
