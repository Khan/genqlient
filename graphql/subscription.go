package graphql

import (
	"reflect"
)

// subscriptionMap is a map of subscription ID to subscription.
// It is NOT thread-safe and must be protected by the caller's lock.
type subscriptionMap struct {
	map_ map[string]subscription
}

type subscription struct {
	interfaceChan   interface{}
	forwardDataFunc ForwardDataFunction
	id              string
	closed          bool // true if the channel has been closed
}

// create adds a new subscription to the map.
// The caller must hold the webSocketClient lock.
func (s *subscriptionMap) create(subscriptionID string, interfaceChan interface{}, forwardDataFunc ForwardDataFunction) {
	s.map_[subscriptionID] = subscription{
		id:              subscriptionID,
		interfaceChan:   interfaceChan,
		forwardDataFunc: forwardDataFunc,
		closed:          false,
	}
}

// get retrieves a subscription by ID.
// The caller must hold the webSocketClient lock.
// Returns nil if not found.
func (s *subscriptionMap) get(subscriptionID string) *subscription {
	sub, ok := s.map_[subscriptionID]
	if !ok {
		return nil
	}
	return &sub
}

// update updates a subscription in the map.
// The caller must hold the webSocketClient lock.
func (s *subscriptionMap) update(subscriptionID string, sub subscription) {
	s.map_[subscriptionID] = sub
}

// getAllIDs returns all subscription IDs.
// The caller must hold the webSocketClient lock.
func (s *subscriptionMap) getAllIDs() []string {
	subscriptionIDs := make([]string, 0, len(s.map_))
	for subID := range s.map_ {
		subscriptionIDs = append(subscriptionIDs, subID)
	}
	return subscriptionIDs
}

// delete removes a subscription from the map.
// The caller must hold the webSocketClient lock.
func (s *subscriptionMap) delete(subscriptionID string) {
	delete(s.map_, subscriptionID)
}

// closeChannel closes a subscription's channel if it hasn't been closed yet.
// The caller must hold the webSocketClient lock.
// Returns true if the channel was closed, false if it was already closed.
func (s *subscriptionMap) closeChannel(subscriptionID string) bool {
	sub := s.get(subscriptionID)
	if sub == nil || sub.closed {
		return false
	}

	// Mark as closed before actually closing to prevent double-close
	sub.closed = true
	s.update(subscriptionID, *sub)

	// Close the channel
	reflect.ValueOf(sub.interfaceChan).Close()
	return true
}
