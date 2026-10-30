package mikorm

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultPort is the default RouterOS API port.
	DefaultPort = "8728"
	// DefaultTLSPort is the default RouterOS API-SSL port.
	DefaultTLSPort = "8729"
	// DefaultTimeout is used when Config.Timeout is zero.
	DefaultTimeout = 10 * time.Second
)

// Config holds the parameters used to connect to a RouterOS device.
type Config struct {
	// Address is the device address as "host" or "host:port". When the
	// port is omitted DefaultPort (or DefaultTLSPort when TLS is set) is used.
	Address string
	// Username used to log in. Required.
	Username string
	// Password used to log in.
	Password string
	// TLS enables the API-SSL service when non-nil.
	TLS *tls.Config
	// Timeout bounds dialing and logging in. Defaults to DefaultTimeout.
	// Use a context to bound individual commands.
	Timeout time.Duration
	// Logger receives debug logs of every command sent to the device.
	// Values of properties whose name contains "password" or "secret" are
	// redacted. Nil disables logging.
	Logger *slog.Logger
}

func (cfg Config) timeout() time.Duration {
	if cfg.Timeout > 0 {
		return cfg.Timeout
	}
	return DefaultTimeout
}

// Client is a connection to a RouterOS device. It is safe for concurrent
// use by multiple goroutines: every command is tagged, so many commands can
// be in flight on the same connection.
type Client struct {
	conn   io.ReadWriteCloser
	logger *slog.Logger

	wmu sync.Mutex // serializes writes
	bw  *bufio.Writer

	mu      sync.Mutex
	pending map[string]*request
	nextTag uint64
	err     error // set once the connection is unusable
}

// Reply is the result of a command.
type Reply struct {
	// Records holds the data (!re) sentences returned by the device.
	Records []map[string]string
	// Done holds the attributes of the final !done sentence, e.g. "ret".
	Done map[string]string
}

// Scan decodes the reply records into dest. See Query.Find for the
// accepted destination types.
func (r *Reply) Scan(dest any) error {
	return decodeRecords(r.Records, dest)
}

// request tracks one in-flight command.
type request struct {
	command string
	notify  chan struct{} // signaled when records arrive or the command ends

	mu       sync.Mutex
	records  []map[string]string
	done     map[string]string
	err      error
	finished bool
}

func newRequest(command string) *request {
	return &request{command: command, notify: make(chan struct{}, 1)}
}

func (r *request) signal() {
	select {
	case r.notify <- struct{}{}:
	default:
	}
}

func (r *request) finish(done map[string]string, err error) {
	r.mu.Lock()
	if !r.finished {
		r.finished, r.done, r.err = true, done, err
	}
	r.mu.Unlock()
	r.signal()
}

// take returns the records received so far and, once the command ended,
// its final attributes or error.
func (r *request) take() (records []map[string]string, done map[string]string, finished bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records, r.records = r.records, nil
	return records, r.done, r.finished, r.err
}

// Dial connects and logs in to the RouterOS device described by cfg.
func Dial(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Address == "" {
		return nil, errors.New("mikorm: address is required")
	}
	if cfg.Username == "" {
		return nil, errors.New("mikorm: username is required")
	}

	addr := cfg.Address
	if _, _, err := net.SplitHostPort(addr); err != nil {
		port := DefaultPort
		if cfg.TLS != nil {
			port = DefaultTLSPort
		}
		addr = net.JoinHostPort(strings.Trim(addr, "[]"), port)
	}

	dialer := &net.Dialer{Timeout: cfg.timeout()}
	var (
		conn net.Conn
		err  error
	)
	if cfg.TLS != nil {
		td := &tls.Dialer{NetDialer: dialer, Config: cfg.TLS}
		conn, err = td.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("mikorm: dial %s: %w", addr, err)
	}
	return New(ctx, conn, cfg)
}

