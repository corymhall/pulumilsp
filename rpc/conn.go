package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Conn is the common interface to jsonrpc servers.
// Conn is bidirectional; it does not have a designated server or client end.
// It manages the jsonrpc2 protocol, connecting responses back to their calls.
type Conn interface {
	// Call invokes the target method and waits for a response.
	// The params will be marshaled to JSON before sending over the wire, and will
	// be handed to the method invoked.
	// The response will be unmarshaled from JSON into the result.
	// The id returned will be unique from this connection, and can be used for
	// logging or tracking.
	Call(ctx context.Context, method string, params, result any) (ID, error)

	// Notify invokes the target method but does not wait for a response.
	// The params will be marshaled to JSON before sending over the wire, and will
	// be handed to the method invoked.
	Notify(ctx context.Context, method string, params any) error

	Run(ctx context.Context, handler Handler)

	Done() <-chan struct{}

	Logger() *log.Logger
}

type conn struct {
	seq       int64 // must only be accessed using atomic operations
	stream    Stream
	pendingMu sync.Mutex // protects the pending map
	pending   map[ID]chan *Response
	logger    *log.Logger
	done      chan struct{}
}

func (c *conn) Logger() *log.Logger {
	return c.logger
}

// NewConn creates a new connection object around the supplied stream.
func NewConn(s Stream, logger *log.Logger) Conn {
	conn := &conn{
		logger:  logger,
		stream:  s,
		pending: make(map[ID]chan *Response),
		done:    make(chan struct{}),
	}
	return conn
}

func (c *conn) Notify(ctx context.Context, method string, params any) (err error) {
	notify, err := NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("marshaling notify parameters: %v", err)
	}
	_, err = c.write(ctx, notify)
	return err
}

func (c *conn) Call(ctx context.Context, method string, params, result any) (_ ID, err error) {
	// generate a new request identifier
	id := ID{number: atomic.AddInt64(&c.seq, 1)}
	call, err := NewCall(id, method, params)
	if err != nil {
		c.logger.Printf("error creating call: %v", err)
		return id, fmt.Errorf("marshaling call parameters: %v", err)
	}
	// We have to add ourselves to the pending map before we send, otherwise we
	// are racing the response. Also add a buffer to rchan, so that if we get a
	// wire response between the time this call is cancelled and id is deleted
	// from c.pending, the send to rchan will not block.
	rchan := make(chan *Response, 1)
	c.pendingMu.Lock()
	c.pending[id] = rchan
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()
	// now we are ready to send
	_, err = c.write(ctx, call)
	if err != nil {
		c.logger.Printf("error writing call: %v", err)
		// sending failed, we will never get a response, so don't leave it pending
		return id, err
	}
	// now wait for the response
	select {
	case response := <-rchan:
		c.logger.Printf("response received for method: %s", method)
		// is it an error response?
		if response.err != nil {
			c.logger.Printf("error in response: %v", response.err)
			return id, response.err
		}
		if result == nil || len(response.result) == 0 {
			return id, nil
		}
		if err := json.Unmarshal(response.result, result); err != nil {
			c.logger.Printf("error unmarshaling result: %v", err)
			return id, fmt.Errorf("unmarshaling result: %v", err)
		}
		return id, nil
	case <-ctx.Done():
		return id, ctx.Err()
	}
}
func (c *conn) replier(req Request) Replier {
	return func(ctx context.Context, result any, err error) error {
		call, ok := req.(*Call)
		if !ok {
			// request was a notify, no need to respond
			return nil
		}
		c.logger.Printf("replier called for method: %s", call.method)
		response, err := NewResponse(call.id, result, err)
		if err != nil {
			c.logger.Printf("error creating response: %v", err)
			return err
		}
		_, err = c.write(ctx, response)
		if err != nil {
			c.logger.Printf("error writing response: %v", err)
			return err
		}
		return nil
	}
}

func (c *conn) write(ctx context.Context, msg Message) (int64, error) {
	return c.stream.Write(ctx, msg)
}

func (c *conn) Run(ctx context.Context, handler Handler) {
	defer close(c.done)
	for {
		// get the next message
		msg, n, err := c.stream.Read(ctx)
		c.logger.Printf("read %d bytes from stream", n)
		if err != nil {
			c.logger.Printf("error reading from stream: %v", err)
			// The stream failed, we cannot continue.
			contract.AssertNoErrorf(err, "error reading from stream: %v", err)
			return
		}
		switch msg := msg.(type) {
		case Request:
			c.logger.Printf("request received for method: %s", msg.Method())
			if err := handler(ctx, c.replier(msg), msg); err != nil {
				// delivery failed, not much we can do
				c.logger.Printf("error handling request: %v", err)
			}
		case *Response:
			c.logger.Printf("response received for id: %v", msg.id)
			// If method is not set, this should be a response, in which case we must
			// have an id to send the response back to the caller.
			c.pendingMu.Lock()
			rchan, ok := c.pending[msg.id]
			c.pendingMu.Unlock()
			if ok {
				rchan <- msg
			}
		}
		c.logger.Printf("Message not handled: %v", msg)
	}
}

func (c *conn) Done() <-chan struct{} {
	return c.done
}
