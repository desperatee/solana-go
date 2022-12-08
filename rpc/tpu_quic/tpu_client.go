package tpu_quic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/desperatee/solana-go"
	"github.com/desperatee/solana-go/rpc"
	"github.com/desperatee/solana-go/rpc/ws"
	"github.com/lucas-clemente/quic-go"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var MAX_SLOT_SKIP_DISTANCE uint64 = 48
var DEFAULT_FANOUT_SLOTS uint64 = 12
var MAX_FANOUT_SLOTS uint64 = 100

type LeaderTPUCache struct {
	LeaderTPUMap      map[string]string
	Connection        *rpc.Client
	FirstSlot         uint64
	SlotsInEpoch      uint64
	LastEpochInfoSlot uint64
	Leaders           []solana.PublicKey
}

func (leaderTPUCache *LeaderTPUCache) Load(connection *rpc.Client, startSlot uint64, fanout uint64) error {
	leaderTPUCache.Connection = connection
	epochInfo, err := leaderTPUCache.Connection.GetEpochInfo(rpc.CommitmentProcessed)
	if err != nil {
		return err
	}
	leaderTPUCache.SlotsInEpoch = epochInfo.SlotsInEpoch
	slotLeaders, err := leaderTPUCache.FetchSlotLeaders(startSlot, fanout, leaderTPUCache.SlotsInEpoch)
	if err != nil {
		return err
	}
	leaderTPUCache.Leaders = slotLeaders
	clusterTPUSockets, err := leaderTPUCache.FetchClusterTPUSockets()
	if err != nil {
		return err
	}
	leaderTPUCache.LeaderTPUMap = clusterTPUSockets
	return nil
}

func (leaderTPUCache *LeaderTPUCache) FetchSlotLeaders(startSlot uint64, fanoutSlots uint64, slotsInEpoch uint64) ([]solana.PublicKey, error) {
	fanout := uint64(math.Min(float64(2*fanoutSlots), float64(slotsInEpoch)))
	slotLeaders, err := leaderTPUCache.Connection.GetSlotLeaders(startSlot, fanout)
	if err != nil {
		return nil, err
	}
	return slotLeaders, nil
}

func (leaderTPUCache *LeaderTPUCache) FetchClusterTPUSockets() (map[string]string, error) {
	var clusterTPUSockets = make(map[string]string)
	clusterNodes, err := leaderTPUCache.Connection.GetClusterNodes()
	if err != nil {
		return nil, err
	}
	for _, contactInfo := range clusterNodes {
		if contactInfo.TPU != nil {
			clusterTPUSockets[contactInfo.Pubkey.String()] = *contactInfo.TPU
		}
	}
	return clusterTPUSockets, nil
}

func (leaderTPUCache *LeaderTPUCache) LastSlot() uint64 {
	return leaderTPUCache.FirstSlot + uint64(len(leaderTPUCache.Leaders)) - 1
}

func (leaderTPUCache *LeaderTPUCache) GetSlotLeader(slot uint64) solana.PublicKey {
	if slot >= leaderTPUCache.FirstSlot {
		return leaderTPUCache.Leaders[slot-leaderTPUCache.FirstSlot]
	} else {
		return solana.PublicKey{}
	}
}

func (leaderTPUCache *LeaderTPUCache) GetLeaderSockets(fanoutSlots uint64) []string {
	var alreadyCheckedLeaders []string
	var leaderTPUSockets []string
	var checkedSlots uint64 = 0
	for _, leader := range leaderTPUCache.Leaders {
		tpuSocket := leaderTPUCache.LeaderTPUMap[leader.String()]
		if tpuSocket != "" {
			isDuplicate := CheckIfDuplicate(alreadyCheckedLeaders, leader.String())
			if !isDuplicate {
				alreadyCheckedLeaders = append(alreadyCheckedLeaders, leader.String())
				leaderTPUSockets = append(leaderTPUSockets, tpuSocket)
			}
		}
		checkedSlots++
		if checkedSlots == fanoutSlots {
			return leaderTPUSockets
		}
	}
	return leaderTPUSockets
}

type RecentLeaderSlots struct {
	RecentSlots []float64
}

func (recentLeaderSlots *RecentLeaderSlots) Load(currentSlot uint64) {
	recentLeaderSlots.RecentSlots = append(recentLeaderSlots.RecentSlots, float64(currentSlot))
}

func (recentLeaderSlots *RecentLeaderSlots) RecordSlot(currentSlot uint64) {
	recentLeaderSlots.RecentSlots = append(recentLeaderSlots.RecentSlots, float64(currentSlot))
	for len(recentLeaderSlots.RecentSlots) > int(DEFAULT_FANOUT_SLOTS) {
		recentLeaderSlots.RecentSlots = recentLeaderSlots.RecentSlots[1:]
	}
}

