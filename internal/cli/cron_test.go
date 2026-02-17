package cli

import (
	"testing"

	"github.com/Lichas/nanobot-go/internal/bus"
	"github.com/Lichas/nanobot-go/internal/cron"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCronUserMessage(t *testing.T) {
	job := &cron.Job{
		Name: "hello",
		Payload: cron.Payload{
			Channel: "telegram",
			Message: "say hi",
		},
	}

	msg := buildCronUserMessage(job)
	assert.Equal(t, "[telegram] [Cron Job: hello] say hi", msg)
}

func TestEnqueueCronJob(t *testing.T) {
	messageBus := bus.NewMessageBus(1)
	job := &cron.Job{
		ID:   "job_1",
		Name: "hello",
		Payload: cron.Payload{
			Channel: "telegram",
			To:      "chat-42",
			Message: "say hi",
			Deliver: true,
		},
	}

	result, err := enqueueCronJob(messageBus, job)
	require.NoError(t, err)
	assert.Contains(t, result, "enqueued cron job")

	msg, ok := messageBus.TryConsumeInbound()
	require.True(t, ok)
	assert.Equal(t, "telegram", msg.Channel)
	assert.Equal(t, "chat-42", msg.ChatID)
	assert.Equal(t, "cron", msg.SenderID)
	assert.Equal(t, "[telegram] [Cron Job: hello] say hi", msg.Content)
}

func TestEnqueueCronJobValidation(t *testing.T) {
	job := &cron.Job{
		ID:   "job_1",
		Name: "hello",
		Payload: cron.Payload{
			Channel: "telegram",
			To:      "chat-42",
			Message: "say hi",
			Deliver: true,
		},
	}

	_, err := enqueueCronJob(nil, job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message bus not available")

	_, err = enqueueCronJob(bus.NewMessageBus(1), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "job is nil")

	_, err = enqueueCronJob(bus.NewMessageBus(1), &cron.Job{
		Payload: cron.Payload{
			To:      "chat-42",
			Message: "say hi",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel is empty")

	_, err = enqueueCronJob(bus.NewMessageBus(1), &cron.Job{
		Payload: cron.Payload{
			Channel: "telegram",
			Message: "say hi",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target is empty")
}

func TestEnqueueCronJobBufferFull(t *testing.T) {
	messageBus := bus.NewMessageBus(1)
	require.NoError(t, messageBus.PublishInbound(bus.NewInboundMessage("telegram", "u", "c", "seed")))

	_, err := enqueueCronJob(messageBus, &cron.Job{
		ID:   "job_2",
		Name: "hello",
		Payload: cron.Payload{
			Channel: "telegram",
			To:      "chat-42",
			Message: "say hi",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to enqueue cron job")
}
