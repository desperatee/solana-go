package tpu_quic

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/lucas-clemente/quic-go"
	"math/rand"
	"net"
	"sync"
	"time"
)

type ConnectionPool struct {
	mutex              sync.Mutex
	connections        map[string][]*quic.Connection
	quicConfig         quic.Config
	quicTokenStore     quic.TokenStore
	tlsConfig          tls.Config
	CurrentConnections []string
	Sockets            []*net.UDPConn
}

func NewConnectionPool() (*ConnectionPool, error) {
	pool := ConnectionPool{}
	pool.quicConfig = pool.GetDefaultQUICConfiguration()
	tlsConfig, err := pool.GetDefaultTLSConfiguration()
	if err != nil {
		return &pool, err
	}
	pool.tlsConfig = tlsConfig
	pool.quicTokenStore = quic.NewLRUTokenStore(999, 999)
	pool.connections = make(map[string][]*quic.Connection)
	return &pool, nil
}

func (p *ConnectionPool) GetDefaultQUICConfiguration() quic.Config {
	return quic.Config{
		KeepAlivePeriod:            1 * time.Second,
		MaxIdleTimeout:             2 * time.Second,
		MaxStreamReceiveWindow:     1252 * 256,
		MaxConnectionReceiveWindow: 1252 * 256,
		MaxIncomingUniStreams:      256,
		DisablePathMTUDiscovery:    true,
		EnableDatagrams:            false,
		TokenStore:                 p.quicTokenStore,
	}
}

func (p *ConnectionPool) GetDefaultTLSConfiguration() (tls.Config, error) {
	cert, err := NewSelfSignedTLSCertificate(net.ParseIP("127.0.0.1"))
	if err != nil {
		return tls.Config{}, err
	}
	return tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"solana-tpu"},
		Certificates:       []tls.Certificate{cert},
	}, nil
}

func (p *ConnectionPool) Create(address string) error {
	udpSocket, err := net.ListenUDP("udp", nil)
	if err != nil {
		return err
	}
	p.mutex.Lock()
	p.Sockets = append(p.Sockets, udpSocket)
	p.mutex.Unlock()
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}
	var conn quic.Connection
	tries := 0
	for {
		ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
		connection, err := quic.DialContext(ctx, udpSocket, udpAddr, address, &p.tlsConfig, &p.quicConfig)
		if err != nil {
			if tries < 3 {
				tries++
				continue
			} else {
				return err
			}
		}
		conn = connection
		break
	}

	p.mutex.Lock()
	currentConns := p.connections[address]
	currentConns = append(currentConns, &conn)
	p.connections[address] = currentConns
	if !p.CheckIfDuplicate(p.CurrentConnections, address) {
		p.CurrentConnections = append(p.CurrentConnections, address)
	}
	p.mutex.Unlock()
	return nil
}

func (p *ConnectionPool) CheckIfDuplicate(arr []string, obj string) bool {
	for _, i := range arr {
		if i == obj {
			return true
		}
	}
	return false
}

func (p *ConnectionPool) Get(address string) (quic.Connection, error) {
	source := rand.NewSource(time.Now().UnixNano())
	generator := rand.New(source)
	if len(p.connections[address]) == 0 {
		return nil, errors.New("no connections")
	}
	return *p.connections[address][generator.Intn(len(p.connections[address]))], nil
}

func (p *ConnectionPool) Clear() {
	p.mutex.Lock()
	for _, socket := range p.Sockets {
		socket.Close()
	}
	for _, conns := range p.connections {
		for _, conn := range conns {
			(*conn).CloseWithError(0, "")
		}
	}
	p.connections = make(map[string][]*quic.Connection)
	p.mutex.Unlock()
}
