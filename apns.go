package manbearpig

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// MAX_POOL_SIZE is how many sockets to open for connection
	// pooling.
	MAX_POOL_SIZE int = 20
)

var apnsUrls = map[string]string{
	"sandbox":    "gateway.sandbox.push.apple.com:2195",
	"production": "gateway.push.apple.com:2195",
}

var errText = map[uint8]string{
	0:   "No errors encountered",
	1:   "Processing Errors",
	2:   "Missing Device Token",
	3:   "Missing Topic",
	4:   "Missing Payload",
	5:   "Invalid Token Size",
	6:   "Invalid Topic Size",
	7:   "Invalid Payload Size",
	8:   "Invalid Token",
	255: "None (Unknown)",
}

// APNSResult is what apple returns.
type APNSResult struct {
	msgId  uint32
	status uint8
	err    error
}

// APNS [...]
type APNS struct {
	Pool map[string]*APNSConnPool
	mu   *sync.Mutex
}

// APNSConnPool for connection pooling.
type APNSConnPool struct {
	conn     chan *APNSConn
	nClients int
}

// APNSConn [...]
type APNSConn struct {
	tlsConn        *tls.Conn
	tlsCfg         tls.Config
	endpoint       string
	ReadTimeout    time.Duration
	mu             sync.Mutex // Sync the Apns connections
	transactionId  uint32
	MaxPayloadSize int // default to 256 as per Apple specifications (June 9 2012)
	connected      bool
}

// connect opens a socket with the APNS service if
// the client status is not yet connected.
// If it is connected then it is closed first before
// reopening.
func (client *APNSConn) connect() (err error) {
	if client.connected {
		return nil
	}

	if client.tlsConn != nil {
		client.Close()
	}

	conn, err := net.Dial("tcp", client.endpoint)
	if err != nil {
		return err
	}

	client.tlsConn = tls.Client(conn, &client.tlsCfg)
	err = client.tlsConn.Handshake()
	if err == nil {
		client.connected = true
	}

	return err
}

// Close [...]
func (client *APNSConn) Close() (err error) {
	err = nil
	if client.tlsConn != nil {
		err = client.tlsConn.Close()
		client.connected = false
	}
	return
}

// Get pops off a connection from the pool channel.
func (p *APNSConnPool) Get() *APNSConn {
	return <-p.conn
}

// Release puts a connection back into the connection pool.
func (p *APNSConnPool) Release(conn *APNSConn) {
	p.conn <- conn
}

// NewAPNSConnPool establishes connections with the APNS service.
func NewAPNSConnPool(certificate, key []byte) (*APNSConnPool, error) {
	conn := make(chan *APNSConn, MAX_POOL_SIZE)
	n := 0
	for x := 0; x < MAX_POOL_SIZE; x++ {
		c, err := NewAPNSClient(certificate, key)
		if err != nil {
			// Possible errors are missing/invalid environment which would be caught earlier.
			// Most likely invalid cert
			return nil, err
		}
		conn <- c
		n++
	}
	return &APNSConnPool{conn, n}, nil
}

// NewClient creates a new apns connection. endpoint and certificate.
func NewAPNSClient(certificate, key []byte) (*APNSConn, error) {
	cert, err := tls.X509KeyPair(certificate, key)
	if err != nil {
		return nil, err
	}
	endpoint, ok := apnsUrls["production"]
	if !ok {
		return nil, fmt.Errorf("MissingEnvironment:")
	}

	apnsConn := &APNSConn{
		tlsConn: nil,
		tlsCfg: tls.Config{
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{cert}},
		endpoint:       endpoint,
		mu:             sync.Mutex{},
		ReadTimeout:    150 * time.Millisecond,
		MaxPayloadSize: 256,
		connected:      false,
	}

	return apnsConn, nil
}

