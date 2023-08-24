package graphql

import (
	"encoding/json"

	"github.com/gorilla/websocket"
)

const (
	webSocketTypeConnInit  = "connection_init"
	webSocketTypeConnAck   = "connection_ack"
	webSocketTypeSubscribe = "subscribe"
	webSocketTypeNext      = "next"
	webSocketTypeError     = "error"
	webSocketTypeComplete  = "complete"
)

type webSocketSendMessage struct {
	Payload *Request `json:"payload"`
	Type    string   `json:"type"`
}

type webSocketReceiveMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func sendInit(conn *websocket.Conn) error {
	connInit := webSocketSendMessage{
		Type: webSocketTypeConnInit,
	}
	return sendStructAsJSON(conn, connInit)
}

func sendSubscribe(conn *websocket.Conn, req *Request) error {
	subscription := webSocketSendMessage{
		Type:    webSocketTypeSubscribe,
		Payload: req,
	}
	return sendStructAsJSON(conn, subscription)
}

func sendComplete(conn *websocket.Conn) error {
	complete := webSocketSendMessage{
		Type: webSocketTypeComplete,
	}
	return sendStructAsJSON(conn, complete)
}

func sendStructAsJSON(conn *websocket.Conn, object any) error {
	jsonBytes, err := json.Marshal(object)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, jsonBytes)
}

func listenWebSocket(conn *websocket.Conn, resp *Response, dataUpdated chan bool, errChan chan error, done chan struct{}) {
	var connAckReceived bool
	var err error
	defer endListenWebSocket(dataUpdated, errChan, done, err)
	for !connAckReceived {
		connAckReceived, err = receiveWebSocketConnAck(conn)
		if err != nil {
			errChan <- err
			return
		}
	}
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			errChan <- err
			return
		}
		err = forwardWebSocketData(resp, dataUpdated, message)
		if err != nil {
			errChan <- err
			return
		}
	}
}

func endListenWebSocket(dataUpdated chan bool, errChan chan error, done chan struct{}, err error) {
	errChan <- err
	close(dataUpdated)
	close(errChan)
	close(done)
}

func receiveWebSocketConnAck(conn *websocket.Conn) (bool, error) {
	_, message, err := conn.ReadMessage()
	if err != nil {
		return false, err
	}
	return checkConnectionAckReceived(message)
}

func checkConnectionAckReceived(message []byte) (bool, error) {
	wsMessage := &webSocketSendMessage{}
	err := json.Unmarshal(message, wsMessage)
	if err != nil {
		return false, err
	}
	return wsMessage.Type == webSocketTypeConnAck, nil
}

func forwardWebSocketData(resp *Response, dataUpdated chan bool, message []byte) error {
	var wsMsg webSocketReceiveMessage
	err := json.Unmarshal(message, &wsMsg)
	if err != nil {
		return err
	}
	switch wsMsg.Type {
	case webSocketTypeNext, webSocketTypeError:
		err = json.Unmarshal(wsMsg.Payload, resp)
		if err != nil {
			return err
		}
		dataUpdated <- true
	default:
	}
	return nil
}

func waitToEndWebSocket(conn *websocket.Conn, errChan chan error, done chan struct{}) {
	<-done
	err := sendComplete(conn)
	if err != nil {
		errChan <- err
	}
	err = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		errChan <- err
		return
	}
}
