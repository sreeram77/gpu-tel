//go:generate mockgen -destination=mocks/message_store_mock.go -package=mocks github.com/sreeram77/gpu-tel/internal/mq/storage MessageStore

package storage

// This file contains the go:generate directive for mockgen. The actual mocks will be generated in the mocks package.
