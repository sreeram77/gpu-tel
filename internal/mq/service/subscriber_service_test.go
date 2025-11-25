package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/sreeram77/gpu-tel/api/v1/mq"
	"github.com/sreeram77/gpu-tel/internal/mq/storage"
	storage_mocks "github.com/sreeram77/gpu-tel/internal/mq/storage/mocks"
)

type mockSubscribeServer struct {
	mq.SubscriberService_SubscribeServer
	msgsToSend    []*mq.SubscribeRequest
	responses     []*mq.SubscribeResponse
	sendErr       error
	recvErr       error
	ctx           context.Context
	sendCalled    chan struct{}
	recvCalled    chan struct{}
	sendResponses []*mq.SubscribeResponse
}

func (m *mockSubscribeServer) Send(resp *mq.SubscribeResponse) error {
	m.responses = append(m.responses, resp)
	m.sendResponses = append(m.sendResponses, resp)
	if m.sendCalled != nil {
		close(m.sendCalled)
	}
	return m.sendErr
}

func (m *mockSubscribeServer) Recv() (*mq.SubscribeRequest, error) {
	if m.recvCalled != nil {
		close(m.recvCalled)
	}

	if len(m.msgsToSend) == 0 {
		// If no more messages to send, wait for context cancellation
		if m.ctx != nil {
			<-m.ctx.Done()
		}
		return nil, m.recvErr
	}

	msg := m.msgsToSend[0]
	m.msgsToSend = m.msgsToSend[1:]
	return msg, nil
}

