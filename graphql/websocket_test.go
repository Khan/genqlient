package graphql

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

const testSubscriptionID = "test-subscription-id"

func forgeTestWebSocketClient(hasBeenUnsubscribed bool) *webSocketClient {
	return &webSocketClient{
		subscriptions: subscriptionMap{
			RWMutex: sync.RWMutex{},
			map_: map[string]*subscription{
				testSubscriptionID: {
					_hasBeenUnsubscribed: hasBeenUnsubscribed,
					interfaceChan:        make(chan any),
					forwardDataFunc: func(interfaceChan any, jsonRawMsg json.RawMessage) error {
						return nil
					},
				},
			},
		},
	}
}

func Test_webSocketClient_forwardWebSocketData(t *testing.T) {
	type args struct {
		message []byte
	}
	tests := []struct {
		wc      *webSocketClient
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "empty message",
			args:    args{message: []byte{}},
			wc:      forgeTestWebSocketClient(false),
			wantErr: true,
		},
		{
			name:    "nil message",
			args:    args{message: nil},
			wc:      forgeTestWebSocketClient(false),
			wantErr: true,
		},
		{
			name:    "unknown subscription id",
			args:    args{message: []byte(`{"type":"next","id":"unknown-id","payload":{}}`)},
			wc:      forgeTestWebSocketClient(false),
			wantErr: true,
		},
		{
			name:    "void subscription ID",
			args:    args{message: []byte(`{"type":"next","id":"","payload":{}}`)},
			wc:      forgeTestWebSocketClient(false),
			wantErr: false,
		},
		{
			name:    "unsubscribed subscription",
			args:    args{message: []byte(`{"type":"next","id":"test-subscription-id","payload":{}}`)},
			wc:      forgeTestWebSocketClient(true),
			wantErr: false,
		},
		{
			name:    "complete message closes channel",
			args:    args{message: []byte(`{"type":"complete","id":"test-subscription-id","payload":{}}`)},
			wc:      forgeTestWebSocketClient(false),
			wantErr: false,
		},
		{
			name:    "valid next message",
			args:    args{message: []byte(`{"type":"next","id":"test-subscription-id","payload":{"foo":"bar"}}`)},
			wc:      forgeTestWebSocketClient(false),
			wantErr: false,
		},
	}
	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Running test: %s", tt.name)

			if err := tt.wc.forwardWebSocketData(tt.args.message); (err != nil) != tt.wantErr {
				t.Errorf("%s: webSocketClient.forwardWebSocketData() error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}
		})
	}
}

type blockingWSConn struct {
	readStarted chan struct{}
	readRelease chan struct{}
	closeOnce   sync.Once
}

func (c *blockingWSConn) Close() error {
	c.closeOnce.Do(func() {
		close(c.readRelease)
	})
	return nil
}

func (c *blockingWSConn) WriteMessage(messageType int, data []byte) error {
	return nil
}

func (c *blockingWSConn) ReadMessage() (messageType int, p []byte, err error) {
	close(c.readStarted)
	<-c.readRelease
	return 0, nil, errors.New("connection closed")
}

func Test_webSocketClient_CloseClosesErrChanWithoutSendingError(t *testing.T) {
	conn := &blockingWSConn{
		readStarted: make(chan struct{}),
		readRelease: make(chan struct{}),
	}
	wc := &webSocketClient{
		conn:    conn,
		errChan: make(chan error),
		subscriptions: subscriptionMap{
			RWMutex: sync.RWMutex{},
			map_:    map[string]*subscription{},
		},
	}

	go wc.listenWebSocket()

	select {
	case <-conn.readStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ReadMessage")
	}

	if err := wc.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	select {
	case err, ok := <-wc.errChan:
		if ok {
			t.Fatalf("expected errChan to be closed, got error %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for errChan to close")
	}
}
