package utils

import (
	"time"
	"net"
)

//noinspection GoUnusedParameter
func Use(unused interface{}) {
}

func StaticClock(sec int64) func() (time.Time) {
	return func() (time.Time) {
		return time.Unix(sec, 0)
	}
}

func GetFreeTcpPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
