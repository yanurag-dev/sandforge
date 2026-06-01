package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/coder/websocket"

	"github.com/yanurag-dev/sandforge/pkg/agentproto"
)

// PTYOptions configures an interactive PTY session.
type PTYOptions struct {
	Cols    uint16   // initial terminal width (default 80)
	Rows    uint16   // initial terminal height (default 24)
	Command []string // command to run (default: the guest's login shell)
}

// PTYSession is a client-side handle to an interactive terminal session. It
// mirrors the server-side session contract: NextEvent returns the {event:"exit"}
// event normally, then io.EOF once the session ends.
//
// Concurrency: call SendStdin/Resize from one goroutine and NextEvent from
// another (the WebSocket allows one concurrent reader + one writer). Do not call
// NextEvent concurrently with itself, nor SendStdin concurrently with itself.
type PTYSession struct {
	conn *websocket.Conn
	ctx  context.Context
}

// OpenPTY dials the control plane's PTY WebSocket for the given sandbox and
// starts an interactive session. The ctx governs the whole session lifetime —
// cancelling it tears the session down.
func (c *Client) OpenPTY(ctx context.Context, id string, opts PTYOptions) (*PTYSession, error) {
	wsURL, err := toWebSocketURL(c.baseURL)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	if opts.Cols > 0 {
		q.Set("cols", strconv.Itoa(int(opts.Cols)))
	}
	if opts.Rows > 0 {
		q.Set("rows", strconv.Itoa(int(opts.Rows)))
	}
	for _, c := range opts.Command {
		q.Add("cmd", c)
	}

	endpoint := wsURL + "/v1/sandboxes/" + id + "/pty"
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}

	conn, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("open pty: %w", err)
	}
	return &PTYSession{conn: conn, ctx: ctx}, nil
}

// SendStdin forwards input bytes to the remote terminal.
func (s *PTYSession) SendStdin(data []byte) error {
	return s.writeEvent(agentproto.StreamEvent{Event: "stdin", Data: data})
}

// Resize updates the remote terminal window size.
func (s *PTYSession) Resize(cols, rows uint16) error {
	return s.writeEvent(agentproto.StreamEvent{Event: "resize", Cols: cols, Rows: rows})
}

// NextEvent blocks until the next event arrives from the session (stdout/exit/
// error), returning io.EOF once the session has ended. It is a blocking pull,
// not a busy-poll: it parks on the socket read until data arrives.
func (s *PTYSession) NextEvent() (agentproto.StreamEvent, error) {
	_, data, err := s.conn.Read(s.ctx)
	if err != nil {
		return agentproto.StreamEvent{}, normalizeReadErr(err)
	}
	var ev agentproto.StreamEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return agentproto.StreamEvent{}, fmt.Errorf("decode event: %w", err)
	}
	return ev, nil
}

// Close ends the session and releases the connection.
func (s *PTYSession) Close() error {
	return s.conn.Close(websocket.StatusNormalClosure, "client closed")
}

func (s *PTYSession) writeEvent(ev agentproto.StreamEvent) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	return s.conn.Write(s.ctx, websocket.MessageText, data)
}

// normalizeReadErr translates a clean WebSocket close into io.EOF so callers can
// use the idiomatic errors.Is(err, io.EOF) end-of-stream check, matching the
// server-side PTYSession contract. Other errors pass through unchanged.
func normalizeReadErr(err error) error {
	var ce websocket.CloseError
	if errors.As(err, &ce) && ce.Code == websocket.StatusNormalClosure {
		return io.EOF
	}
	return err
}

// toWebSocketURL converts an http(s):// base URL to its ws(s):// equivalent.
func toWebSocketURL(base string) (string, error) {
	switch {
	case strings.HasPrefix(base, "https://"):
		return "wss://" + strings.TrimPrefix(base, "https://"), nil
	case strings.HasPrefix(base, "http://"):
		return "ws://" + strings.TrimPrefix(base, "http://"), nil
	case strings.HasPrefix(base, "ws://"), strings.HasPrefix(base, "wss://"):
		return base, nil
	default:
		return "", fmt.Errorf("unsupported base URL scheme: %q", base)
	}
}
