/*
 * Copyright (C) 2018 The go-contentbox Authors
 * This file is part of The go-contentbox library.
 *
 * The go-contentbox library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The go-contentbox library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The go-contentbox library.  If not, see <http://www.gnu.org/licenses/>.
 */

package p2p

import (
	"bufio"

	libp2pnet "github.com/libp2p/go-libp2p-net"
)

type Conn struct {
	stream libp2pnet.Stream
}

func NewConn(stream libp2pnet.Stream) *Conn {
	return &Conn{stream: stream}
}

func (conn *Conn) readData(rw *bufio.ReadWriter) {
	buf := make([]byte, 1024)
	messageBuffer := make([]byte, 0)
	var header *MessageHeader

	for {
		n, err := rw.Read(buf)
		if err != nil {
			conn.stream.Close()
			return
		}
		messageBuffer = append(messageBuffer, buf[:n]...)
		for {
			if header == nil {
				var err error
				if len(messageBuffer) < HeaderLength {
					break
				}
				header, err = ParseHeader(messageBuffer)
				if err != nil {
					return
				}
				messageBuffer = messageBuffer[HeaderLength:]
			}

			if len(messageBuffer) < header.DataLength {
				break
			}
			body := messageBuffer[:header.DataLength]
			messageBuffer = messageBuffer[header.DataLength:]
			conn.handle(header.Code, body)
			header = nil
		}
	}

}

func (conn *Conn) writeData(rw *bufio.ReadWriter) {

	for {
		select {}
	}

}

func (conn *Conn) handle(messageCode uint32, body []byte) {

	switch messageCode {
	case Ping:
		conn.onPing(body)
	case Pong:
		conn.onPong(body)
	}
}

func (conn *Conn) onPing(data []byte) {

}

func (conn *Conn) onPong(data []byte) {

}
