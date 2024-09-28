package graphql

import (
	"fmt"
	"sync"
)

// map of subscription ID to subscription
type subscriptionMap[T any] struct {
	map_ map[string]subscription[T]
	sync.RWMutex
}

type subscription[T any] struct {
	dataChan            chan WsResponse[T]
	forwardDataFunc     ForwardDataFunction[T]
	id                  string
	hasBeenUnsubscribed bool
}

func (s *subscriptionMap[T]) Create(subscriptionID string, dataChan chan WsResponse[T], forwardDataFunc ForwardDataFunction[T]) {
	s.Lock()
	defer s.Unlock()
	s.map_[subscriptionID] = subscription[T]{
		id:                  subscriptionID,
		dataChan:            dataChan,
		forwardDataFunc:     forwardDataFunc,
		hasBeenUnsubscribed: false,
	}
}

func (s *subscriptionMap[T]) Read(subscriptionID string) (sub subscription[T], success bool) {
	s.RLock()
	defer s.RUnlock()
	sub, success = s.map_[subscriptionID]
	return sub, success
}

func (s *subscriptionMap[_]) Unsubscribe(subscriptionID string) error {
	s.Lock()
	defer s.Unlock()
	unsub, success := s.map_[subscriptionID]
	if !success {
		return fmt.Errorf("tried to unsubscribe from unknown subscription with ID '%s'", subscriptionID)
	}
	unsub.hasBeenUnsubscribed = true
	s.map_[subscriptionID] = unsub
	close(s.map_[subscriptionID].dataChan)
	return nil
}

func (s *subscriptionMap[_]) GetAllIDs() (subscriptionIDs []string) {
	s.RLock()
	defer s.RUnlock()
	for subID := range s.map_ {
		subscriptionIDs = append(subscriptionIDs, subID)
	}
	return subscriptionIDs
}

func (s *subscriptionMap[_]) Delete(subscriptionID string) {
	s.Lock()
	defer s.Unlock()
	delete(s.map_, subscriptionID)
}