// Push [...]
func (a APNS) Push(notification *Notification, authKey string) *PushStatus {
	ps := NewPushStatus(notification)

	if len(notification.DeviceTokens) == 0 {
		ps.Errors[""] = fmt.Errorf("NoDeviceTokens")
		return ps
	}

	for _, devToken := range notification.DeviceTokens {
		btoken, err := hex.DecodeString(devToken)
		if err != nil {
			log.Printf("%s", err)
			ps.Errors[devToken] = err
			return ps
		}

		payload, ok := notification.Payload["payload"].(string)
		if !ok {
			log.Printf("Invalid payload, should be string but got %T", notification.Payload)
			ps.Errors[devToken] = fmt.Errorf("InvalidJSON")
			return ps
		}
		bpayload := []byte(payload)

		a.mu.Lock()
		pool, ok := a.Pool[notification.AppName]
		if !ok {
			pool, err = NewAPNSConnPool([]byte(authKey), []byte(authKey))
			if err != nil {
				ps.Errors[""] = err
				log.Printf("%s", err)
				return ps
			}
			a.Pool[notification.AppName] = pool
		}
		a.mu.Unlock()
		client := pool.Get()
		defer pool.Release(client)
		err = client.connect()
		if err != nil {
			ps.Retry = true
			ps.Errors[devToken] = err
			return ps
		}

		// https://developer.apple.com/library/mac/#documentation/
		// NetworkingInternet/Conceptual/RemoteNotificationsPG/Chapters/
		// CommunicatingWIthAPS.html#//apple_ref/doc/uid/TP40008194-CH101-SW4
		if len(bpayload) > client.MaxPayloadSize {
			log.Printf("MessageTooBig: given: %v max: %v", len(bpayload), client.MaxPayloadSize)
			ps.Errors[devToken] = fmt.Errorf("MessageTooBig")
			return ps
		}

		// build the actual pdu
		buffer := bytes.NewBuffer([]byte{})

		// command
		binary.Write(buffer, binary.BigEndian, uint8(1))

		// transaction id, optional
		binary.Write(buffer, binary.BigEndian, uint32(1))

		// expiration time, default 1 hour
		expiry := notification.Expiry
		if expiry == 0 {
			unixNow := uint32(time.Now().Unix())
			expiry = unixNow + 60*60
		}
		binary.Write(buffer, binary.BigEndian, expiry)

		// push device token
		binary.Write(buffer, binary.BigEndian, uint16(len(btoken)))
		binary.Write(buffer, binary.BigEndian, btoken)

		// push payload
		binary.Write(buffer, binary.BigEndian, uint16(len(bpayload)))
		binary.Write(buffer, binary.BigEndian, bpayload)
		pdu := buffer.Bytes()

		// write pdu
		client.mu.Lock()
		defer client.mu.Unlock()
		_, err = client.tlsConn.Write(pdu)
		if err != nil {
			log.Printf("%s", err)
			client.connected = false
			ps.Retry = true
			ps.Errors[devToken] = fmt.Errorf("ClientNotConnected")
			return ps
		}
		// wait for error pdu from the socket
		client.tlsConn.SetReadDeadline(time.Now().Add(client.ReadTimeout))

		readb := [6]byte{}
		n, err := client.tlsConn.Read(readb[:])
		if err != nil {
			e2, ok := err.(net.Error)
			if ok && e2.Timeout() {
				// Success, apns doesn't usually return a response if successful.
				// Only issue is, is timeout length long enough (150ms) for err response.
				ps.Successes = 1
				return ps
			}
			log.Printf("%s", err)
			ps.Errors[devToken] = err
			return ps
		}

		if n >= 0 {
			var status uint8 = uint8(readb[1])
			switch status {
			case 0:
				// OK
				ps.Successes = 1
				return ps
			case 1:
				// Processing errors
				log.Printf("%s", errText[status])
				ps.Retry = true
				ps.Errors[devToken] = fmt.Errorf("%s", errText[status])
				return ps
			case 2, 3, 4, 5, 6, 7, 8:
				//2:   "Missing Device Token",
				//3:   "Missing Topic",
				//4:   "Missing Payload",
				//5:   "Invalid Token Size",
				//6:   "Invalid Topic Size",
				//7:   "Invalid Payload Size",
				//8:   "Invalid Token",
				log.Printf("%s", errText[status])
				ps.Errors[devToken] = fmt.Errorf("%s", errText[status])
				return ps
			case 255:
				log.Printf("Unknown error code %v", hex.EncodeToString(readb[:n]))
				ps.Retry = true
				ps.Errors[devToken] = fmt.Errorf("Unknown")
				return ps
			default:
				log.Printf("Unknown error code %v", hex.EncodeToString(readb[:n]))
				ps.Errors[devToken] = fmt.Errorf("UnknownAPNS")
				return ps
			}
		}
	}

	return ps
}
