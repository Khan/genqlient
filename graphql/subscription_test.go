package graphql

import (
	"testing"
)

func Test_subscriptionMap_Unsubscribe(t *testing.T) {
	type args struct {
		subscriptionID string
	}
	tests := []struct {
		name    string
		args    args
		sm      subscriptionMap
		wantErr bool
	}{
		{
			name: "unsubscribe existing subscription",
			sm: subscriptionMap{
				map_: map[string]subscription{
					"sub1": {
						id:                  "sub1",
						interfaceChan:       make(chan struct{}),
						forwardDataFunc:     nil,
						hasBeenUnsubscribed: false,
					},
				},
			},
			args:    args{subscriptionID: "sub1"},
			wantErr: false,
		},
		{
			name: "unsubscribe non-existent subscription",
			sm: subscriptionMap{
				map_: map[string]subscription{},
			},
			args:    args{subscriptionID: "doesnotexist"},
			wantErr: true,
		},
		{
			name: "unsubscribe already unsubscribed subscription",
			sm: subscriptionMap{
				map_: map[string]subscription{
					"sub2": {
						id:                  "sub2",
						interfaceChan:       nil,
						forwardDataFunc:     nil,
						hasBeenUnsubscribed: true,
					},
				},
			},
			args:    args{subscriptionID: "sub2"},
			wantErr: false,
		},
	}
	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			s := &tt.sm
			if err := s.Unsubscribe(tt.args.subscriptionID); (err != nil) != tt.wantErr {
				t.Errorf("subscriptionMap.Unsubscribe() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
