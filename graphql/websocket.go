package graphql

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

const (
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
	CloseNormalClosure           = 1000
	CloseGoingAway               = 1001
	CloseProtocolError           = 1002
	CloseUnsupportedData         = 1003
	CloseNoStatusReceived        = 1005
	CloseAbnormalClosure         = 1006
	CloseInvalidFramePayloadData = 1007
	ClosePolicyViolation         = 1008
	CloseMessageTooBig           = 1009
	CloseMandatoryExtension      = 1010
	CloseInternalServerErr       = 1011
	CloseServiceRestart          = 1012
	CloseTryAgainLater           = 1013
	CloseTLSHandshake            = 1015
)

// The message types are defined in RFC 6455, section 11.8.
const (
	// TextMessage denotes a text data message. The text message payload is
	// interpreted as UTF-8 encoded text data.
	TextMessage = 1

	// BinaryMessage denotes a binary data message.
	BinaryMessage = 2

	// CloseMessage denotes a close control message. The optional message
	// payload contains a numeric code and text. Use the FormatCloseMessage
	// function to format a close message payload.
	CloseMessage = 8

	// PingMessage denotes a ping control message. The optional message payload
	// is UTF-8 encoded text.
	PingMessage = 9

	// PongMessage denotes a pong control message. The optional message payload
	// is UTF-8 encoded text.
	PongMessage = 10
)

type webSocketSendMessage struct {
	Payload *Request `json:"payload"`
	Type    string   `json:"type"`
}

type webSocketReceiveMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func sendInit(conn WSConn) error {
	connInit := webSocketSendMessage{
		Type: webSocketTypeConnInit,
	}
	return sendStructAsJSON(conn, connInit)
}

func sendSubscribe(conn WSConn, req *Request) error {
	subscription := webSocketSendMessage{
		Type:    webSocketTypeSubscribe,
		Payload: req,
	}
	return sendStructAsJSON(conn, subscription)
}

func sendComplete(conn WSConn) error {
	complete := webSocketSendMessage{
		Type: webSocketTypeComplete,
	}
	return sendStructAsJSON(conn, complete)
}

func sendStructAsJSON(conn WSConn, object any) error {
	jsonBytes, err := json.Marshal(object)
	if err != nil {
		return err
	}
	return conn.WriteMessage(TextMessage, jsonBytes)
}

func waitForConnAck(conn WSConn) error {
	var connAckReceived bool
	var err error
	start := time.Now()
	for !connAckReceived {
		connAckReceived, err = receiveWebSocketConnAck(conn)
		if err != nil {
			return err
		}
		if time.Since(start) > websocketConnAckTimeOut {
			return fmt.Errorf("timed out while waiting for connAck (> %v)", websocketConnAckTimeOut)
		}
	}
	return nil
}

func listenWebSocket(conn WSConn, resp *Response, dataUpdated chan bool, errChan chan error, doneChan chan bool) {
	defer endListenWebSocket(dataUpdated, errChan, doneChan)
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

func endListenWebSocket(dataUpdated chan bool, errChan chan error, doneChan chan bool) {
	close(dataUpdated)
	close(errChan)
	close(doneChan)
}

func receiveWebSocketConnAck(conn WSConn) (bool, error) {
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

func waitToEndWebSocket(conn WSConn, errChan chan error, doneChan chan bool) {
	defer conn.Close()
	<-doneChan
	err := sendComplete(conn)
	if err != nil {
		errChan <- err
	}
	err = conn.WriteMessage(CloseMessage, formatCloseMessage(CloseNormalClosure, ""))
	if err != nil {
		errChan <- err
		return
	}
}

// formatCloseMessage formats closeCode and text as a WebSocket close message.
// An empty message is returned for code CloseNoStatusReceived.
func formatCloseMessage(closeCode int, text string) []byte {
	if closeCode == CloseNoStatusReceived {
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
