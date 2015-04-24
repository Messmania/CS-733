package raft

import (
	"encoding/gob"
	//"fmt"
	"io"
	"net"
	"strconv"
)

func (r *Raft) connHandler(f int, e int) {
	//go r.listenToServers()
	go r.listenToServers()
	go r.listenToClients()
	go r.ServerSM(f, e)
}

//==============+Assign4+===========

//timeout param added Only for testing
func (r *Raft) ServerSM(f int, e int) {
	state := follower //how to define type for this?--const
	for {
		switch state {
		case follower:
			//fmt.Println("in case follower")
			state = r.follower(f)
		case candidate:
			//fmt.Println("in case candidate of ServSM()")
			state = r.candidate(e)
		case leader:
			//fmt.Println("in case leader")
			state = r.leader()
		default:
			return

		}
	}
}

func (r *Raft) listenToServers() {
	port := r.Myconfig.LogPort
	service := r.Myconfig.Hostname + ":" + strconv.Itoa(port)
	tcpaddr, err := net.ResolveTCPAddr("tcp", service)
	if err != nil {
		checkErr("Error in listenToServers(),ResolveTCPAddr", err)
		return
	} else {
		listener, err := net.ListenTCP("tcp", tcpaddr)
		if err != nil {
			checkErr("Error in listenToServers(),ListenTCP", err)
			return
		} else {
			for {
				conn, err := listener.Accept()
				if err != nil {
					checkErr("Error in listenToServers(),Accept", err)
					continue
				} else if conn != nil { //Added if to remove nil pointer reference error
					go r.writeToEvCh(conn)
				}
			}

		}
	}
}

func (r *Raft) listenToClients() {
	port := r.Myconfig.ClientPort
	service := r.Myconfig.Hostname + ":" + strconv.Itoa(port)
	tcpaddr, err := net.ResolveTCPAddr("tcp", service)
	if err != nil {
		checkErr("Error in listenToClients(),ResolveTCPAddr", err)
		return
	} else {
		listener, err := net.ListenTCP("tcp", tcpaddr)
		if err != nil {
			checkErr("Error in listenToClients(),ListenTCP", err)
			return
		} else {
			for {
				conn, err := listener.Accept()
				if err != nil {
					continue
				} else if conn != nil { //Added if to remove nil pointer reference error
					//fmt.Println("Connection made at server side", r.myId())
					go r.handleClient(conn)
				}
			}

		}
	}
}

func (r *Raft) writeToEvCh(conn net.Conn) {
	//r.registerTypes()
	for {
		msg, err := r.DecodeInterface(conn)
		if err != nil {
			if err != io.EOF {
				checkErr("Error in writeToEvCh(),DecodeInterface", err)
			}
			return
		}
		r.EventCh <- msg
	}
}

func (r *Raft) registerTypes() {
	gob.Register(AppendEntriesReq{})
	gob.Register(AppendEntriesResponse{})
	gob.Register(RequestVote{})
	gob.Register(RequestVoteResponse{})
	gob.Register(ClientAppendReq{})
	gob.Register(ClientAppendResponse{})
}

//For decoding the values
func (r *Raft) DecodeInterface(conn net.Conn) (interface{}, error) {
	r.registerTypes()
	dec_net := gob.NewDecoder(conn)
	var obj_dec interface{}
	err_dec := dec_net.Decode(&obj_dec)
	if err_dec != nil {
		if err_dec == io.EOF {
			checkErr("Client closed the connection,bye!", nil)
			return nil, err_dec
		} else {
			checkErr("In DecodeInterface, err is:", err_dec)
			return nil, err_dec
		}

	}
	return obj_dec, nil
}

//to convert client reads and write to conn Read and write rather than gob, so telnet can be used
func (r *Raft) handleClient(conn net.Conn) {
	for {
		var msg [512]byte
		n, err := conn.Read(msg[0:])
		if err == nil && conn != nil {
			cmd := msg
			logEntry, err := r.Append(cmd[0:n])
			connMapMutex.Lock()
			connLog[&logEntry] = conn //write the logEntry:conn to map
			connMapMutex.Unlock()

			if err == nil {
				//fmt.Println("launched kvstore")
				go r.kvStoreProcessing(&logEntry)

			} else {
				//fmt.Println("Wrong leader")
				ldrHost, ldrPort := r.LeaderConfig.Hostname, r.LeaderConfig.ClientPort
				errRedirectStr := "ERR_REDIRECT " + ldrHost + " " + strconv.Itoa(ldrPort)
				_, err1 := conn.Write([]byte(errRedirectStr))
				if err1 != nil {
					checkErr("Error in writing redirect string to conn", err1)
				}
			}
		} else {
			return
		}
	}
}