// New logs in over an already established connection and returns a Client
// that owns conn. Only the Username, Password, Timeout and Logger fields of
// cfg are used. It is useful for custom transports; most callers want Dial.
func New(ctx context.Context, conn io.ReadWriteCloser, cfg Config) (*Client, error) {
	c := &Client{
		conn:    conn,
		logger:  cfg.Logger,
		bw:      bufio.NewWriter(conn),
		pending: make(map[string]*request),
	}
	go c.readLoop(bufio.NewReader(conn))

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout())
	defer cancel()
	if err := c.login(ctx, cfg.Username, cfg.Password); err != nil {
		c.Close()
		var de *DeviceError
		if errors.As(err, &de) {
			return nil, err
		}
		return nil, fmt.Errorf("mikorm: login: %w", err)
	}
	return c, nil
}

func (c *Client) login(ctx context.Context, username, password string) error {
	reply, err := c.Run(ctx, "/login", "=name="+username, "=password="+password)
	if err != nil {
		return err
	}
	challenge, ok := reply.Done["ret"]
	if !ok {
		return nil // RouterOS 6.43+: plain login succeeded
	}

	// Pre-6.43 challenge/response login.
	b, err := hex.DecodeString(challenge)
	if err != nil {
		return fmt.Errorf("invalid challenge %q", challenge)
	}
	h := md5.New()
	h.Write([]byte{0})
	io.WriteString(h, password)
	h.Write(b)
	_, err = c.Run(ctx, "/login", "=name="+username, "=response=00"+hex.EncodeToString(h.Sum(nil)))
	return err
}

// Close closes the connection. Pending and later commands return ErrClosed.
func (c *Client) Close() error {
	c.shutdown(ErrClosed)
	return nil
}

// shutdown marks the client unusable with err, closes the connection and
// fails every pending command.
func (c *Client) shutdown(err error) {
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	c.err = err
	pending := c.pending
	c.pending = map[string]*request{}
	c.mu.Unlock()

	c.conn.Close()
	for _, r := range pending {
		r.finish(nil, err)
	}
}

func (c *Client) readLoop(r *bufio.Reader) {
	for {
		s, err := readSentence(r)
		if err != nil {
			c.shutdown(fmt.Errorf("mikorm: connection lost: %w", err))
			return
		}

		if s.word == "!fatal" {
			// The device closes the connection after !fatal.
			c.shutdown(&DeviceError{Command: "connection", Message: s.attrs["message"], Fatal: true})
			return
		}

		c.mu.Lock()
		req := c.pending[s.tag]
		if req != nil && s.word != "!re" {
			delete(c.pending, s.tag)
		}
		c.mu.Unlock()
		if req == nil {
			// Unknown tag, e.g. the reply of a canceled command or the
			// !done following a !trap that was already delivered.
			continue
		}

		switch s.word {
		case "!re":
			req.mu.Lock()
			req.records = append(req.records, s.attrs)
			req.mu.Unlock()
			req.signal()
		case "!done", "!empty":
			req.finish(s.attrs, nil)
		case "!trap":
			req.finish(nil, &DeviceError{
				Command:  req.command,
				Message:  s.attrs["message"],
				Category: s.attrs["category"],
			})
		default:
			req.finish(nil, fmt.Errorf("mikorm: %s: unexpected reply %q", req.command, s.word))
		}
	}
}

// start registers and sends a command.
func (c *Client) start(words []string) (tag string, req *request, err error) {
	req = newRequest(words[0])

	c.mu.Lock()
	if c.err != nil {
		err = c.err
		c.mu.Unlock()
		return "", nil, err
	}
	c.nextTag++
	tag = strconv.FormatUint(c.nextTag, 10)
	// Register before writing so the reply cannot arrive first.
	c.pending[tag] = req
	c.mu.Unlock()

	c.wmu.Lock()
	err = writeSentence(c.bw, words, tag)
	c.wmu.Unlock()
	if err != nil {
		c.shutdown(fmt.Errorf("mikorm: connection lost: %w", err))
		return "", nil, c.connErr()
	}
	return tag, req, nil
}

