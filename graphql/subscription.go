package graphql

import (
	"encoding/json"
	"sync"
)

// map of subscription ID to subscription
type subscriptionMap struct {
	map_ map[string]subscription
	sync.RWMutex
}

type subscription struct {
	interfaceChan   interface{}
	forwardDataFunc ForwardDataFunction
	id              string
}

type ForwardDataFunction func(interfaceChan interface{}, jsonRawMsg json.RawMessage) error

func (s *subscriptionMap) Create(subscriptionID string, interfaceChan interface{}, forwardDataFunc ForwardDataFunction) {
	s.Lock()
	defer s.Unlock()
	s.map_[subscriptionID] = subscription{
		id:              subscriptionID,
		interfaceChan:   interfaceChan,
		forwardDataFunc: forwardDataFunc,
	}
}

func (s *subscriptionMap) Read(subscriptionID string) (sub subscription, success bool) {
	s.RLock()
	defer s.RUnlock()
	sub, success = s.map_[subscriptionID]
	return sub, success
}

func (s *subscriptionMap) Delete(subscriptionID string) {
	s.Lock()
	defer s.Unlock()
	delete(s.map_, subscriptionID)
}