func (recentLeaderSlots *RecentLeaderSlots) EstimatedCurrentSlot() uint64 {
	if len(recentLeaderSlots.RecentSlots) == 0 {
		return 0
	}
	recentSlots := recentLeaderSlots.RecentSlots
	sort.Float64s(recentSlots)
	maxIndex := len(recentSlots) - 1
	medianIndex := maxIndex / 2
	medianRecentSlot := recentSlots[medianIndex]
	expectedCurrentSlot := uint64(medianRecentSlot) + uint64(maxIndex-medianIndex)
	maxReasonableCurrentSlot := expectedCurrentSlot + MAX_SLOT_SKIP_DISTANCE
	sort.Sort(sort.Reverse(sort.Float64Slice(recentSlots)))
	var slotToReturn uint64 = 0
	for _, slot := range recentSlots {
		if uint64(slot) <= maxReasonableCurrentSlot && uint64(slot) > slotToReturn {
			slotToReturn = uint64(slot)
		}
	}
	return slotToReturn
}

type LeaderTPUService struct {
	RecentSlots       *RecentLeaderSlots
	LTPUCache         *LeaderTPUCache
	Subscription      *ws.SlotsUpdatesSubscription
	Connection        *rpc.Client
	WSConnection      *ws.Client
	LeaderConnections []quic.EarlyConnection
	QUICTokenStore    quic.TokenStore
}

func (leaderTPUService *LeaderTPUService) Load(connection *rpc.Client, websocketURL string, fanout uint64) error {
	leaderTPUService.Connection = connection
	slot, err := leaderTPUService.Connection.GetSlot(rpc.CommitmentProcessed)
	leaderTPUService.QUICTokenStore = quic.NewLRUTokenStore(1, 10)
	if err != nil {
		return err
	}
	recentSlots := RecentLeaderSlots{}
	recentSlots.Load(slot)
	leaderTPUService.RecentSlots = &recentSlots
	leaderTPUCache := LeaderTPUCache{}
	err = leaderTPUCache.Load(leaderTPUService.Connection, slot, fanout)
	if err != nil {
		return err
	}
	leaderTPUService.LTPUCache = &leaderTPUCache
	if websocketURL != "" {
		wsConnection, err := ws.Connect(context.TODO(), websocketURL)
		if err == nil {
			subscription, err := wsConnection.SlotsUpdatesSubscribe()
			if err == nil {
				leaderTPUService.Subscription = subscription
				go func() {
					for {
						message, err := leaderTPUService.Subscription.Recv()
						if err == nil {
							//Slot already full, skip over 1 slot.
							if message.Type == ws.SlotsUpdatesCompleted {
								leaderTPUService.RecentSlots.RecordSlot(message.Slot + 1)
							}
							//Slot received first shred, it's still accepting transactions so we record.
							if message.Type == ws.SlotsUpdatesFirstShredReceived {
								leaderTPUService.RecentSlots.RecordSlot(message.Slot)
							}
						}
					}
				}()
			} else {
				leaderTPUService.Connection = nil
			}
		} else {
			leaderTPUService.Connection = nil
		}
	} else {
		leaderTPUService.Connection = nil
	}
	go leaderTPUService.Run(fanout)
	return nil
}

func (leaderTPUService *LeaderTPUService) LeaderTPUSockets(fanoutSlots uint64) []string {
	return leaderTPUService.LTPUCache.GetLeaderSockets(fanoutSlots)
}

