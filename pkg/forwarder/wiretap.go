package forwarder

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/datawire/dlib/dlog"
)

// AddWiretaps installs wiretaps on a connection. The wiretapped connection is returned along with the wiretaps.
// If count is zero, then this function returns the original connection and nil.
//
// A wiretap will receive all data read on the original connection and has the following characteristics:
//
//   - It is buffered. Each call to Read on the original connection creates one entry in the cache.
//   - Each Read on the wiretap consumes one entry in the cache.
//   - A wiretap will never cause the tapped connection to block. Data is discarded when the cache is full.
//   - Errors occurring when sending wiretapped data will cause the wiretap to close and be discarded.
//   - Errors occurring when reading on the original connection will be propagated and cause all wiretaps to be closed.
//
// The wiretap connection has the following characteristics:
//
//   - A Read will return the same as a Read on the original connection.
//   - A Write will silently discard the data to be written.
//   - SetTimeout, SetReadTimeout, and SetWriteTimeout are all no-ops.
//   - The LocalAddr and RemoteAddr calls are dispatched to the original connection.
//   - The connection is closed on error reading the original connection, when the context is done, or when the
//     original connection is explicitly closed.
func AddWiretaps(ctx context.Context, conn net.Conn, count, cacheSize int) (net.Conn, []net.Conn) {
	if count == 0 {
		return conn, nil
	}
	wrs := make([]*io.PipeWriter, count)
	taps := make([]net.Conn, count)
	chs := make([]chan []byte, count)

	for i := 0; i < count; i++ {
		rd, wr := io.Pipe()
		taps[i] = &readOverrideConn{Conn: conn, rd: rd}
		wrs[i] = wr
		chs[i] = make(chan []byte, cacheSize)
	}
	for i, c := range chs {
		go writePump(ctx, c, wrs[i])
	}
	return &teeConn{
		Conn: conn,
		cs:   chs,
		ws:   wrs,
	}, taps
}

// readConn wraps an io.ReadCloser in a net.Conn. Writes are discarded and addresses are
// retrieved from the wrapped Conn.
type readOverrideConn struct {
	net.Conn
	rd io.ReadCloser
}

// Read reads from the wrapped io.ReadCloser.
func (e *readOverrideConn) Read(b []byte) (n int, err error) {
	return e.rd.Read(b)
}

// Write discards the data and returns its length.
func (e *readOverrideConn) Write(b []byte) (n int, err error) {
	return len(b), nil
}

// Close closes the wrapped io.ReadCloser.
func (e *readOverrideConn) Close() error {
	return e.rd.Close()
}

// SetDeadline calls SetReadDeadline.
func (e *readOverrideConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline is currently a no-op.
func (e *readOverrideConn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline is a no-op.
func (e *readOverrideConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// teeConn wraps a net.Conn and copies everything read from it to its pipe-writers
// using channel buffers.
// The teeConn is designed to discard data when the buffers fill up rather than
// causing a block.
// Any error encountered during Read will be propagated to the pipe-writers in a
// CloseWithError call.
type teeConn struct {
	net.Conn
	mu sync.Mutex
	cs []chan []byte
	ws []*io.PipeWriter
}

func (mw *teeConn) Read(b []byte) (n int, err error) {
	n, err = mw.Conn.Read(b)
	if n > 0 {
		dc := make([]byte, n)
		copy(dc, b)
		mw.send(dc)
	}
	if err != nil {
		mw.mu.Lock()
		for i, w := range mw.ws {
			_ = w.CloseWithError(err)
			close(mw.cs[i])
		}
		mw.ws = nil
		mw.cs = nil
		mw.mu.Unlock()
	}
	return n, err
}

func (mw *teeConn) Close() error {
	mw.mu.Lock()
	for i, w := range mw.ws {
		_ = w.Close()
		close(mw.cs[i])
	}
	mw.ws = nil
	mw.cs = nil
	mw.mu.Unlock()
	return mw.Conn.Close()
}

func (mw *teeConn) send(data []byte) {
	mw.mu.Lock()
	for _, c := range mw.cs {
		select {
		case c <- data:
		default:
			// We end up discarding data here if the consumer is too slow.
		}
	}
	mw.mu.Unlock()
}

func writePump(ctx context.Context, ch <-chan []byte, w *io.PipeWriter) {
	for {
		select {
		case <-ctx.Done():
			w.Close()
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			n, err := w.Write(data)
			if err == nil && n != len(data) {
				err = io.ErrShortWrite
			}
			if err != nil {
				dlog.Errorf(ctx, "failed to write wiretap data: %s", err)
				return
			}
		}
	}
}
