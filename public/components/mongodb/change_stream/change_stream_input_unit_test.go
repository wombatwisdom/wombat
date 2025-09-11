package change_stream

import (
	"context"
	"errors"
	"testing"

	"github.com/redpanda-data/benthos/v4/public/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wombatwisdom/wombat/public/components/mongodb"
)

func TestNewChangeStreamInput(t *testing.T) {
	t.Run("should create input with valid configuration", func(t *testing.T) {
		spec := service.NewConfigSpec().
			Fields(mongodb.Fields...).
			Field(service.NewStringField("database").Optional().Default("")).
			Field(service.NewStringField("collection").Optional().Default(""))

		env := service.NewEnvironment()
		confStr := `
uri: mongodb://localhost:27017
database: test_db
collection: test_collection
`
		parsedConf, err := spec.ParseYAML(confStr, env)
		require.NoError(t, err)

		input, err := newChangeStreamInput(parsedConf, service.MockResources())
		assert.NoError(t, err)
		assert.NotNil(t, input)
		assert.NotNil(t, input.reader)
	})

	t.Run("should create input with minimal configuration", func(t *testing.T) {
		spec := service.NewConfigSpec().
			Fields(mongodb.Fields...).
			Field(service.NewStringField("database").Optional().Default("")).
			Field(service.NewStringField("collection").Optional().Default(""))

		env := service.NewEnvironment()
		confStr := `
uri: mongodb://localhost:27017
`
		parsedConf, err := spec.ParseYAML(confStr, env)
		require.NoError(t, err)

		input, err := newChangeStreamInput(parsedConf, service.MockResources())
		assert.NoError(t, err)
		assert.NotNil(t, input)
		assert.NotNil(t, input.reader)
	})

	t.Run("should fail with invalid URI configuration", func(t *testing.T) {
		spec := service.NewConfigSpec().
			Field(service.NewStringField("other_field")).
			Field(service.NewStringField("database").Optional().Default("")).
			Field(service.NewStringField("collection").Optional().Default(""))

		env := service.NewEnvironment()
		confStr := `
other_field: value
`
		parsedConf, err := spec.ParseYAML(confStr, env)
		require.NoError(t, err)

		input, err := newChangeStreamInput(parsedConf, service.MockResources())
		assert.Error(t, err)
		assert.Nil(t, input)
		assert.Contains(t, err.Error(), "uri")
	})

	t.Run("should handle database field error", func(t *testing.T) {
		// Create a configuration that will cause FieldString("database") to fail
		spec := service.NewConfigSpec().
			Fields(mongodb.Fields...).
			Field(service.NewIntField("database")) // Wrong type - should be string

		env := service.NewEnvironment()
		confStr := `
uri: mongodb://localhost:27017
database: 123
`
		parsedConf, err := spec.ParseYAML(confStr, env)
		require.NoError(t, err)

		input, err := newChangeStreamInput(parsedConf, service.MockResources())
		assert.Error(t, err)
		assert.Nil(t, input)
	})

	t.Run("should handle collection field error", func(t *testing.T) {
		// Create a configuration that will cause FieldString("collection") to fail
		spec := service.NewConfigSpec().
			Fields(mongodb.Fields...).
			Field(service.NewStringField("database").Optional().Default("")).
			Field(service.NewIntField("collection")) // Wrong type - should be string

		env := service.NewEnvironment()
		confStr := `
uri: mongodb://localhost:27017
collection: 123
`
		parsedConf, err := spec.ParseYAML(confStr, env)
		require.NoError(t, err)

		input, err := newChangeStreamInput(parsedConf, service.MockResources())
		assert.Error(t, err)
		assert.Nil(t, input)
	})
}

func TestChangeStreamInputConnect(t *testing.T) {
	t.Run("should delegate to reader Connect", func(t *testing.T) {
		// Create a mock reader
		mockReader := &MockChangeStreamReader{}
		input := &changeStreamInput{reader: mockReader}

		ctx := context.Background()
		expectedError := errors.New("connection failed")
		mockReader.connectError = expectedError

		err := input.Connect(ctx)
		assert.Equal(t, expectedError, err)
		assert.True(t, mockReader.connectCalled)
	})

	t.Run("should handle successful connection", func(t *testing.T) {
		mockReader := &MockChangeStreamReader{}
		input := &changeStreamInput{reader: mockReader}

		ctx := context.Background()
		err := input.Connect(ctx)
		assert.NoError(t, err)
		assert.True(t, mockReader.connectCalled)
	})
}

func TestChangeStreamInputRead(t *testing.T) {
	t.Run("should return ErrNotConnected when reader is nil", func(t *testing.T) {
		input := &changeStreamInput{reader: nil}
		ctx := context.Background()

		msg, ackFn, err := input.Read(ctx)
		assert.Nil(t, msg)
		assert.Nil(t, ackFn)
		assert.Equal(t, service.ErrNotConnected, err)
	})

	t.Run("should delegate to reader Read and return message", func(t *testing.T) {
		mockReader := &MockChangeStreamReader{}
		input := &changeStreamInput{reader: mockReader}

		ctx := context.Background()
		expectedMsg := service.NewMessage([]byte(`{"test": "data"}`))
		mockReader.readMessage = expectedMsg

		msg, ackFn, err := input.Read(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedMsg, msg)
		assert.NotNil(t, ackFn)
		assert.True(t, mockReader.readCalled)

		// Test ack function
		ackErr := ackFn(ctx, nil)
		assert.NoError(t, ackErr)

		testErr := errors.New("test error")
		ackErr = ackFn(ctx, testErr)
		assert.Equal(t, testErr, ackErr)
	})

	t.Run("should return error from reader Read", func(t *testing.T) {
		mockReader := &MockChangeStreamReader{}
		input := &changeStreamInput{reader: mockReader}

		ctx := context.Background()
		expectedError := errors.New("read failed")
		mockReader.readError = expectedError

		msg, ackFn, err := input.Read(ctx)
		assert.Nil(t, msg)
		assert.Nil(t, ackFn)
		assert.Equal(t, expectedError, err)
		assert.True(t, mockReader.readCalled)
	})
}

func TestChangeStreamInputClose(t *testing.T) {
	t.Run("should delegate to reader Close", func(t *testing.T) {
		mockReader := &MockChangeStreamReader{}
		input := &changeStreamInput{reader: mockReader}

		ctx := context.Background()
		expectedError := errors.New("close failed")
		mockReader.closeError = expectedError

		err := input.Close(ctx)
		assert.Equal(t, expectedError, err)
		assert.True(t, mockReader.closeCalled)
	})

	t.Run("should handle successful close", func(t *testing.T) {
		mockReader := &MockChangeStreamReader{}
		input := &changeStreamInput{reader: mockReader}

		ctx := context.Background()
		err := input.Close(ctx)
		assert.NoError(t, err)
		assert.True(t, mockReader.closeCalled)
	})
}

// MockChangeStreamReader is a mock implementation for testing
type MockChangeStreamReader struct {
	connectCalled bool
	connectError  error
	readCalled    bool
	readMessage   *service.Message
	readError     error
	closeCalled   bool
	closeError    error
}

func (m *MockChangeStreamReader) Connect(ctx context.Context) error {
	m.connectCalled = true
	return m.connectError
}

func (m *MockChangeStreamReader) Read(ctx context.Context) (*service.Message, error) {
	m.readCalled = true
	if m.readError != nil {
		return nil, m.readError
	}
	return m.readMessage, nil
}

func (m *MockChangeStreamReader) Close(ctx context.Context) error {
	m.closeCalled = true
	return m.closeError
}
