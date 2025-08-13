package proxy

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

func readAndWrite(ctx context.Context, connToRead net.Conn, connToWrite net.Conn, cancelConn context.CancelFunc, wg *sync.WaitGroup, bufPool *sync.Pool) {
	defer wg.Done()
	buf := bufPool.Get().([]byte)
	defer bufPool.Put(&buf)

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		//nolint:errcheck
		connToRead.Close()
		//nolint:errcheck
		connToWrite.Close()
	}()

	for {
		n, err := connToRead.Read(buf)
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) {
				log.Printf("Error reading %v: %v", connToRead.RemoteAddr(), err)
			}
			if tcpConn, ok := connToRead.(*net.TCPConn); ok {
				//nolint:errcheck
				tcpConn.CloseWrite()
			}
			cancelConn()
			return
		}

		written := 0
		for written < n {
			newWritten, writeErr := connToWrite.Write(buf[written:n])
			if writeErr != nil {
				log.Printf("write to %v error: %v", connToWrite.RemoteAddr(), writeErr)
				if tcpConn, ok := connToWrite.(*net.TCPConn); ok {
					//nolint:errcheck
					tcpConn.CloseRead()
				}
				cancelConn()
				return
			}
			written += newWritten
		}
	}
}

func handle(parentCtx context.Context, client net.Conn, backendAddr string, wg *sync.WaitGroup, bufPool *sync.Pool) {
	defer wg.Done()
	connCtx, cancelConn := context.WithCancel(parentCtx)
	defer cancelConn()
	//nolint:errcheck
	defer client.Close()

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	backend, err := dialer.DialContext(connCtx, "tcp", backendAddr)
	if err != nil {
		log.Printf("Error connecting to backend: %s\n", err)
		return
	}
	//nolint:errcheck
	defer backend.Close()

	wg.Add(2)
	go readAndWrite(connCtx, client, backend, cancelConn, wg, bufPool)
	go readAndWrite(connCtx, backend, client, cancelConn, wg, bufPool)

	<-connCtx.Done()
}