func (leaderTPUService *LeaderTPUService) LeaderTPUSocketsWithConn(fanoutSlots uint64) []string {
	sockets := leaderTPUService.LTPUCache.GetLeaderSockets(fanoutSlots)
	var conns []quic.EarlyConnection
	for _, socket := range sockets {
		socketSplit := strings.Split(socket, ":")
		port, err := strconv.Atoi(socketSplit[1])
		if err != nil {
			fmt.Println(err)
			continue
		}
		ip := fmt.Sprintf("%v:%v", socketSplit[0], port+6)
		_, cert, err := NewSelfSignedTLSCertificate(net.ParseIP("0.0.0.0"))
		if err != nil {
			fmt.Println(err)
		}
		connectionTries := 0
		var connection quic.EarlyConnection
		for {
			conn, err := quic.DialAddrEarly(ip, &tls.Config{
				InsecureSkipVerify: true,
				NextProtos:         []string{"solana-tpu"},
				Certificates:       []tls.Certificate{cert},
				//ServerName:         "Solana node",
				//CipherSuites: []uint16{
				//	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				//	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				//	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				//	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
				//	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				//	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				//	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				//	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				//	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				//	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				//},
				//CurvePreferences: []tls.CurveID{
				//	tls.X25519,
				//	tls.CurveP256,
				//	tls.CurveP384,
				//	tls.CurveP521,
				//},
			}, &quic.Config{
				KeepAlivePeriod:            1 * time.Second,
				MaxIdleTimeout:             2 * time.Second,
				MaxStreamReceiveWindow:     1252 * 256,
				MaxConnectionReceiveWindow: 1252 * 256,
				MaxIncomingUniStreams:      256,
				DisablePathMTUDiscovery:    true,
				EnableDatagrams:            false,
				TokenStore:                 leaderTPUService.QUICTokenStore,
			})
			if err != nil {
				fmt.Println(err)
				if connectionTries < 3 {
					connectionTries++
					continue
				} else {
					break
				}
			}
			connection = conn
			break
		}
		if connection != nil {
			conns = append(conns, connection)
		}
		//streamTries := 0
		//var stream quic.SendStream
		//for {
		//	strm, err := connection.OpenUniStream()
		//	if err != nil {
		//		fmt.Println(err)
		//		if streamTries < 3 {
		//			streamTries++
		//			continue
		//		} else {
		//			break
		//		}
		//	}
		//	stream = strm
		//	break
		//}
		//if stream != nil {
		//	streams = append(streams, stream)
		//}
	}
	for _, old := range leaderTPUService.LeaderConnections {
		old.CloseWithError(0, "")
	}
	//for _, old := range leaderTPUService.LeaderStreams {
	//	old.Close()
	//}
	leaderTPUService.LeaderConnections = conns
	//leaderTPUService.LeaderStreams = streams
	return sockets
}

func (leaderTPUService *LeaderTPUService) GetLeaderConnections(fanoutSlots uint64) []quic.Connection {
	sockets := leaderTPUService.LTPUCache.GetLeaderSockets(fanoutSlots)
	var conns []quic.Connection
	for _, socket := range sockets {
		socketSplit := strings.Split(socket, ":")
		port, err := strconv.Atoi(socketSplit[1])
		if err != nil {
			fmt.Println(err)
			continue
		}
		ip := fmt.Sprintf("%v:%v", socketSplit[0], port+6)
		_, cert, err := NewSelfSignedTLSCertificate(net.ParseIP("127.0.0.1"))
		if err != nil {
			fmt.Println(err)
		}
		connectionTries := 0
		var connection quic.Connection
		for {
			conn, err := quic.DialAddrEarly(ip, &tls.Config{
				InsecureSkipVerify: true,
				NextProtos:         []string{"solana-tpu"},
				Certificates:       []tls.Certificate{cert},
				//ServerName:         "Solana node",
				//CipherSuites: []uint16{
				//	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				//	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				//	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				//	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
				//	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				//	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				//	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				//	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				//	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				//	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				//},
				//CurvePreferences: []tls.CurveID{
				//	tls.X25519,
				//	tls.CurveP256,
				//	tls.CurveP384,
				//	tls.CurveP521,
				//},
			}, &quic.Config{
				KeepAlivePeriod:            1 * time.Second,
				MaxIdleTimeout:             2 * time.Second,
				MaxStreamReceiveWindow:     1252 * 256,
				MaxConnectionReceiveWindow: 1252 * 256,
				MaxIncomingUniStreams:      256,
				DisablePathMTUDiscovery:    true,
				EnableDatagrams:            false,
			})
			if err != nil {
				fmt.Println(err)
				if connectionTries < 3 {
					connectionTries++
					continue
				} else {
					break
				}
			}
			connection = conn
			break
		}
		if connection != nil {
			conns = append(conns, connection)
		}
		//streamTries := 0
		//var stream quic.SendStream
		//for {
		//	strm, err := connection.OpenUniStream()
		//	if err != nil {
		//		fmt.Println(err)
		//		if streamTries < 3 {
		//			streamTries++
		//			continue
		//		} else {
		//			break
		//		}
		//	}
		//	stream = strm
		//	break
		//}
		//if stream != nil {
		//	streams = append(streams, stream)
		//}
	}
	return conns
}

func (leaderTPUService *LeaderTPUService) Run(fanout uint64) {
	var lastClusterRefreshTime = time.Now()
	for {
		if time.Now().UnixMilli()-lastClusterRefreshTime.UnixMilli() > 5000*60 {
			latestTPUSockets, err := leaderTPUService.LTPUCache.FetchClusterTPUSockets()
			if err != nil {
				continue
			}
			leaderTPUService.LTPUCache.LeaderTPUMap = latestTPUSockets
			leaderTPUService.LeaderTPUSocketsWithConn(fanout)
			lastClusterRefreshTime = time.Now()
		}

		time.Sleep(1 * time.Second)
		currentSlot := leaderTPUService.RecentSlots.EstimatedCurrentSlot()
		if int64(currentSlot) >= int64(leaderTPUService.LTPUCache.LastEpochInfoSlot)-int64(leaderTPUService.LTPUCache.SlotsInEpoch) {
			latestEpochInfo, err := leaderTPUService.Connection.GetEpochInfo(rpc.CommitmentProcessed)
			if err != nil {
				continue
			}
			leaderTPUService.LTPUCache.SlotsInEpoch = latestEpochInfo.SlotsInEpoch
			leaderTPUService.LTPUCache.LastEpochInfoSlot = latestEpochInfo.AbsoluteSlot
		}
		if currentSlot >= (leaderTPUService.LTPUCache.LastSlot() - fanout) {
			slotLeaders, err := leaderTPUService.LTPUCache.FetchSlotLeaders(currentSlot, fanout, leaderTPUService.LTPUCache.SlotsInEpoch)
			if err != nil {
				continue
			}
			leaderTPUService.LTPUCache.FirstSlot = currentSlot
			leaderTPUService.LTPUCache.Leaders = slotLeaders
		}
	}
}

