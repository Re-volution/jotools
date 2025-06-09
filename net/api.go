package net

import (
	"encoding/binary"
	"wb3/jotools/chantypes"
	"wb3/jotools/dencode"
	"wb3/jotools/filelog"
)

const MaxLength = 4
const ProtoLen = 2

func parseData(data []byte, protoid uint16) []byte {
	var totlelen = MaxLength + ProtoLen + uint32(len(data))
	var wdata = make([]byte, 0, totlelen)

	wdata = binary.BigEndian.AppendUint32(wdata, totlelen)
	wdata = binary.BigEndian.AppendUint16(wdata, protoid)
	if len(data) > 0 {
		wdata = append(wdata, data...)
	}
	//filelog.Debug("parseData len:", totlelen, " protoid:", protoid, " data:", string(data))
	return wdata
}

// 发送给玩家
func (n *NetManger) SendTimeById(msg int64, id int64, protoid uint16) {
	var totlelen = MaxLength + ProtoLen + 8
	var wdata = make([]byte, 0, totlelen)

	wdata = binary.BigEndian.AppendUint32(wdata, uint32(totlelen))
	wdata = binary.BigEndian.AppendUint16(wdata, protoid)
	wdata = binary.BigEndian.AppendUint64(wdata, uint64(msg))

	n.cLock.RLock()
	conn := n.conn[id]
	n.cLock.RUnlock()
	if conn == nil {
		return
	}
	chantypes.TryWriteChan(conn.sendc, wdata, true)

}

// 发送给玩家
func (n *NetManger) SendById(msg interface{}, id int64, protoid uint16) {
	var data []byte
	if msg != nil {
		data, _ = dencode.Marshal(msg)
	}

	wdata := parseData(data, protoid)

	n.cLock.RLock()
	conn := n.conn[id]
	n.cLock.RUnlock()
	if conn == nil {
		filelog.Warring("not found this nid:", id)
		return
	}
	filelog.Debug("SendById id：", id, " protoid:", protoid)
	chantypes.TryWriteChan(conn.sendc, wdata, true)
}

// 随机发送
func (n *NetManger) SendRandC(msg interface{}, protoid uint16) {
	var data []byte
	if msg != nil {
		data, _ = dencode.Marshal(msg)
	}

	wdata := parseData(data, protoid)
	var conn *netC
	n.cLock.RLock()
	for _, v := range n.conn {
		conn = v
		break
	}

	n.cLock.RUnlock()
	if conn == nil {
		filelog.Error("无可用连接", string(data), protoid)
		return
	}
	chantypes.TryWriteChan(conn.sendc, wdata, true)
	//filelog.Debug("消息发送成功 protoid：", protoid, " data:", string(data))
}

func (n *NetManger) SendAllConn(msg interface{}, protoid uint16) {
	var data []byte
	if msg != nil {
		data, _ = dencode.Marshal(msg)
	}

	wdata := parseData(data, protoid)
	var conns []*netC
	n.cLock.RLock()
	for _, conn := range n.conn {
		conns = append(conns, conn)
	}
	n.cLock.RUnlock()
	if len(conns) == 0 {
		return
	}
	for _, conn := range conns {
		chantypes.TryWriteChan(conn.sendc, wdata, true)
	}
	filelog.Debug("SendAllConn protoid:", protoid)
}

// 发送给玩家
func (n *NetManger) Close(nid int64) {
	n.cLock.RLock()
	conn := n.conn[nid]
	n.cLock.RUnlock()
	if conn == nil {
		filelog.Warring("Close not found this nid:", nid)
		return
	}
	conn.conn.Close()
}
