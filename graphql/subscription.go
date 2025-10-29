package graphql

import (
	"fmt"
	"reflect"
	"sync"
)

// map of subscription ID to subscription
type subscriptionMap struct {
	map_ map[string]subscription
	sync.RWMutex
}

type subscription struct {
	interfaceChan       interface{}
	forwardDataFunc     ForwardDataFunction
	id                  string
	hasBeenUnsubscribed bool
}

func (s *subscriptionMap) Create(subscriptionID string, interfaceChan interface{}, forwardDataFunc ForwardDataFunction) {
	s.Lock()
	defer s.Unlock()
	s.map_[subscriptionID] = subscription{
		id:                  subscriptionID,
		interfaceChan:       interfaceChan,
		forwardDataFunc:     forwardDataFunc,
		hasBeenUnsubscribed: false,
	}
}

func (s *subscriptionMap) Unsubscribe(subscriptionID string) error {
	s.Lock()
	defer s.Unlock()
	unsub, success := s.map_[subscriptionID]
	if !success {
		return fmt.Errorf("tried to unsubscribe from unknown subscription with ID '%s'", subscriptionID)
	}
	hasBeenUnsubscribed := unsub.hasBeenUnsubscribed
	unsub.hasBeenUnsubscribed = true
	s.map_[subscriptionID] = unsub

	if !hasBeenUnsubscribed {
		safeClose(unsub.interfaceChan)
	}

	return nil
}

func (s *subscriptionMap) GetAllIDs() (subscriptionIDs []string) {
	s.RLock()
	defer s.RUnlock()
	for subID := range s.map_ {
		subscriptionIDs = append(subscriptionIDs, subID)
	}
	return subscriptionIDs
}

func (s *subscriptionMap) Delete(subscriptionID string) {
	s.Lock()
	defer s.Unlock()
	delete(s.map_, subscriptionID)
}

func (s *subscriptionMap) GetOrClose(subscriptionID string, subscriptionType string) (*subscription, error) {
	s.Lock()
	defer s.Unlock()
	sub, success := s.map_[subscriptionID]
	if !success {
		return nil, fmt.Errorf("received message for unknown subscription ID '%s'", subscriptionID)
	}
	if sub.hasBeenUnsubscribed {
		return nil, nil
	}
	if subscriptionType == webSocketTypeComplete {
		sub.hasBeenUnsubscribed = true
		s.map_[subscriptionID] = sub
		reflect.ValueOf(sub.interfaceChan).Close()
		return nil, nil
	}

	return &sub, nil
}
