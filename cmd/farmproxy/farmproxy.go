package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"

	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds)
}

var ftcpport = flag.Int("tcpport", 13333, "listen tcp port")
var fredirectaddr = flag.String("tcpto", "hello.com:443", "tcp redirect to tls addr")
var ftlsport = flag.Int("tlsport", 14333, "listen tls port")
var ftlscrtfile = flag.String("tlscert", "server.crt", "tls cert file")
var ftlskeyfile = flag.String("tlskey", "server.key", "tls key file")
var ftlstoaddr = flag.String("tlsto", "hello.com:80", "tls redirect to tcp addr")

func main() {
	log.Println("main")
	flag.Usage()
	flag.Parse()
	log.Println("listen tcp port:", *ftcpport)
	log.Println("tcp redirect to tls addr:", *fredirectaddr)
	log.Println("listen tls port:", *ftlsport)
	log.Println("tls redirect to tcp addr:", *ftlstoaddr)
	Start()
	select {}
}

var glistenctx, glistencancer = context.WithCancel(context.Background())
var glistentlsctx, glistentlscancer = context.WithCancel(context.Background())

func Start() {
	go startlisten()
	go startlistentls()
}

func Safeclose(closer io.Closer) (rterr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Safeclose", r)
			rterr = fmt.Errorf("r:%v", r)
			return
		}
	}()
	rterr = closer.Close()
	return
}

func startlisten() {
	lc := net.ListenConfig{}
	lis, err := lc.Listen(glistenctx, "tcp", fmt.Sprintf(":%d", *ftcpport))
	if err != nil || lis == nil {
		log.Println("tcp.listen", err)
		return
	}
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("tcp.listen.Accept", err)
			break
		}
		if conn == nil {
			continue
		}
		go func() {
			dcc, err := tls.Dial("tcp", *fredirectaddr, nil)
			if err != nil || dcc == nil {
				log.Println("Dial tls", err, dcc)
				Safeclose(conn)
				return
			}
			go func() {
				io.Copy(dcc, conn)
				//log.Println("copy1",n,err)
				Safeclose(dcc)
				Safeclose(conn)
			}()
			go func() {
				io.Copy(conn, dcc)
				//log.Println("copy2",n,err)
				Safeclose(dcc)
				Safeclose(conn)
			}()
		}()
	}
}

func startlistentls() {
	lis, err := (&net.ListenConfig{}).Listen(glistentlsctx, "tcp", fmt.Sprintf(":%d", *ftlsport))
	if err != nil || lis == nil {
		log.Println("tls.listen", err)
		return
	}
	cert, err := tls.LoadX509KeyPair(*ftlscrtfile, *ftlskeyfile)
	if err != nil {
		log.Println("opentlsfile", err)
		return
	}
	tlscfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("tls.listen.Accept", err)
			break
		}
		if conn == nil {
			continue
		}
		conn = tls.Server(conn, tlscfg)
		go func() {
			dcc, err := net.Dial("tcp", *ftlstoaddr)
			if err != nil || dcc == nil {
				log.Println("Dial tcp", err, dcc)
				Safeclose(conn)
				return
			}
			go func() {
				io.Copy(dcc, conn)
				//log.Println("copy1",n,err)
				Safeclose(dcc)
				Safeclose(conn)
			}()
			go func() {
				io.Copy(conn, dcc)
				//log.Println("copy2",n,err)
				Safeclose(dcc)
				Safeclose(conn)
			}()
		}()
	}
}

func Stop() {
	log.Println("Stop")
	glistencancer()
	glistentlscancer()
}
