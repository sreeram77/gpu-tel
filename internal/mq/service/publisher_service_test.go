package service

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sreeram77/gpu-tel/api/v1/mq"
	storage_mocks "github.com/sreeram77/gpu-tel/internal/mq/storage/mocks"
)

type mockPublishServer struct {
	mq.PublisherService_PublishServer
	messages    []*mq.PublishRequest
	err         error
	sendErr     error
	ctx         context.Context
	returnError bool
}

func (m *mockPublishServer) Recv() (*mq.PublishRequest, error) {
	if m.returnError && m.err != nil {
		return nil, m.err
	}

	if len(m.messages) == 0 {
		// Return EOF when there are no more messages
		return nil, errors.New("EOF")
	}
	msg := m.messages[0]
	m.messages = m.messages[1:]
	return msg, nil
}

func (m *mockPublishServer) Send(resp *mq.PublishResponse) error {
	return m.sendErr
}

func (m *mockPublishServer) Context() context.Context {
	if m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

func createTestMessage() *mq.Message {
	return &mq.Message{
		Id:      "test-msg",
		Topic:   "test-topic",
		Payload: []byte("test-payload"),
	}
}

func TestPublisherService_Publish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := storage_mocks.NewMockMessageStore(ctrl)
	logger := zerolog.New(zerolog.NewTestWriter(t))
	service := &PublisherService{
		logger:      logger,
		messageStore: mockStore,
	}

	tests := []struct {
		name        string
		setupMocks  func() *mockPublishServer
		expectError bool
		errCode     codes.Code
	}{
		{
			name: "successful message publish",
			setupMocks: func() *mockPublishServer {
				msg := createTestMessage()

				mockStore.EXPECT().
					Store(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, m *mq.Message) error {
						assert.Equal(t, msg.Id, m.Id)
						assert.Equal(t, msg.Topic, m.Topic)
						assert.Equal(t, msg.Payload, m.Payload)
						return nil
					}).
					Times(1)

				return &mockPublishServer{
					messages: []*mq.PublishRequest{{
						Message:    msg,
						WaitForAck: false,
					}},
					ctx: context.Background(),
				}
			},
			expectError: false,
		},
		{
			name: "error on message store",
			setupMocks: func() *mockPublishServer {
				msg := createTestMessage()

				mockStore.EXPECT().
					Store(gomock.Any(), gomock.Any()).
					Return(errors.New("storage error")).
					Times(1)

				return &mockPublishServer{
					messages: []*mq.PublishRequest{{
						Message:    msg,
						WaitForAck: false,
					}},
					ctx: context.Background(),
				}
			},
			expectError: true,
			errCode:     codes.Internal,
		},
		{
			name: "error on receive",
			setupMocks: func() *mockPublishServer {
				// Don't set up any mock expectations since we expect an error on receive
				return &mockPublishServer{
					err:         errors.New("receive error"),
					returnError: true, // This will make Recv() return the error
					ctx:         context.Background(),
				}
			},
			expectError: true,
			errCode:     codes.Internal, // The service should convert the receive error to Internal
		},
		{
			name: "nil message",
			setupMocks: func() *mockPublishServer {
				return &mockPublishServer{
					messages: []*mq.PublishRequest{{}},
					ctx:      context.Background(),
				}
			},
			expectError: true,
			errCode:     codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupMocks()
			err := service.Publish(server)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errCode != codes.Unknown {
					st, ok := status.FromError(err)
					if ok {
						assert.Equal(t, tt.errCode, st.Code())
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPublisherService_HealthCheck(t *testing.T) {
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

	service := &PublisherService{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := service.HealthCheck(context.Background(), tt.request)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.Status, resp.Status)
		})
	}
}

func TestNewPublisherService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := storage_mocks.NewMockMessageStore(ctrl)
	logger := zerolog.Nop()

	service := NewPublisherService(logger, mockStore)
	assert.NotNil(t, service)
}
