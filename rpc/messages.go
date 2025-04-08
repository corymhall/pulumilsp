package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Message interface {
	isRPCMessage()
}

// Request is the shared interface to rpc messages that request
// a method be invoked.
// The request types are a closed set of *Call and *Notification.
type Request interface {
	Message
	// Method is a string containing the method name to invoke.
	Method() string
	// Params is a JSON value (object, array, null, or "") with the parameters of the method.
	Params() json.RawMessage
	// isJSONRPC2Request is used to make the set of request implementations closed.
	isRPCRequest()
}

// Notification is a request for which a response cannot occur, and as such
// it has not ID.
type Notification struct {
	// Method is a string containing the method name to invoke.
	method string
	params json.RawMessage
}

// Response is a reply to a Call.
// It will have the same ID as the call it is a response to.
type Response struct {
	// result is the content of the response.
	result json.RawMessage
	// err is set only if the call failed.
	err error
	// ID of the request this is a response to.
	id ID
}

// Call is a request that expects a response.
// The response will have a matching ID.
type Call struct {
	// Method is a string containing the method name to invoke.
	method string
	// Params is a JSON value (object, array, null, or "") with the parameters of the method.
	params json.RawMessage
	// id of this request, used to tie the Response back to the request.
	id ID
}

// NewNotification constructs a new Notification message for the supplied
// method and parameters.
func NewNotification(method string, params any) (*Notification, error) {
	p, merr := marshalToRaw(params)
	return &Notification{method: method, params: p}, merr
}

func (msg *Notification) Method() string          { return msg.method }
func (msg *Notification) Params() json.RawMessage { return msg.params }
func (msg *Notification) isRPCMessage()           {}
func (msg *Notification) isRPCRequest()           {}

func (n *Notification) MarshalJSON() ([]byte, error) {
	msg := wireRequest{Method: n.method, Params: &n.params}
	data, err := json.Marshal(msg)
	if err != nil {
		return data, fmt.Errorf("marshaling notification: %w", err)
	}
	return data, nil
}

func (n *Notification) UnmarshalJSON(data []byte) error {
	msg := wireRequest{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("unmarshaling notification: %w", err)
	}
	n.method = msg.Method
	if msg.Params != nil {
		n.params = *msg.Params
	}
	return nil
}

// NewCall constructs a new Call message for the supplied ID, method and
// parameters.
func NewCall(id ID, method string, params any) (*Call, error) {
	p, merr := marshalToRaw(params)
	return &Call{id: id, method: method, params: p}, merr
}

func (msg *Call) Method() string          { return msg.method }
func (msg *Call) Params() json.RawMessage { return msg.params }
func (msg *Call) ID() ID                  { return msg.id }
func (msg *Call) isRPCMessage()           {}
func (msg *Call) isRPCRequest()           {}

func (c *Call) MarshalJSON() ([]byte, error) {
	msg := wireRequest{Method: c.method, Params: &c.params, ID: &c.id}
	data, err := json.Marshal(msg)
	if err != nil {
		return data, fmt.Errorf("marshaling call: %w", err)
	}
	return data, nil
}

func (c *Call) UnmarshalJSON(data []byte) error {
	msg := wireRequest{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return fmt.Errorf("unmarshaling call: %w", err)
	}
	c.method = msg.Method
	if msg.Params != nil {
		c.params = *msg.Params
	}
	if msg.ID != nil {
		c.id = *msg.ID
	}
	return nil
}

// NewResponse constructs a new Response message that is a reply to the
// supplied. If err is set result may be ignored.
func NewResponse(id ID, result any, err error) (*Response, error) {
	r, merr := marshalToRaw(result)
	return &Response{id: id, result: r, err: err}, merr
}

func (msg *Response) ID() ID                  { return msg.id }
func (msg *Response) Result() json.RawMessage { return msg.result }
func (msg *Response) Err() error              { return msg.err }
func (msg *Response) isRPCMessage()           {}

func (r *Response) MarshalJSON() ([]byte, error) {
	msg := &wireResponse{Error: r.err, ID: &r.id}
	if msg.Error == nil {
		msg.Result = &r.result
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return data, fmt.Errorf("marshaling notification: %w", err)
	}
	return data, nil
}

func marshalToRaw(obj any) (json.RawMessage, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return json.RawMessage{}, err
	}
	return json.RawMessage(data), nil
}

func DecodeMessage(data []byte) (Message, error) {
	msg := wireCombined{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("unmarshaling jsonrpc message: %w", err)
	}
	if msg.Method == "" {
		// no method, should be a response
		if msg.ID == nil {
			return nil, errors.New(ErrInvalidRequest)
		}
		response := &Response{id: *msg.ID}
		if msg.Error != nil {
			response.err = msg.Error
		}
		if msg.Result != nil {
			response.result = *msg.Result
		}
		return response, nil
	}
	// has a method, must be a request
	if msg.ID == nil {
		// request with no ID is a notify
		notify := &Notification{method: msg.Method}
		if msg.Params != nil {
			notify.params = *msg.Params
		}
		return notify, nil
	}
	// request with an ID, must be a call
	call := &Call{method: msg.Method, id: *msg.ID}
	if msg.Params != nil {
		call.params = *msg.Params
	}
	return call, nil
}
