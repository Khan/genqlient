package graphql

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

const (
	webSocketMethod         = "websocket"
	webSocketTypeConnInit   = "connection_init"
	webSocketTypeConnAck    = "connection_ack"
	webSocketTypeSubscribe  = "subscribe"
	webSocketTypeNext       = "next"
	webSocketTypeError      = "error"
	webSocketTypeComplete   = "complete"
	websocketConnAckTimeOut = time.Second * 30
)

// Close codes defined in RFC 6455, section 11.7.
const (
	closeNormalClosure    = 1000
	closeNoStatusReceived = 1005
)

// The message types are defined in RFC 6455, section 11.8.
const (
	// textMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	textMessage = 1

	// closeMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	closeMessage = 8
)

type webSocketSendMessage struct {
	Payload *Request `json:"payload"`
	Type    string   `json:"type"`
	ID      string   `json:"id"`
}

type webSocketReceiveMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

func (w *webSocketClient) sendInit() error {
	connInitMsg := webSocketSendMessage{
		Type: webSocketTypeConnInit,
	}
	return w.sendStructAsJSON(connInitMsg)
}

func (w *webSocketClient) sendStructAsJSON(object any) error {
	jsonBytes, err := json.Marshal(object)
	if err != nil {
		return err
	}
	return w.conn.WriteMessage(textMessage, jsonBytes)
}

func (w *webSocketClient) waitForConnAck() error {
	var connAckReceived bool
	var err error
	start := time.Now()
	for !connAckReceived {
		connAckReceived, err = w.receiveWebSocketConnAck()
		if err != nil {
			return err
		}
		if time.Since(start) > websocketConnAckTimeOut {
			return fmt.Errorf("timed out while waiting for connAck (> %v)", websocketConnAckTimeOut)
		}
	}
	return nil
}

func (w *webSocketClient) listenWebSocket() {
	for {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			w.errChan <- err
			return
		}
		err = w.forwardWebSocketData(message)
		if err != nil {
			w.errChan <- err
			return
		}
	}
}

func (w *webSocketClient) forwardWebSocketData(message []byte) error {
	var wsMsg webSocketReceiveMessage
	err := json.Unmarshal(message, &wsMsg)
	if err != nil {
		return err
	}
	sub, ok := w.subscriptions.Read(wsMsg.ID)
	if !ok {
		return fmt.Errorf("received message for unknown subscription ID '%s'", wsMsg.ID)
	}
	return sub.forwardDataFunc(sub.interfaceChan, wsMsg.Payload)
}

func (w *webSocketClient) receiveWebSocketConnAck() (bool, error) {
	_, message, err := w.conn.ReadMessage()
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

// formatCloseMessage formats closeCode and text as a WebSocket close message.
// An empty message is returned for code CloseNoStatusReceived.
func formatCloseMessage(closeCode int, text string) []byte {
	if closeCode == closeNoStatusReceived {
		// Return empty message because it's illegal to send
		// CloseNoStatusReceived. Return non-nil value in case application
		// checks for nil.
		return []byte{}
	}
	buf := make([]byte, 2+len(text))
	binary.BigEndian.PutUint16(buf, uint16(closeCode))
	copy(buf[2:], text)
	return buf
}
