package storage

import (
	"context"
	"testing"
	"time"

	"github.com/sreeram77/gpu-tel/api/v1/mq"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestInMemoryStore_StoreAndGet(t *testing.T) {
	store := NewInMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Test storing and retrieving a message
	msg := &mq.Message{
		Id:      "test-msg-1",
		Topic:   "test-topic",
		Payload: []byte("test payload"),
	}

	err := store.Store(ctx, msg)
	assert.NoError(t, err)

	// Retrieve the message
	retrieved, err := store.Get(ctx, "test-msg-1")
	assert.NoError(t, err)
	assert.Equal(t, msg.Id, retrieved.Id)
	assert.Equal(t, msg.Topic, retrieved.Topic)
	assert.Equal(t, msg.Payload, retrieved.Payload)
}

func TestInMemoryStore_StoreWithGeneratedID(t *testing.T) {
	store := NewInMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Test storing a message without an ID
	msg := &mq.Message{
		Topic:   "test-topic",
		Payload: []byte("test payload"),
	}

	err := store.Store(ctx, msg)
	assert.NoError(t, err)
	assert.NotEmpty(t, msg.Id) // ID should be generated

	// Retrieve the message
	retrieved, err := store.Get(ctx, msg.Id)
	assert.NoError(t, err)
	assert.Equal(t, msg.Id, retrieved.Id)
}

func TestInMemoryStore_GetNonExistent(t *testing.T) {
	store := NewInMemoryStore()
	defer store.Close()

	ctx := context.Background()

	_, err := store.Get(ctx, "non-existent")
	assert.Equal(t, ErrMessageNotFound, err)
}

func TestInMemoryStore_Delete(t *testing.T) {
	store := NewInMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Store a message
	msg := &mq.Message{
		Id:      "test-msg-1",
		Topic:   "test-topic",
		Payload: []byte("test payload"),
	}

	err := store.Store(ctx, msg)
	assert.NoError(t, err)

	// Delete the message
	err = store.Delete(ctx, "test-msg-1")
	assert.NoError(t, err)

	// Try to get the deleted message
	_, err = store.Get(ctx, "test-msg-1")
	assert.Equal(t, ErrMessageNotFound, err)
}

func TestInMemoryStore_Subscribe(t *testing.T) {
	store := NewInMemoryStore()
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	topic := "test-topic"
	msgChan, err := store.Subscribe(ctx, topic)
	assert.NoError(t, err)

	// Send a message in a goroutine
	go func() {
		msg := &mq.Message{
			Id:      "test-msg-1",
			Topic:   topic,
			Payload: []byte("test payload"),
		}
		store.Store(ctx, msg)
	}()

	// Wait for the message
	select {
	case msg := <-msgChan:
		assert.Equal(t, "test-msg-1", msg.Id)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestInMemoryStore_Acknowledge(t *testing.T) {
	store := NewInMemoryStore()
	defer store.Close()

	ctx := context.Background()

	// Store a message
	msg := &mq.Message{
		Id:      "test-msg-1",
		Topic:   "test-topic",
		Payload: []byte("test payload"),
	}

	err := store.Store(ctx, msg)
	assert.NoError(t, err)

	// Acknowledge the message
	err = store.Acknowledge(ctx, "test-msg-1", "test-consumer")
	assert.NoError(t, err)

	// Verify the message is marked as acknowledged
	store.Lock()
	defer store.Unlock()
	assert.Equal(t, "test-consumer", store.ackTracker["test-msg-1"])
}

func TestInMemoryStore_AddAndRemoveSubscriber(t *testing.T) {
	store := NewInMemoryStore()
	defer store.Close()

	ctx := context.Background()

	subscriber := Subscriber{
		ID:        "test-subscriber",
		Topic:     "test-topic",
		BatchSize: 10,
	}

	// Add subscriber
	err := store.AddSubscriber(ctx, subscriber)
	assert.NoError(t, err)

	// Remove subscriber
	err = store.RemoveSubscriber(ctx, "test-subscriber", "test-topic")
	assert.NoError(t, err)
}

func TestInMemoryStore_GetMessages(t *testing.T) {
	store := NewInMemoryStore()
	defer store.Close()

	ctx := context.Background()
	topic := "test-topic"
	consumerID := "test-consumer"

	// Add some test messages
	for i := 0; i < 5; i++ {
		msg := &mq.Message{
			Id:        "test-msg-" + string(rune('a'+i)),
			Topic:     topic,
			Payload:   []byte("test payload " + string(rune('a'+i))),
			Timestamp: timestamppb.Now(),
		}
		err := store.Store(ctx, msg)
		assert.NoError(t, err)
	}

	// Get messages
	messages, err := store.GetMessages(ctx, topic, consumerID, 3)
	assert.NoError(t, err)
	assert.Len(t, messages, 3)

	// Acknowledge the messages
	for _, msg := range messages {
		err = store.Acknowledge(ctx, msg.Id, consumerID)
		assert.NoError(t, err)
	}

	// Get more messages (should get the remaining 2)
	messages, err = store.GetMessages(ctx, topic, consumerID, 3)
	assert.NoError(t, err)
	assert.Len(t, messages, 2)
}

func TestInMemoryStore_Close(t *testing.T) {
	store := NewInMemoryStore()

	// Add some test data
	store.Lock()
	store.messages["test-msg"] = &mq.Message{Id: "test-msg"}
	store.topics["test-topic"] = map[string]bool{"test-msg": true}
	store.Unlock()

	// Close the store
	err := store.Close()
	assert.NoError(t, err)

	// Verify all data is cleared
	store.Lock()
	defer store.Unlock()
	assert.Empty(t, store.messages)
	assert.Empty(t, store.topics)
}

func TestInMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewInMemoryStore()
	defer store.Close()

	ctx := context.Background()
	numMessages := 100
	done := make(chan bool)

	// Start a subscriber
	msgChan, err := store.Subscribe(ctx, "test-topic")
	assert.NoError(t, err)

	// Start a goroutine to publish messages
	go func() {
		for i := 0; i < numMessages; i++ {
			msg := &mq.Message{
				Id:      "msg-" + string(rune(i)),
				Topic:   "test-topic",
				Payload: []byte("test payload " + string(rune(i))),
			}
			store.Store(ctx, msg)
			time.Sleep(time.Millisecond) // Allow other goroutines to run
		}
		done <- true
	}()

	// Start a goroutine to consume messages
	go func() {
		received := 0
		for {
			select {
			case <-msgChan:
				received++
				if received == numMessages {
					done <- true
					return
				}
			case <-time.After(2 * time.Second):
				t.Error("Timeout waiting for messages")
				return
			}
		}
	}()

	// Wait for both goroutines to complete
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("Test timed out")
		}
	}
}
