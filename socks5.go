package socks

import (
	"errors"
	"net"
)

func (cfg *config) dialSocks5(targetAddr string) (_ net.Conn, err error) {
	proxy := cfg.Host

	// dial TCP
	conn, err := net.DialTimeout("tcp", proxy, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	var req requestBuilder

	version := byte(5) // socks version 5
	method := byte(0)  // method 0: no authentication (only anonymous access supported for now)
	if cfg.Auth != nil {
		method = 2 // method 2: username/password
	}

	// version identifier/method selection request
	req.add(
		version, // socks version
		1,       // number of methods
		method,
	)

	resp, err := cfg.sendReceive(conn, req.Bytes())
	if err != nil {
		return nil, err
	} else if len(resp) != 2 {
		return nil, errors.New("server does not respond properly")
	} else if resp[0] != 5 {
		return nil, errors.New("server does not support Socks 5")
	} else if resp[1] != method {
		return nil, errors.New("socks method negotiation failed")
	}
	if cfg.Auth != nil {
		version := byte(1) // user/password version 1
		req.Reset()
		req.add(
			version,                      // user/password version
			byte(len(cfg.Auth.Username)), // length of username
		)
		req.add([]byte(cfg.Auth.Username)...)
		req.add(byte(len(cfg.Auth.Password)))
		req.add([]byte(cfg.Auth.Password)...)
		resp, err := cfg.sendReceive(conn, req.Bytes())
		if err != nil {
			return nil, err
		} else if len(resp) != 2 {
			return nil, errors.New("server does not respond properly")
		} else if resp[0] != version {
			return nil, errors.New("server does not support user/password version 1")
		} else if resp[1] != 0 { // not success
			return nil, errors.New("user/password login failed")
		}
	}

	// detail request
	host, port, err := splitHostPort(targetAddr)
	if err != nil {
		return nil, err
	}

	addrType, ip := getAddrType(host)

	req.Reset()
	req.add(
		5,        // version number
		1,        // connect command
		0,        // reserved, must be zero
		addrType, // address type, 3 means domain name
	)
	if addrType == 3 {
		req.add(byte(len(host)))
		req.add([]byte(host)...)
	}
	if addrType == 1 {
		req.add(ip...)
	}
	if addrType == 4 {
		req.add(ip...)
	}

	req.add(
		byte(port>>8), // higher byte of destination port
		byte(port),    // lower byte of destination port (big endian)
	)
	resp, err = cfg.sendReceive(conn, req.Bytes())
	if err != nil {
		return
	} else if len(resp) != 10 {
		return nil, errors.New("server does not respond properly")
	} else if resp[1] != 0 {
		return nil, errors.New("can't complete SOCKS5 connection")
	}

	return conn, nil
}
