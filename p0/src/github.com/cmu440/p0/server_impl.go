// Implementation of a MultiEchoServer. Students should write their code in this file.

package p0

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
)

type client struct {
	Connection net.Conn
	Reader     *bufio.Reader
	// his buffer is used to buffer the message which should be sent to client
	// Accoring to the description of assignment, the size of this buffer
	// is 100.
	WriteBuffer chan string
	Index       int
}

func NewClient(conn net.Conn) *client {
	client := new(client)
	client.Connection = conn
	client.Reader = bufio.NewReader(conn)
	client.WriteBuffer = make(chan string, 100)
	client.Index = -1
	return client
}

type multiEchoServer struct {
	// TODO: implement this!
	MaxConnectionNum int
	// This Array is used to save the connection whcih connected to server
	ClientPool []*client
	// The following four buffer is used to handle the atomic
	// operation on server side.
	AddChanel     chan net.Conn
	DeleteChannel chan int
	CountChannel  chan int
	ReturnCHannel chan int
	// This channel is used to buffer the message the server received
	// THe client buffer in client object
	// continue reading message from this buffer
	// and send the message to client
	AddMsgChannel chan string
	//
	QuitChannel chan int
}

// New creates and returns (but does not start) a new MultiEchoServer.
func New() MultiEchoServer {
	// TODO: implement this!
	server := new(multiEchoServer)
	server.MaxConnectionNum = 100
	server.ClientPool = make([]*client, server.MaxConnectionNum)
	server.AddChanel = make(chan net.Conn)
	server.DeleteChannel = make(chan int)
	server.CountChannel = make(chan int)
	server.ReturnCHannel = make(chan int)
	server.AddMsgChannel = make(chan string, 100)
	server.QuitChannel = make(chan int)
	return server
}

func (mes *multiEchoServer) Start(port int) error {
	// TODO: implement this!
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))

	if err != nil {
		return err
	}
	// start go routine to manage client which connect to the server
	go mes.ManageClient()

	fmt.Println("Start listen")
	go func() {
		for {
			// start to handle connection request
			conn, err := ln.Accept()
			if err != nil {
				fmt.Println("error")
			}
			// add connection to add buffer and wait server to hanle it.
			mes.AddChanel <- conn
			<-mes.ReturnCHannel
		}
	}()
	return nil
}

func (mes *multiEchoServer) Close() {
	// TODO: implement this!
	mes.QuitChannel <- 1
}

func (mes *multiEchoServer) Count() int {
	// TODO: implement this!
	// send conunt singla to count buffer and wait server to hanle it.
	mes.CountChannel <- 0
	return <-mes.ReturnCHannel
}

// TODO: add additional methods/functions below!
// This functio is used to manage add, delete, count and add message to client buffer operation,
// Since these four operations touch the clientpool, in any time, only one of them can be operated.
// Thus I use select to guarantee this and avoid data race.
func (mes *multiEchoServer) ManageClient() {
	// TODO: implement this!
	for {
		// select guarantee there is no data race between add delete and count clients.
		select {
		case conn := <-mes.AddChanel:
			//find availplace place
			index := -1
			for i := 0; i <= len(mes.ClientPool)-1; i++ {
				if mes.ClientPool[i] == nil {
					index = i
					break
				}
			}
			if index < 0 {
				// full
				fmt.Println("Client pool is full")
				conn.Close()
			} else {
				// add to pool
				fmt.Println("Get new client")
				client := NewClient(conn)
				client.Connection = conn
				client.Index = index
				mes.ClientPool[index] = client
				// add add client to client pool, start go routine to read message from this client
				go mes.ServerRead(client)
				// add add client to client pool, start go routine to write message to client
				go client.ClientWrite()
				mes.ReturnCHannel <- 0
			}

		case index := <-mes.DeleteChannel:
			mes.ClientPool[index] = nil
			mes.ReturnCHannel <- 0

		case <-mes.CountChannel:
			count := 0
			for _, client := range mes.ClientPool {
				if client != nil {
					count++
				}
			}
			// send result back to main go routine through channel
			mes.ReturnCHannel <- count

		case line := <-mes.AddMsgChannel:
			// read message from the server and write to the write buffer of each client
			for _, client := range mes.ClientPool {
				if client != nil && len(client.WriteBuffer) <= 99 {
					client.WriteBuffer <- line
				}
			}
			mes.ReturnCHannel <- 1

		case <-mes.QuitChannel:
			return
		}
	}
}

// This function is used to read message from client side and add message to the clients' writebuffer
// If any time there is an EOF return from client side, I delete this client from client pool.
func (mes *multiEchoServer) ServerRead(client *client) {
	for {
		line, err := client.Reader.ReadString('\n')
		if err != nil {
			mes.DeleteChannel <- client.Index
			<-mes.ReturnCHannel
			return
		}
		go func(line string) {
			mes.AddMsgChannel <- line
			<-mes.ReturnCHannel
		}(line)
	}
}

// This function is used to read message from clients' writebuffer and send it to client
func (client *client) ClientWrite() {
	for {
		line := <-client.WriteBuffer
		_, err := client.Connection.Write([]byte(line))
		if err != nil {
			fmt.Println("Fail to write client:", client.Index, ". Error message:", err.Error())
			return
		}
	}
}