type TPUClientConfig struct {
	FanoutSlots uint64
}

type TPUClient struct {
	FanoutSlots uint64
	LTPUService *LeaderTPUService
	Exit        bool
	Connection  *rpc.Client
	SendMutex   sync.Mutex
}

func (tpuClient *TPUClient) Load(connection *rpc.Client, websocketURL string, config TPUClientConfig) error {
	tpuClient.Connection = connection
	tpuClient.FanoutSlots = uint64(math.Max(math.Min(float64(config.FanoutSlots), float64(MAX_FANOUT_SLOTS)), 1))
	tpuClient.Exit = false
	leaderTPUService := LeaderTPUService{}
	tpuClient.LTPUService = &leaderTPUService
	err := tpuClient.LTPUService.Load(tpuClient.Connection, websocketURL, tpuClient.FanoutSlots)
	if err != nil {
		return err
	}
	tpuClient.LTPUService.LeaderTPUSocketsWithConn(tpuClient.FanoutSlots)
	return nil
}

func (tpuClient *TPUClient) SendTransaction(transaction *solana.Transaction, amount int) (solana.Signature, error) {
	rawTransaction, err := transaction.MarshalBinary()
	if err != nil {
		return solana.Signature{}, err
	}
	err = tpuClient.SendRawTransaction(rawTransaction, amount)
	if err != nil {
		return solana.Signature{}, err
	}
	return transaction.Signatures[0], nil
}

func (tpuClient *TPUClient) SendTransactionSameConn(transaction *solana.Transaction, amount int) (solana.Signature, error) {
	rawTransaction, err := transaction.MarshalBinary()
	if err != nil {
		return solana.Signature{}, err
	}
	err = tpuClient.SendRawTransactionSameConn(rawTransaction, amount)
	if err != nil {
		return solana.Signature{}, err
	}
	return transaction.Signatures[0], nil
}

func (tpuClient *TPUClient) SendRawTransaction(transaction []byte, amount int) error {
	var successes = 0
	var lastError = ""
	leaderTPUSockets := tpuClient.LTPUService.LeaderTPUSockets(tpuClient.FanoutSlots)
	for _, leader := range leaderTPUSockets {
		var connectionTries = 0
		var failed = false
		var connection net.Conn
		for {
			conn, err := net.Dial("udp", leader)
			if err != nil {
				lastError = err.Error()
				if connectionTries < 3 {
					connectionTries++
					continue
				} else {
					failed = true
					break
				}
			}
			connection = conn
			break
		}
		if failed == true {
			continue
		}
		for i := 0; i < amount; i++ {
			_, err := connection.Write(transaction)
			if err != nil {
				lastError = err.Error()
			} else {
				successes++
			}
		}
		connection.Close()
	}
	if successes == 0 {
		return errors.New(lastError)
	} else {
		return nil
	}
}

func (tpuClient *TPUClient) SendRawTransactionSameConn(transaction []byte, amount int) error {
	var success = 0
	var lastError error
	for _, connection := range tpuClient.LTPUService.LeaderConnections {
		for i := 0; i < amount; i++ {
			tpuClient.SendMutex.Lock()
			retries := 0
			var stream quic.SendStream
			for {
				strm, err := connection.NextConnection().OpenUniStreamSync(context.Background())
				if err != nil {
					if retries < 3 {
						retries++
						continue
					} else {
						tpuClient.SendMutex.Unlock()
						return err
					}
				}
				stream = strm
				break
			}
			_, err := stream.Write(transaction)
			if err == nil {
				success++
			} else {
				lastError = err
			}
			stream.Close()
			tpuClient.SendMutex.Unlock()
		}
	}
	if success == 0 {
		return lastError
	} else {
		return nil
	}
}

func New(connection *rpc.Client, websocketURL string, config TPUClientConfig) (*TPUClient, error) {
	tpuClient := TPUClient{}
	err := tpuClient.Load(connection, websocketURL, config)
	return &tpuClient, err
}
