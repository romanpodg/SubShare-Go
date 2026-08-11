package profileconfig

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"sync"
	"time"

	hyclient "github.com/apernet/hysteria/core/v2/client"
	hyerrors "github.com/apernet/hysteria/core/v2/errors"
	"github.com/apernet/hysteria/extras/v2/obfs"
	"github.com/apernet/hysteria/extras/v2/transport/udphop"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
)

type hysteria2PacketConnFactory struct {
	ctx                 context.Context
	hopAddress          *udphop.UDPHopAddr
	obfuscationType     string
	obfuscationPassword []byte

	mu   sync.Mutex
	used bool
}

func (factory *hysteria2PacketConnFactory) New(net.Addr) (net.PacketConn, error) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	if factory.used {
		return nil, errors.New("hysteria2 connection factory already used")
	}
	factory.used = true

	openUDP := func() (net.PacketConn, error) { return net.ListenUDP("udp", nil) }
	var conn net.PacketConn
	var err error
	if factory.hopAddress != nil {
		conn, err = udphop.NewUDPHopPacketConn(factory.hopAddress, udphop.HopIntervalConfig{}, openUDP)
	} else {
		conn, err = openUDP()
	}
	if err != nil {
		return nil, err
	}
	wrapped, err := wrapHysteria2Obfuscation(conn, factory.obfuscationType, factory.obfuscationPassword)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if deadline, ok := factory.ctx.Deadline(); ok {
		_ = wrapped.SetDeadline(deadline)
	}
	go func() {
		<-factory.ctx.Done()
		_ = wrapped.Close()
	}()
	return wrapped, nil
}

func wrapHysteria2Obfuscation(conn net.PacketConn, obfuscationType string, password []byte) (net.PacketConn, error) {
	switch obfuscationType {
	case "":
		return conn, nil
	case "salamander":
		return obfs.WrapPacketConnSalamander(conn, password)
	case "gecko":
		return obfs.WrapPacketConnGecko(conn, obfs.GeckoOptions{Password: password})
	default:
		return nil, errors.New("unsupported hysteria2 obfuscation")
	}
}

func performHysteria2Handshake(ctx context.Context, profile *profiles.Profile, addresses []netip.Addr) error {
	data, ok := profile.Data.(profiles.Hysteria2Data)
	if !ok {
		return errors.New("invalid hysteria2 profile model")
	}
	var lastErr error
	for _, address := range addresses {
		if err := ctx.Err(); err != nil {
			return err
		}
		serverAddress, hopAddress := hysteria2ServerAddress(address, profile.Port)
		factory := &hysteria2PacketConnFactory{
			ctx:                 ctx,
			hopAddress:          hopAddress,
			obfuscationType:     data.ObfuscationType,
			obfuscationPassword: []byte(data.ObfuscationPassword.Reveal()),
		}
		config := &hyclient.Config{
			ConnFactory: factory,
			ServerAddr:  serverAddress,
			Auth:        data.Authentication.Reveal(),
			TLSConfig: hyclient.TLSConfig{
				ServerName:         hysteria2ServerName(profile.Server, data.SNI),
				InsecureSkipVerify: data.Insecure,
			},
			QUICConfig: hyclient.QUICConfig{
				MaxIdleTimeout:  configurationProbeTimeout,
				KeepAlivePeriod: 2 * time.Second,
			},
		}
		if data.CertificateSHA256 != "" {
			config.TLSConfig.VerifyPeerCertificate = pinnedCertificateVerifier(data.CertificateSHA256)
		}
		client, _, err := hyclient.NewClient(config)
		if err == nil {
			_ = client.Close()
			return nil
		}
		lastErr = err
	}
	return lastErr
}

func hysteria2ServerName(host, configuredSNI string) string {
	if configuredSNI != "" {
		return configuredSNI
	}
	return host
}

func hysteria2ServerAddress(address netip.Addr, port profiles.PortSpec) (net.Addr, *udphop.UDPHopAddr) {
	ip := net.IP(address.AsSlice())
	if port.Kind == profiles.PortSingle {
		return &net.UDPAddr{IP: ip, Port: int(port.Ranges[0].Start)}, nil
	}
	ports := make([]uint16, 0)
	for _, interval := range port.Ranges {
		for candidate := uint32(interval.Start); candidate <= uint32(interval.End); candidate++ {
			ports = append(ports, uint16(candidate))
		}
	}
	hopAddress := &udphop.UDPHopAddr{IP: ip, Ports: ports, PortStr: port.Expression}
	return hopAddress, hopAddress
}

func pinnedCertificateVerifier(expected string) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCertificates [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCertificates) == 0 {
			return errors.New("hysteria2 certificate missing")
		}
		hash := sha256.Sum256(rawCertificates[0])
		if hex.EncodeToString(hash[:]) != expected {
			return errors.New("hysteria2 certificate pin mismatch")
		}
		return nil
	}
}

func classifyHysteria2ProbeError(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return "hysteria2_timeout"
	}
	var authenticationError hyerrors.AuthError
	if errors.As(err, &authenticationError) {
		return "hysteria2_authentication_failed"
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return "hysteria2_timeout"
	}
	return "hysteria2_connection_failed"
}