func (m *mockSubscribeServer) Context() context.Context {
	if m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

type mockAckServer struct {
	mq.SubscriberService_AcknowledgeServer
	msgsToSend  []*mq.AcknowledgeRequest
	sendErr     error
	recvErr     error
	ctx         context.Context
	sendCalled  bool
	recvCalled  bool
	done        chan struct{}
}

func (m *mockAckServer) Recv() (*mq.AcknowledgeRequest, error) {
	if m.recvCalled {
		if m.done != nil {
			close(m.done)
		}
		return nil, m.recvErr
	}

	if len(m.msgsToSend) == 0 {
		m.recvCalled = true
		if m.done != nil {
			close(m.done)
		}
		return nil, m.recvErr
	}

	msg := m.msgsToSend[0]
	m.msgsToSend = m.msgsToSend[1:]
	return msg, nil
}

func (m *mockAckServer) SendAndClose(*emptypb.Empty) error {
	m.sendCalled = true
	return m.sendErr
}

func (m *mockAckServer) Context() context.Context {
	if m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

func TestNewSubscriberService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := storage_mocks.NewMockMessageStore(ctrl)
	logger := zerolog.Nop()

	service := NewSubscriberService(logger, mockStore)
	assert.NotNil(t, service)
}

func TestSubscriberService_Subscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name        string
		setupMocks  func() (*storage_mocks.MockMessageStore, *mockSubscribeServer)
		expectError bool
		errCode     codes.Code
	}{
		{
			name: "successful subscription",
			setupMocks: func() (*storage_mocks.MockMessageStore, *mockSubscribeServer) {
				mockStore := storage_mocks.NewMockMessageStore(ctrl)

				// Expect AddSubscriber to be called with the correct topic and consumer ID
				mockStore.EXPECT().
					AddSubscriber(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, sub storage.Subscriber) error {
						assert.Equal(t, "test-topic", sub.Topic)
						assert.NotEmpty(t, sub.ID)
						return nil
					}).
					Times(1)

				// Create a cancellable context for the test
				ctx, cancel := context.WithCancel(context.Background())
				
				// Create a message channel with a small buffer
				msgChan := make(chan *mq.Message, 1)
				
				// Expect Subscribe to be called
				mockStore.EXPECT().
					Subscribe(gomock.Any(), "test-topic").
					DoAndReturn(func(ctx context.Context, topic string) (<-chan *mq.Message, error) {
						// Simulate a message being received
						go func() {
							msgChan <- &mq.Message{
								Id:      "test-msg",
								Topic:   "test-topic",
								Payload: []byte("test-payload"),
							}
							// Cancel the context after sending the message
							cancel()
						}()
						return msgChan, nil
					}).
					Times(1)

				// Expect RemoveSubscriber to be called when the stream is closed
				mockStore.EXPECT().
					RemoveSubscriber(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, consumerID, topic string) error {
						assert.Equal(t, "test-topic", topic)
						assert.NotEmpty(t, consumerID)
						return nil
					}).
					Times(1)

				server := &mockSubscribeServer{
					msgsToSend: []*mq.SubscribeRequest{{
						Topic: "test-topic",
					}},
					recvErr: context.Canceled,
					ctx:     ctx, // Use the cancellable context
				}

				return mockStore, server
			},
			expectError: false,
		},
		{
			name: "missing topic",
			setupMocks: func() (*storage_mocks.MockMessageStore, *mockSubscribeServer) {
				mockStore := storage_mocks.NewMockMessageStore(ctrl)
				server := &mockSubscribeServer{
					msgsToSend: []*mq.SubscribeRequest{{}},
					ctx:     context.Background(),
				}
				return mockStore, server
			},
			expectError: true,
			errCode:     codes.InvalidArgument,
		},
		{
			name: "error on add subscriber",
			setupMocks: func() (*storage_mocks.MockMessageStore, *mockSubscribeServer) {
				mockStore := storage_mocks.NewMockMessageStore(ctrl)
				mockStore.EXPECT().
					AddSubscriber(gomock.Any(), gomock.Any()).
					Return(errors.New("add subscriber error")).
					Times(1)

				server := &mockSubscribeServer{
					msgsToSend: []*mq.SubscribeRequest{{
						Topic: "test-topic",
					}},
				}

				return mockStore, server
			},
			expectError: true,
			errCode:     codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore, server := tt.setupMocks()
			service := NewSubscriberService(zerolog.Nop(), mockStore)

			err := service.Subscribe(server)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errCode != codes.Unknown {
					st, ok := status.FromError(err)
					assert.True(t, ok)
					assert.Equal(t, tt.errCode, st.Code())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSubscriberService_Acknowledge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name        string
		setupMocks  func() (*storage_mocks.MockMessageStore, *mockAckServer)
		expectError bool
		errCode     codes.Code
	}{
		{
			name: "successful ack",
			setupMocks: func() (*storage_mocks.MockMessageStore, *mockAckServer) {
				mockStore := storage_mocks.NewMockMessageStore(ctrl)
				mockStore.EXPECT().
					Acknowledge(gomock.Any(), "test-msg", "test-consumer").
					Return(nil).
					Times(1)

				done := make(chan struct{})
				server := &mockAckServer{
					msgsToSend: []*mq.AcknowledgeRequest{{
						MessageId:  "test-msg",
						ConsumerId: "test-consumer",
					}},
					recvErr: io.EOF, // Simulate end of stream after first message
					done:     done,
				}

				return mockStore, server
			},
			expectError: false,
		},
		{
			name: "error on ack",
			setupMocks: func() (*storage_mocks.MockMessageStore, *mockAckServer) {
				mockStore := storage_mocks.NewMockMessageStore(ctrl)
				mockStore.EXPECT().
					Acknowledge(gomock.Any(), "test-msg", "test-consumer").
					Return(errors.New("ack error")).
					Times(1)

				done := make(chan struct{})
				server := &mockAckServer{
					msgsToSend: []*mq.AcknowledgeRequest{{
						MessageId:  "test-msg",
						ConsumerId: "test-consumer",
					}},
					recvErr: io.EOF, // Simulate end of stream after first message
					done:     done,
				}

				return mockStore, server
			},
			expectError: true,
			errCode:     codes.Internal,
		},
		{
			name: "empty message id",
			setupMocks: func() (*storage_mocks.MockMessageStore, *mockAckServer) {
				mockStore := storage_mocks.NewMockMessageStore(ctrl)
				// No expectations for Acknowledge since we expect it to fail validation first

				done := make(chan struct{})
				server := &mockAckServer{
					msgsToSend: []*mq.AcknowledgeRequest{{
						MessageId:  "", // Empty message ID
						ConsumerId: "test-consumer",
					}},
					recvErr: nil, // No error on first recv
					done:     done,
				}

				return mockStore, server
			},
			expectError: true,
			errCode:     codes.InvalidArgument,
		},
		{
			name: "empty consumer id",
			setupMocks: func() (*storage_mocks.MockMessageStore, *mockAckServer) {
				mockStore := storage_mocks.NewMockMessageStore(ctrl)
				// No expectations for Acknowledge since we expect it to fail validation first

				done := make(chan struct{})
				server := &mockAckServer{
					msgsToSend: []*mq.AcknowledgeRequest{{
						MessageId:  "test-msg",
						ConsumerId: "", // Empty consumer ID
					}},
					recvErr: nil, // No error on first recv
					done:     done,
				}

				return mockStore, server
			},
			expectError: true,
			errCode:     codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore, server := tt.setupMocks()
			service := NewSubscriberService(zerolog.Nop(), mockStore)

			// Start the Acknowledge in a goroutine since it blocks
			errCh := make(chan error, 1)
			go func() {
				errCh <- service.Acknowledge(server)
			}()

			// For test cases where we expect an error, we need to close the done channel
			// after a short delay to prevent the test from hanging
			if tt.expectError && server.done != nil {
				go func() {
					time.Sleep(100 * time.Millisecond)
					close(server.done)
				}()
			}

			// Wait for the Acknowledge to complete or time out
			var err error
			select {
			case err = <-errCh:
				// Test completed
			case <-time.After(1 * time.Second):
				t.Fatal("test timed out waiting for Acknowledge to complete")
			}

			if tt.expectError {
				assert.Error(t, err)
				if tt.errCode != codes.Unknown {
					st, ok := status.FromError(err)
					assert.True(t, ok, "expected gRPC status error")
					if ok {
						assert.Equal(t, tt.errCode, st.Code())
					}
				}
			} else {
				assert.NoError(t, err)
			}

			// Clean up the done channel if it wasn't already closed
			if server.done != nil {
				select {
				case <-server.done:
					// Already closed
				default:
					close(server.done)
				}
			}
		})
	}
}

func TestSubscriberService_HealthCheck(t *testing.T) {
	tests := []struct {
		name     string
		request  *mq.HealthCheckRequest
		expected *mq.HealthCheckResponse
	}{
		{
			name:     "successful health check",
			request:  &mq.HealthCheckRequest{},
			expected: &mq.HealthCheckResponse{Status: mq.HealthCheckResponse_SERVING},
		},
	}

	service := &SubscriberService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.HealthCheck(context.Background(), tt.request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.Status, resp.Status)
		})
	}
}
