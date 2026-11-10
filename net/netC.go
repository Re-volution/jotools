package net

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/Re-volution/jotools/filelog"
	"github.com/gorilla/websocket"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type NetC struct {
	Id         int64
	off        atomic.Bool
	quiet      atomic.Bool
	lastActive atomic.Int64
	closeOnce  sync.Once
	CloseC     chan bool   //关闭后广播，发送和接收一起退出
	SendC      chan []byte //发送通道
	conn       net.Conn
	connws     *websocket.Conn
	parseH     func(interface{}, uint16) []byte
	readData   func() ([]byte, uint16, error)
	sendData   func([]byte) error
	close      func()
	onClose    func()
}

type nets struct {
	mu      sync.Mutex
	wsNet   []*NetManger
	listens []*NetManger
	conns   []*ConnManger
}

func (nc *NetC) shutdown() {
	if nc == nil {
		return
	}
	nc.closeOnce.Do(func() {
		nc.off.Store(true)
		if nc.close != nil {
			nc.close()
		}
		if nc.CloseC != nil {
			close(nc.CloseC)
		}
		if !nc.quiet.Load() && nc.onClose != nil {
			nc.onClose()
		}
	})
}

func (nc *NetC) shutdownQuiet() {
	if nc == nil {
		return
	}
	nc.quiet.Store(true)
	nc.shutdown()
}

func (nc *NetC) Close() {
	nc.shutdown()
}

func (nc *NetC) touch() {
	if nc == nil {
		return
	}
	nc.lastActive.Store(time.Now().Unix())
}

func (nc *NetC) remoteAddr() string {
	if nc == nil {
		return ""
	}
	if nc.conn != nil {
		return nc.conn.RemoteAddr().String()
	}
	if nc.connws != nil {
		return nc.connws.RemoteAddr().String()
	}
	return ""
}

func (nc *NetC) watchCtx(ctx context.Context) {
	if nc == nil || ctx == nil {
		return
	}
	select {
	case <-ctx.Done():
		nc.shutdown()
	case <-nc.CloseC:
	}
}

func (nc *NetC) loopSend(ctx context.Context) {
	defer nc.shutdown()
	for {
		if nc.off.Load() {
			return
		}
		select {
		case <-ctxDone(ctx):
			return
		case <-nc.CloseC:
			return
		case data := <-nc.SendC:
			if nc.sendData == nil {
				return
			}
			if err := nc.sendData(data); err != nil {
				if !nc.off.Load() {
					filelog.Error("send err:", err, " id:", nc.Id)
				}
				return
			}
			nc.touch()
		}
	}
}

func (nc *NetC) loopRecv(ctx context.Context, handle func([]byte, int64)) {
	defer nc.shutdown()
	for {
		if nc.off.Load() {
			return
		}
		select {
		case <-ctxDone(ctx):
			return
		case <-nc.CloseC:
			return
		default:
		}
		if nc.readData == nil {
			return
		}
		data, flag, err := nc.readData()
		if err != nil {
			if flag != 0 {
				filelog.Error("read err:", err)
			} else if !nc.off.Load() {
				filelog.Log("连接断开:", nc.Id)
			}
			return
		}
		nc.touch()
		if len(data) < ProtoLen {
			filelog.Error("short packet, id:", nc.Id, " len:", len(data))
			return
		}
		protoId := binary.BigEndian.Uint16(data[:ProtoLen])
		if protoId == HEART {
			continue
		}
		if handle != nil {
			handle(data, nc.Id)
		}
	}
}

func ctxDone(ctx context.Context) <-chan struct{} {
	if ctx == nil {
		return nil
	}
	return ctx.Done()
}

func setTCPKeepAlive(conn net.Conn) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcp.SetKeepAlive(true)
	_ = tcp.SetKeepAlivePeriod(30 * time.Second)
}

func writeFull(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func (nc *NetC) readTcpData() ([]byte, uint16, error) {
	var bhead [MaxLength]byte
	_, err := io.ReadFull(nc.conn, bhead[:])
	if err != nil {
		return nil, 0, err
	}
	total := binary.BigEndian.Uint32(bhead[:])
	if total < MaxLength+ProtoLen || total > MaxLength+maxPayload {
		return nil, 1, errors.New(fmt.Sprint("data too big or little,length:", total, " ordata:", bhead[:]))
	}
	data := make([]byte, total-MaxLength)
	_, err = io.ReadFull(nc.conn, data)
	if err != nil {
		return nil, 0, err
	}
	return data, 0, nil
}

func (nc *NetC) sendTcpData(data []byte) error {
	if nc.conn == nil {
		return errors.New("tcp conn is nil, id:" + strconv.FormatInt(nc.Id, 10))
	}
	return writeFull(nc.conn, data)
}

func (nc *NetC) closeTcp() {
	if nc.conn != nil {
		_ = nc.conn.Close()
	}
}
