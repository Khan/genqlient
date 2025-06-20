package graphql

import (
	"encoding/json"
	"sync"
	"testing"
)

const testSubscriptionID = "test-subscription-id"

func forgeTestWebSocketClient(hasBeenUnsubscribed bool) *webSocketClient {
	return &webSocketClient{
		subscriptions: subscriptionMap{
			RWMutex: sync.RWMutex{},
			map_: map[string]subscription{
				testSubscriptionID: {
					hasBeenUnsubscribed: hasBeenUnsubscribed,
					interfaceChan:       make(chan any),
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
