package main

import (
        "flag"
        "fmt"
        "github.com/paulbellamy/ratecounter"
        "github.com/tv42/topic"
        "net"
        "os"
        "time"
)

var countInbound = 0
var countOutbound = 0
var countMessagesInbound = ratecounter.NewRateCounter(60 * time.Second)
var countMessagesOutbound = ratecounter.NewRateCounter(60 * time.Second)

func main() {

        var msgsize int
        flag.IntVar(&msgsize, "msgsize", 4096, "Maximum message size")

        top := topic.New()
        go startInboundTCP(top, msgsize)
        go startInboundUDP(top, msgsize)
        go startOutboundTCP(top)
        for {
                time.Sleep(60 * time.Second)
                msgSecIn := countMessagesInbound.Rate() / 60
                msgSecOut := countMessagesOutbound.Rate() / 60
                fmt.Printf("Inbound: %v clients, %v msg/sec. Outbound: %v clients, %v msg/sec\n",
                        countInbound, msgSecIn, countOutbound, msgSecOut)
        }
}

func startInboundUDP(top *topic.Topic, msgsize int) {
        pc, err := net.ListenPacket("udp", "0.0.0.0:2000")
        if err != nil {
                fmt.Println("Error binding UDP: ", err.Error())
                os.Exit(1)
        }
        defer pc.Close()
        buf := make([]byte, msgsize)
        for {
                pc.ReadFrom(buf)
                countMessagesInbound.Incr(1)
                top.Broadcast <- buf
        }
}

func startInboundTCP(top *topic.Topic, msgsize int) {
        l, err := net.Listen("tcp", "0.0.0.0:2000")
        if err != nil {
                fmt.Println("Error binding TCP: ", err.Error())
                os.Exit(1)
        }
        defer l.Close()
        for {
                conn, err := l.Accept()
                if err != nil {
                        fmt.Println("Error accepting connection: ", err.Error())
                        continue
                }
                go handleInbound(conn, top, msgsize)
        }
}

func startOutboundTCP(top *topic.Topic) {
        l, err := net.Listen("tcp", "0.0.0.0:3000")
        if err != nil {
                fmt.Println("Error binding port: ", err.Error())
                os.Exit(1)
        }
        defer l.Close()
        for {
                conn, err := l.Accept()
                if err != nil {
                        fmt.Println("Error accepting connection: ", err.Error())
                        continue
                }
                go handleOutbound(conn, top)
        }
}

func handleInboundExit() {
        countInbound--
}

func handleOutboundExit() {
        countOutbound--
}

func handleInbound(conn net.Conn, top *topic.Topic, msgsize int) {
        defer conn.Close()
        defer handleInboundExit()
        countInbound++
        for {
                buf := make([]byte, msgsize)
                _, err := conn.Read(buf)
                if err != nil {
                        conn.Close()
                        return
                }
                countMessagesInbound.Incr(1)
                top.Broadcast <- buf
        }
}

func handleOutbound(conn net.Conn, top *topic.Topic) {
        defer conn.Close()
        defer handleOutboundExit()
        countOutbound++

        consumer := make(chan interface{}, 10)
        defer top.Unregister(consumer)
        top.Register(consumer)

        for {
                select {
                case msg, ok := <-consumer:
                        if ok {
                                _, err := conn.Write(msg.([]byte))
                                if err != nil {
                                        conn.Close()
                                        return
                                }
                                countMessagesOutbound.Incr(1)
                        } else {
                                break
                        }
                }
        }
}
