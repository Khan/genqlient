package graphql

import (
	"testing"
)

func Test_subscriptionMap_closeChannel(t *testing.T) {
	tests := []struct {
		name           string
		sm             subscriptionMap
		subscriptionID string
		wantClosed     bool
	}{
		{
			name: "close existing open channel",
			sm: subscriptionMap{
				map_: map[string]subscription{
					"sub1": {
						id:            "sub1",
						interfaceChan: make(chan struct{}),
						closed:        false,
					},
				},
			},
			subscriptionID: "sub1",
			wantClosed:     true,
		},
		{
			name: "close already closed channel",
			sm: subscriptionMap{
				map_: map[string]subscription{
					"sub2": {
						id:            "sub2",
						interfaceChan: make(chan struct{}),
						closed:        true,
					},
				},
			},
			subscriptionID: "sub2",
			wantClosed:     false,
		},
		{
			name: "close non-existent subscription",
			sm: subscriptionMap{
				map_: map[string]subscription{},
			},
			subscriptionID: "doesnotexist",
			wantClosed:     false,
		},
	}
	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			s := &tt.sm
			gotClosed := s.closeChannel(tt.subscriptionID)
			if gotClosed != tt.wantClosed {
				t.Errorf("subscriptionMap.closeChannel() = %v, want %v", gotClosed, tt.wantClosed)
			}
		})
	}
}
