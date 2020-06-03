package main

import (
	"fmt"
	"log"
	"net"
	"strings"
)

type Server struct {
	// server监听udp地址
	laddr string

	// 其他宿主机的laddr
	peers []*Node

	// 与其他宿主机的udp connect
	peerConns []*peerConn

	// 虚拟设备接口
	iface *Interface
}

type peerConn struct {
	conn *net.UDPConn
	cidr string
}

func NewServer(laddr string, peers []*Node, iface *Interface) *Server {
	return &Server{
		laddr:     laddr,
		peers:     peers,
		peerConns: make([]*peerConn, 0),
		iface:     iface,
	}
}

func (s *Server) ListenAndServe() error {
	laddr, err := net.ResolveUDPAddr("udp", s.laddr)
	if err != nil {
		return err
	}

	lconn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	defer lconn.Close()

	s.connectPeers()

	go s.readLocal(lconn)
	s.readRemote(lconn)
	return nil
}

func (s *Server) connectPeers() {
	for _, node := range s.peers {
		raddr, err := net.ResolveUDPAddr("udp", node.Addr)
		if err != nil {
			log.Println("[E] ", err)
			continue
		}

		conn, err := net.DialUDP("udp", nil, raddr)
		if err != nil {
			log.Println("[E] ", err)
			continue
		}

		out, err := execCmd("route", []string{"add", "-net",
			node.CIDR, "dev", s.iface.tun.Name()})

		if err != nil {
			log.Println("[E] add route fail: ", err, out)
		}

		log.Printf("[I] add route %s to %s\n", node.CIDR, s.iface.tun.Name())

		peer := &peerConn{
			conn: conn,
			cidr: node.CIDR,
		}

		s.peerConns = append(s.peerConns, peer)
	}
}

func (s *Server) readRemote(lconn *net.UDPConn) {
	buf := make([]byte, 1024*64)
	for {
		nr, _, err := lconn.ReadFromUDP(buf)
		if err != nil {
			log.Println(err)
			return
		}

		p := Packet(buf[:nr])
		if p.Invalid() {
			log.Println("[E] invalid ipv4 packet")
			continue
		}

		src := p.Src()
		dst := p.Dst()
		log.Printf("[D] %s => %s\n", src, dst)

		s.iface.Write(buf[:nr])
	}
}

func (s *Server) readLocal(lconn *net.UDPConn) {
	for {
		buf, err := s.iface.Read()
		if err != nil {
			log.Println("[E] read iface error: ", err)
			continue
		}

		p := Packet(buf)
		if p.Invalid() {
			log.Println("[E] invalid ipv4 packet")
			continue
		}

		src := p.Src()
		dst := p.Dst()
		log.Printf("[D] %s => %s\n", src, dst)

		peer, err := s.route(dst)
		if err != nil {
			log.Println("[E] not route to host: ", dst)
			continue
		}

		_, err = peer.Write(buf)
		if err != nil {
			log.Println("[E] write to peer: ", err)
		}
	}
}

func (s *Server) route(dst string) (*net.UDPConn, error) {
	for _, p := range s.peerConns {
		_, ipnet, err := net.ParseCIDR(p.cidr)
		if err != nil {
			log.Println("parse cidr fail: ", err)
			continue
		}

		sp := strings.Split(p.cidr, "/")
		if len(sp) != 2 {
			log.Println("parse cidr fail: ", err)
			continue
		}

		dstCidr := fmt.Sprintf("%s/%s", dst, sp[1])
		_, dstNet, err := net.ParseCIDR(dstCidr)
		if err != nil {
			log.Println("parse cidr fail: ", err)
			continue
		}

		if ipnet.String() == dstNet.String() {
			return p.conn, nil
		}
	}

	return nil, fmt.Errorf("no route")
}