// cancel abandons a command: its reply is ignored and the device is asked
// to stop it.
func (c *Client) cancel(tag string) {
	c.mu.Lock()
	_, ok := c.pending[tag]
	delete(c.pending, tag)
	usable := c.err == nil
	c.mu.Unlock()
	if !ok || !usable {
		return
	}
	// Fire and forget: the reply to /cancel has no registered tag.
	go func() {
		c.wmu.Lock()
		defer c.wmu.Unlock()
		_ = writeSentence(c.bw, []string{"/cancel", "=tag=" + tag}, "c"+tag)
	}()
}

func (c *Client) connErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Run sends a raw command to the device and waits for its reply. command
// is the API path, e.g. "/system/resource/print", and args are raw API
// words such as "=name=value" or "?name=value".
//
// If ctx is done before the device replies, the command is canceled and
// Run returns ctx.Err().
func (c *Client) Run(ctx context.Context, command string, args ...string) (*Reply, error) {
	words := make([]string, 0, len(args)+1)
	words = append(words, command)
	words = append(words, args...)
	return c.run(ctx, words)
}

func (c *Client) run(ctx context.Context, words []string) (reply *Reply, err error) {
	start := time.Now()
	defer func() { c.log(ctx, words, time.Since(start), err) }()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tag, req, err := c.start(words)
	if err != nil {
		return nil, err
	}
	var records []map[string]string
	for {
		select {
		case <-ctx.Done():
			c.cancel(tag)
			return nil, ctx.Err()
		case <-req.notify:
		}
		recs, done, finished, err := req.take()
		records = append(records, recs...)
		if !finished {
			continue
		}
		if err != nil {
			return nil, err
		}
		if records == nil {
			records = []map[string]string{}
		}
		if done == nil {
			done = map[string]string{}
		}
		return &Reply{Records: records, Done: done}, nil
	}
}

// Listen runs a streaming command, such as "/interface/listen" or
// "/tool/torch", and calls fn for every record as it arrives. It returns
// when the command finishes, ctx is done (returning ctx.Err()), or fn
// returns an error (returning that error). In the last two cases the
// command is canceled on the device.
//
// fn is called from the goroutine that called Listen.
func (c *Client) Listen(ctx context.Context, fn func(record map[string]string) error, command string, args ...string) (err error) {
	words := make([]string, 0, len(args)+1)
	words = append(words, command)
	words = append(words, args...)

	start := time.Now()
	defer func() { c.log(ctx, words, time.Since(start), err) }()

	if err := ctx.Err(); err != nil {
		return err
	}
	tag, req, err := c.start(words)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			c.cancel(tag)
			return ctx.Err()
		case <-req.notify:
		}
		records, _, finished, err := req.take()
		for _, rec := range records {
			if ferr := fn(rec); ferr != nil {
				c.cancel(tag)
				return ferr
			}
		}
		if finished {
			return err
		}
	}
}

func (c *Client) log(ctx context.Context, words []string, d time.Duration, err error) {
	if c.logger == nil {
		return
	}
	attrs := []any{
		slog.String("command", words[0]),
		slog.Any("args", redact(words[1:])),
		slog.Duration("duration", d),
	}
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	}
	c.logger.DebugContext(ctx, "mikorm: run", attrs...)
}

// redact hides values of sensitive properties in API words.
func redact(args []string) []string {
	out := make([]string, len(args))
	for i, w := range args {
		out[i] = w
		if len(w) < 2 {
			continue
		}
		key, _, ok := strings.Cut(w[1:], "=")
		if !ok {
			continue
		}
		lk := strings.ToLower(key)
		if strings.Contains(lk, "password") || strings.Contains(lk, "secret") || lk == "response" {
			out[i] = w[:1] + key + "=***"
		}
	}
	return out
}
