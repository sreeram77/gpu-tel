package storage

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/sreeram77/gpu-tel/api/v1/mq"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Subscriber represents a message subscriber
type Subscriber struct {
	ID            string
	Topic         string
	BatchSize     int
	MessageFilter map[string]string
}

// MessageStore defines the interface for message storage operations
type MessageStore interface {
	// Store stores a message in the store
	Store(ctx context.Context, msg *mq.Message) error
	// Get retrieves a message by ID
	Get(ctx context.Context, id string) (*mq.Message, error)
	// Delete removes a message from the store
	Delete(ctx context.Context, id string) error
	// Subscribe returns a channel that receives messages for the given topic
	Subscribe(ctx context.Context, topic string) (<-chan *mq.Message, error)
	// Acknowledge acknowledges processing of a message
	Acknowledge(ctx context.Context, msgID string, consumerID string) error
	// AddSubscriber adds a new subscriber
	AddSubscriber(ctx context.Context, subscriber Subscriber) error
	// RemoveSubscriber removes a subscriber
	RemoveSubscriber(ctx context.Context, consumerID, topic string) error
	// GetMessages gets a batch of messages for a subscriber
	GetMessages(ctx context.Context, topic, consumerID string, batchSize int) ([]*mq.Message, error)
	// Close cleans up resources used by the store
	Close() error
}

// InMemoryStore is an in-memory implementation of MessageStore
type InMemoryStore struct {
	sync.RWMutex
	messages       map[string]*mq.Message
	topics         map[string]map[string]bool           // topic -> message IDs
	consumers      map[string]map[string]bool           // consumer ID -> message IDs
	ackTracker     map[string]string                    // message ID -> consumer ID
	subscribers    map[string]map[chan *mq.Message]bool // topic -> subscribers
	subscriberInfo map[string]Subscriber                // consumer ID -> Subscriber
}

// NewInMemoryStore creates a new in-memory message store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		messages:       make(map[string]*mq.Message),
		topics:         make(map[string]map[string]bool),
		consumers:      make(map[string]map[string]bool),
		ackTracker:     make(map[string]string),
		subscribers:    make(map[string]map[chan *mq.Message]bool),
		subscriberInfo: make(map[string]Subscriber),
	}
}

// Store stores a message in the in-memory store
func (s *InMemoryStore) Store(ctx context.Context, msg *mq.Message) error {
	s.Lock()
	defer s.Unlock()

	// Set message ID if not set
	if msg.Id == "" {
		msg.Id = uuid.New().String()
	}

	// Set timestamp if not set
	if msg.Timestamp == nil {
		msg.Timestamp = timestamppb.Now()
	}

	// Store the message
	s.messages[msg.Id] = msg

	// Add message ID to topic
	if _, ok := s.topics[msg.Topic]; !ok {
		s.topics[msg.Topic] = make(map[string]bool)
	}
	s.topics[msg.Topic][msg.Id] = true

	// Notify subscribers
	if subs, ok := s.subscribers[msg.Topic]; ok {
		for ch := range subs {
			select {
			case ch <- msg:
				// Message sent to subscriber
			default:
				// Skip if subscriber's channel is full
			}
		}
	}

	return nil
}

// Get retrieves a message by ID
func (s *InMemoryStore) Get(ctx context.Context, id string) (*mq.Message, error) {
	s.RLock()
	defer s.RUnlock()

	msg, ok := s.messages[id]
	if !ok {
		return nil, ErrMessageNotFound
	}

	return msg, nil
}

// Delete removes a message from the store
func (s *InMemoryStore) Delete(ctx context.Context, id string) error {
	s.Lock()
	defer s.Unlock()

	delete(s.messages, id)
	delete(s.ackTracker, id)
	return nil
}

// Subscribe returns a channel that receives messages for the given topic
func (s *InMemoryStore) Subscribe(ctx context.Context, topic string) (<-chan *mq.Message, error) {
	s.Lock()

	// Create channel for this subscription
	ch := make(chan *mq.Message, 100) // Buffered channel to prevent blocking

	// Initialize topic map if it doesn't exist
	if _, ok := s.subscribers[topic]; !ok {
		s.subscribers[topic] = make(map[chan *mq.Message]bool)
	}

	// Add channel to subscriptions
	s.subscribers[topic][ch] = true
	s.Unlock()

	// Start a goroutine to clean up when the context is done
	go func() {
		<-ctx.Done()
		s.Lock()
		if chMap, ok := s.subscribers[topic]; ok {
			if _, chExists := chMap[ch]; chExists {
				delete(chMap, ch)
				close(ch)
			}
		}
		s.Unlock()
	}()

	return ch, nil
}

// Acknowledge acknowledges processing of a message by a consumer
func (s *InMemoryStore) Acknowledge(ctx context.Context, msgID string, consumerID string) error {
	s.Lock()
	defer s.Unlock()

	if _, exists := s.messages[msgID]; !exists {
		return ErrMessageNotFound
	}

	s.ackTracker[msgID] = consumerID
	return nil
}

// AddSubscriber adds a new subscriber
func (s *InMemoryStore) AddSubscriber(ctx context.Context, subscriber Subscriber) error {
	s.Lock()
	defer s.Unlock()

	s.subscriberInfo[subscriber.ID] = subscriber
	return nil
}

// RemoveSubscriber removes a subscriber
func (s *InMemoryStore) RemoveSubscriber(ctx context.Context, consumerID, topic string) error {
	s.Lock()
	defer s.Unlock()

	delete(s.subscriberInfo, consumerID)
	return nil
}

// GetMessages gets a batch of messages for a subscriber
func (s *InMemoryStore) GetMessages(ctx context.Context, topic, consumerID string, batchSize int) ([]*mq.Message, error) {
	s.Lock()
	defer s.Unlock()

	var messages []*mq.Message
	count := 0

	// Get all message IDs for the topic
	messageIDs, ok := s.topics[topic]
	if !ok {
		return messages, nil
	}

	// Get messages that haven't been acknowledged by this consumer
	for msgID := range messageIDs {
		if count >= batchSize {
			break
		}

		// Skip if message is already acknowledged by this consumer
		if consumerID != "" {
			if ackConsumer, exists := s.ackTracker[msgID]; exists && ackConsumer == consumerID {
				continue
			}
		}

		msg, exists := s.messages[msgID]
		if exists {
			messages = append(messages, msg)
			count++

			// Track this message as in-flight for the consumer
			if consumerID != "" {
				s.ackTracker[msgID] = consumerID
			}
		}
	}

	return messages, nil
}

// Close cleans up resources used by the store
func (s *InMemoryStore) Close() error {
	s.Lock()
	defer s.Unlock()

	// Close all subscriber channels
	for _, subs := range s.subscribers {
		for ch := range subs {
			close(ch)
		}
	}

	// Clear all maps
	s.messages = make(map[string]*mq.Message)
	s.topics = make(map[string]map[string]bool)
	s.consumers = make(map[string]map[string]bool)
	s.ackTracker = make(map[string]string)
	s.subscribers = make(map[string]map[chan *mq.Message]bool)
	s.subscriberInfo = make(map[string]Subscriber)

	return nil
}

// Error definitions
var (
	ErrMessageNotFound = &MessageError{"message not found"}
)

// MessageError represents an error in the message store
type MessageError struct {
	message string
}

func (e *MessageError) Error() string {
	return e.message
}
