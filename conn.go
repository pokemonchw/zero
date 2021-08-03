package zero

import (
	"context"
	"net"
	"time"
)

// Conn wrap net.Conn
type Conn struct {
	sid        string
	rawConn    net.Conn
	sendCh     chan []byte
	done       chan error
	hbTimer    *time.Timer
	name       string
	messageCh  chan []byte
	hbInterval time.Duration
	hbTimeout  time.Duration
}

// GetName Get conn name
func (c *Conn) GetName() string {
	return c.name
}

// NewConn create new conn
func NewConn(c net.Conn, hbInterval time.Duration, hbTimeout time.Duration) *Conn {
	conn := &Conn{
		rawConn:    c,
		sendCh:     make(chan []byte, 100),
		done:       make(chan error),
		messageCh:  make(chan []byte, 100),
		hbInterval: hbInterval,
		hbTimeout:  hbTimeout,
	}

	conn.name = c.RemoteAddr().String()
	conn.hbTimer = time.NewTimer(conn.hbInterval)

	if conn.hbInterval == 0 {
		conn.hbTimer.Stop()
	}

	return conn
}

// Close close connection
func (c *Conn) Close() {
	c.hbTimer.Stop()
	c.rawConn.Close()
}

// SendMessage send message
func (c *Conn) SendMessage(msg []byte) error {

	c.sendCh <- msg
	return nil
}

// writeCoroutine write coroutine
func (c *Conn) writeCoroutine(ctx context.Context) {
	hbData := make([]byte, 0)

	for {
		select {
		case <-ctx.Done():
			return

		case pkt := <-c.sendCh:

			if pkt == nil {
				continue
			}

			if _, err := c.rawConn.Write(pkt); err != nil {
				c.done <- err
			}

		case <-c.hbTimer.C:
			c.SendMessage(hbData)
			// 设置心跳timer
			if c.hbInterval > 0 {
				c.hbTimer.Reset(c.hbInterval)
			}
		}
	}
}

// readCoroutine read coroutine
func (c *Conn) readCoroutine(ctx context.Context) {

	for {
		select {
		case <-ctx.Done():
			return

		default:
			// 设置超时
			if c.hbInterval > 0 {
				err := c.rawConn.SetReadDeadline(time.Now().Add(c.hbTimeout))
				if err != nil {
					c.done <- err
					continue
				}
			}
			buf := make([]byte, 2048)
			c.rawConn.Read(buf)
			c.messageCh <- buf
		}
	}
}
