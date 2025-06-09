package net

import (
	"context"
	"github.com/Re-volution/ltime"
	"net"
	"sync"
	"time"
	"wb3/jotools/chantypes"
	"wb3/jotools/filelog"
	"wb3/jotools/recover_p"
)

type reData struct {
	addr     string
	offlineh func()
	h        func([]byte, int64)
}

type reDatas struct {
	Data []chan bool
	sync.Mutex
}

func (rds *reDatas) add(d chan bool) {
	rds.Lock()
	defer rds.Unlock()
	rds.Data = append(rds.Data, d)
}

func (netM *NetManger) Conn(cxt context.Context) {
	var ch = make(chan bool, 1)
	netM.reConns.add(ch)

	conn, err := net.Dial("tcp", netM.addr)
	if err != nil {
		filelog.Error("Dial fail ,err:", err, " addr:", netM.addr)
		chantypes.TryWriteChan(ch, true, true)
		return
	}
	filelog.Log("连接成功,", netM.addr)
	netM.sucessH()

	nconn := netM.newConn(conn, ch)

	recover_p.Go(func() { netM.send(nconn, cxt) })
	recover_p.Go(func() { netM.recv(nconn, cxt) })

	return
}

// Conn 调用此方法会生成一条新的连接，如果方法相同请使用 NetManger下的Conn接口
func Conn(addr string, h func([]byte, int64), offlineh func(), successh func(), cxt context.Context) *NetManger {
	var netM = new(NetManger)
	netM.idd = new(int64)
	netM.conn = make(map[int64]*netC)
	netM.addr = addr
	netM.handle = h
	netM.reConns = new(reDatas)
	netM.reConns.Data = make([]chan bool, 0, 16)
	recover_p.Go(func() {
		netM.reConn(cxt)
	})

	netM.connFailH = func(c *netC, rech chan bool) {
		chantypes.TryWriteChan(c.closec, true, false)
		netM.delConnByC(c)
		chantypes.TryWriteChan(rech, true, true)
		offlineh()
	}
	netM.sucessH = successh
	netsM.conns = append(netsM.conns, netM)

	netM.Conn(cxt)
	return netM
}

type reconData struct {
	retime int64 //下一次重连时间
}

func (netM *NetManger) reConn(cxt context.Context) {
	var reconns = make([]*reconData, 0, 16)
	for {
		select {
		case <-cxt.Done():
			filelog.Log("重连系统关闭")
			return
		default:
		}
		netM.reConns.Lock()
		if len(netM.reConns.Data) != 0 {
			for i := 0; i < len(netM.reConns.Data); i++ {
				select {
				case <-netM.reConns.Data[i]:
					var temp = new(reconData)
					temp.retime = ltime.GetS() + 3
					reconns = append(reconns, temp)
					netM.reConns.Data = append(netM.reConns.Data[:i], netM.reConns.Data[i+1:]...)
					i--
				default:
				}
			}

		}
		netM.reConns.Unlock()

		if len(reconns) == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		for i := 0; i < len(reconns); i++ {
			if ltime.GetS() > reconns[i].retime {
				filelog.Log("正在重连:" + netM.addr)
				netM.Conn(cxt)
				reconns = append(reconns[:i], reconns[i+1:]...)
				i--
			}
		}
		time.Sleep(3 * time.Second)
	}
}
