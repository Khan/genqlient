package graphql

import (
	"fmt"
	"sync"
)

// map of subscription ID to subscription
type subscriptionMap struct {
	map_ map[string]*subscription
	sync.RWMutex
}

type subscription struct {
	// interfaceChan is passed in by the user when creating a subscription but
	// closed by webSocketClient when the subscription is unsubscribed, i.e.
	// ownership of interfaceChan is passed from the user to the client.
	//
	// The subscription is unsubscribed either explicitly by the user or when
	// a message of webSocketTypeComplete is received. On unsubscribe,
	// the _hasBeenUnsubscribed flag is set to true. listenWebSocket then
	// closes interfaceChan on the next receive loop.
	//
	// The listenWebSocket client method handles both sending on the channel
	// and closing of the channel, so is no possibility of races between send
	// and close.
	interfaceChan interface{}

	forwardDataFunc ForwardDataFunction
	id              string

	// Hold when accessing _hasBeenUnsubscribed
	hasBeenUnsubscribedMu sync.Mutex
	_hasBeenUnsubscribed  bool
}

func (s *subscription) unsubscribe() {
	s.hasBeenUnsubscribedMu.Lock()
	defer s.hasBeenUnsubscribedMu.Unlock()

	s._hasBeenUnsubscribed = true
}

func (s *subscription) hasBeenUnsubscribed() bool {
	s.hasBeenUnsubscribedMu.Lock()
	defer s.hasBeenUnsubscribedMu.Unlock()

	return s._hasBeenUnsubscribed
}

func (s *subscriptionMap) Create(subscriptionID string, interfaceChan interface{}, forwardDataFunc ForwardDataFunction) {
	s.Lock()
	defer s.Unlock()
	s.map_[subscriptionID] = &subscription{
		id:                   subscriptionID,
		interfaceChan:        interfaceChan,
		forwardDataFunc:      forwardDataFunc,
		_hasBeenUnsubscribed: false,
	}
}

func (s *subscriptionMap) Unsubscribe(subscriptionID string) error {
	s.Lock()
	defer s.Unlock()
	unsub, success := s.map_[subscriptionID]
	if !success {
		return fmt.Errorf("tried to unsubscribe from unknown subscription with ID '%s'", subscriptionID)
	}
	unsub.unsubscribe()
	s.map_[subscriptionID] = unsub

	return nil
}

func (s *subscriptionMap) forEachSubscription(fn func(sub *subscription)) {
	s.Lock()
	defer s.Unlock()

	for id := range s.map_ {
		fn(s.map_[id])
	}
}

func (s *subscriptionMap) GetSubscription(subscriptionID string) (*subscription, bool) {
	s.Lock()
	defer s.Unlock()
	sub, ok := s.map_[subscriptionID]
	return sub, ok
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
