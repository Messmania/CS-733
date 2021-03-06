package raft

import (
	"encoding/gob"
	"encoding/json"
	//"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"time"
)

type LSN uint64 //Log sequence number, unique for all time.

//-- Log entry interface and its implementation
type LogEntry interface {
	Lsn() LSN
	Data() []byte
	Committed() bool
}

//--implementation
type LogItem struct {
	lsn         LSN
	isCommitted bool
	data        []byte
}

func (l LogItem) Lsn() LSN {
	return l.lsn
}
func (l LogItem) Data() []byte {
	return l.data
}
func (l LogItem) Committed() bool {
	return l.isCommitted
}

type ServerConfig struct {
	Id         int    // Id of server. Must be unique
	Hostname   string // name or ip of host
	ClientPort int    // port at which server listens to client messages.
	LogPort    int    // tcp port for inter-replica protocol messages.
}

type ClusterConfig struct {
	Path    string         //re-added for ease of using exec file
	Servers []ServerConfig // All servers in this cluster
}

type SharedLog interface {
	Append(Data []byte) (LogEntry, error)
}

// Raft implements the SharedLog interface.
type Raft struct {
	ClusterConfigObj   ClusterConfig
	Myconfig           ServerConfig
	LeaderConfig       ServerConfig
	CurrentLogEntryCnt LSN
	CommitCh           chan *LogEntry

	EventCh chan interface{}

	//Persistent state on all servers--ON DISK
	myCV  TermVotedFor
	MyLog []LogVal

	//Volatile state on all servers
	MyMetaData LogMetaData

	//used by leader only
	f_specific map[int]*followerDetails //map of serverId-details

	//Path strings to files on disk
	Path_CV  string
	Path_Log string
}

type TermVotedFor struct {
	CurrentTerm int
	VotedFor    int
}

//used by leader to track details specific to a follower
type followerDetails struct {
	Vote      bool
	sConn     net.Conn
	nextIndex int
}

func (r *Raft) Append(Data []byte) (LogEntry, error) {
	obj := ClientAppendReq{Data}
	//r.send(r.Myconfig.Id, obj)
	r.EventCh <- obj
	//fmt.Println(r.myId(), "In Append,Data sent to channel", string(Data))
	response := <-r.CommitCh

	if !(*response).Committed() { //if committed is false then it means this server is not the leader--
		// what if server is the leader but entry is not committed, will it be false then or it doesn't write to chann? mp it doesn't write to CommitCh--CHECK
		var e error = ErrRedirect(r.LeaderConfig.Id)
		return nil, e
	} else {
		return *response, nil
	}
}

type ErrRedirect int

//==========Assign -4 ========
var hostname string = "localhost"

//==========================Addition for assgn3============
//temp for testing
const layout = "3:04:5 pm (MST)"

//For converting default time unit of ns to millisecs
var msecs time.Duration = time.Millisecond * 100 ///increased to x10, with conn only MilliSecond might be too small, hence failing--changed to x100

//var msecs time.Duration = time.Second //for testing

const majority int = 3

const noOfServers int = 5
const (
	ElectionTimeout      = iota
	HeartbeatTimeout     = iota
	AppendEntriesTimeOut = iota
	RetryTimeOut         = iota
	ResendVoteTimeOut    = iota
)

//type LogDB []LogVal //log is array of type LogVal
type LogVal struct {
	Term int
	Cmd  []byte
	Acks int
}
type LogMetaData struct {
	LastLogIndex int //last entry in log,corresponding Term is Term in LogVal, can also be calculated as len(LogDB)-1
	PrevLogIndex int // value is LastLogIndex-1 always so not needed as such,---Change this later
	PrevLogTerm  int
	CommitIndex  int
}
type AppendEntriesReq struct {
	Term               int
	LeaderId           int
	PrevLogIndex       int
	PrevLogTerm        int
	Entries            []byte
	LeaderCommitIndex  int
	LeaderLastLogIndex int
	LeaderLastLogTerm  int
}
type AppendEntriesResponse struct {
	Term         int
	Success      bool
	FollowerId   int
	IsHeartBeat  bool
	LastLogIndex int //added for parallel appends!-4 April-03:25am
}
type ClientAppendReq struct {
	Data []byte
}
type ClientAppendResponse struct {
	LogEntry LogEntry
	RetError error
}
type RequestVote struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}
type RequestVoteResponse struct {
	Term        int
	VoteGranted bool
	Id          int //added so that leader can keep track of Votes from a server rather than VoteCount
}

//===========Assgn-4==========

func (r *Raft) makeConnection(port int) net.Conn {
	service := hostname + ":" + strconv.Itoa(port)
	tcpAddr, err := net.ResolveTCPAddr("tcp", service)
	if err != nil {
		checkErr("Error in makeConnection,ResolveTCP", err)
		return nil
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		msg := "Err in net.Resolve in makeConn is"
		checkErr(msg, err)
		return nil
	} else {
		return conn
	}
}

//Encodes the msg into gob
func (r *Raft) EncodeInterface(conn net.Conn, msg interface{}) int {
	r.registerTypes()
	enc_net := gob.NewEncoder(conn)
	msgPtr := &msg
	err_enc := enc_net.Encode(msgPtr)
	if err_enc != nil {
		msg := r.myId() + ", Error in EncodeInterface"
		checkErr(msg, err_enc)
		return -1
	}
	return 0
}

//changed to r.send to access port of serverId
func (r *Raft) send(serverId int, msg interface{}) {
	port := r.ClusterConfigObj.Servers[serverId].LogPort
	//added to reduce conns!
	conn := r.getSenderConn(serverId)
	if conn == nil {
		conn = r.makeConnection(port)
		if conn == nil {
			return //returns cz it will retry making connection in next HB
		} else {
			r.setSenderConn(serverId, conn)
		}
	}
	//if conn has been closed by server(but still present in my map) or it crashed(so this conn is meaningless now),
	//encode will throw error, then set the conn map as nil for this
	err := r.EncodeInterface(conn, msg)
	if err != 0 {
		r.setSenderConn(serverId, nil)
	}

}

func (r *Raft) setSenderConn(serverId int, conn net.Conn) {
	r.f_specific[serverId].sConn = conn
}

func (r *Raft) getSenderConn(serverId int) net.Conn {
	conn := r.f_specific[serverId].sConn
	return conn

}

func (r *Raft) receive() interface{} {
	request := <-r.EventCh
	return request
}

func (r *Raft) sendToAll_AppendReq(msg []interface{}) {
	for k := range r.ClusterConfigObj.Servers {
		if r.Myconfig.Id != k { //send to all except self
			go r.send(k, msg[k])
		}
	}
}

func (r *Raft) sendToAll(msg interface{}) {
	//==Assgn4
	//make connections with all and dump msg after encoding with gob to all conns
	for k, _ := range r.ClusterConfigObj.Servers {
		if r.Myconfig.Id != k { //send to all except self
			go r.send(k, msg)
		}
	}

}

//Keeps looping and performing follower functions till it timesout and changes to candidate
func (r *Raft) follower(timeout int) int {
	waitTime := timeout                                        //start heartbeat timer,timeout func wil place HeartbeatTimeout on channel
	HeartBeatTimer := r.StartTimer(HeartbeatTimeout, waitTime) //start the timer to wait for HBs
	for {
		req := r.receive()
		switch req.(type) {
		case AppendEntriesReq:
			request := req.(AppendEntriesReq) //explicit typecasting
			r.serviceAppendEntriesReq(request, HeartBeatTimer, waitTime, follower)
		case RequestVote:
			request := req.(RequestVote)
			r.serviceRequestVote(request, follower)
		case ClientAppendReq: //follower can't handle clients and redirects to leader, sends upto CommitCh as well as clientCh
			request := req.(ClientAppendReq) //explicit typecasting
			response := ClientAppendResponse{}
			logItem := LogItem{r.CurrentLogEntryCnt, false, request.Data} //lsn is count started from 0
			r.CurrentLogEntryCnt += 1
			response.LogEntry = logItem
			r.CommitCh <- &response.LogEntry
		case int:
			HeartBeatTimer.Stop()
			return candidate
		}
	}
}

//conducts election, returns only when state is changed else keeps looping on outer loop(i.e. restarting elections)
func (r *Raft) candidate(timeout int) int {
	waitTime := timeout //added for passing timeout from outside--In SingleServerBinary
	resendTime := 5     //should be much smaller than waitTime
	ElectionTimer := r.StartTimer(ElectionTimeout, waitTime)
	//This loop is for election process which keeps on going until a leader is elected
	for {
		//reset the Votes else it will reflect the Votes received in last Term
		r.resetVotes()
		r.myCV.CurrentTerm += 1                 //increment current Term
		r.myCV.VotedFor = r.Myconfig.Id         //Vote for self
		r.WriteCVToDisk()                       //write Current Term and VotedFor to disk
		r.f_specific[r.Myconfig.Id].Vote = true //vote true
		reqVoteObj := r.prepRequestVote()       //prepare request Vote obj
		r.sendToAll(reqVoteObj)                 //send requests for Vote to all servers
		ResendVoteTimer := r.StartTimer(ResendVoteTimeOut, resendTime)
		for { //this loop for reading responses from all servers
			req := r.receive()
			switch req.(type) {
			case ClientAppendReq: ///candidate must also respond as false just like follower
				request := req.(ClientAppendReq) //explicit typecasting
				response := ClientAppendResponse{}
				logItem := LogItem{r.CurrentLogEntryCnt, false, request.Data} //lsn is count started from 0
				r.CurrentLogEntryCnt += 1
				response.LogEntry = logItem
				r.CommitCh <- &response.LogEntry
			case RequestVoteResponse: //got the Vote response
				response := req.(RequestVoteResponse) //explicit typecasting so that fields of struct can be used
				if response.VoteGranted {
					r.f_specific[response.Id].Vote = true
				}
				VoteCount := r.countVotes()
				if VoteCount >= majority {
					ResendVoteTimer.Stop()
					ElectionTimer.Stop()
					r.LeaderConfig.Id = r.Myconfig.Id //update leader details
					return leader                     //become the leader
				}

			case AppendEntriesReq: //received an AE request instead of Votes, i.e. some other leader has been elected
				request := req.(AppendEntriesReq)
				retVal := r.serviceAppendEntriesReq(request, nil, 0, candidate)
				if retVal == follower {
					ResendVoteTimer.Stop()
					ElectionTimer.Stop()
					return follower
				}

			case RequestVote:
				request := req.(RequestVote)
				//==Can be shared with service request vote with additinal param of caller(candidate or follower)
				response := RequestVoteResponse{} //prep response object,for responding back to requester
				candidateId := request.CandidateId
				response.Id = r.Myconfig.Id
				if r.isDeservingCandidate(request) {
					response.VoteGranted = true
					r.myCV.VotedFor = candidateId
					r.myCV.CurrentTerm = request.Term
					if request.Term > r.myCV.CurrentTerm { //write to disk only when value has changed
						r.WriteCVToDisk()
					}
					ResendVoteTimer.Stop()
					ElectionTimer.Stop()
					return follower
				} else {
					response.VoteGranted = false
				}
				response.Term = r.myCV.CurrentTerm
				r.send(candidateId, response)

			case int:
				timeout := req.(int)
				if timeout == ResendVoteTimeOut {
					rT := msecs * time.Duration(resendTime)
					ResendVoteTimer.Reset(rT)
					reqVoteObj := r.prepRequestVote() //prepare request Vote agn and send to all, ones rcvg the vote agn will vote true agn so won't matter and countVotes func counts no.of true entries
					r.sendToAll(reqVoteObj)
				} else if timeout == ElectionTimeout {
					waitTime_msecs := msecs * time.Duration(waitTime)
					ElectionTimer.Reset(waitTime_msecs)
					break
				}
			}
		}
	}
}

//Keeps sending heartbeats until state changes to follower
func (r *Raft) leader() int {
	r.setNextIndex_All()     //so that new leader sets it map
	r.sendAppendEntriesRPC() //send Heartbeats
	waitTime := 1            //duration between two heartbeats
	waitTime_msecs := msecs * time.Duration(waitTime)
	HeartbeatTimer := r.StartTimer(HeartbeatTimeout, waitTime) //starts the timer and places timeout object on the channel
	waitStepDown := 7
	RetryTimer := r.StartTimer(RetryTimeOut, waitStepDown)
	responseCount := 0
	totalCount := 0
	for {
		req := r.receive() //wait for client append req,extract the msg received on self EventCh
		switch req.(type) {
		case ClientAppendReq:
			//reset the heartbeat timer, now this sendRPC will maintain the authority of the leader
			HeartbeatTimer.Reset(waitTime_msecs)
			request := req.(ClientAppendReq)
			Data := request.Data
			//No check for semantics of cmd before appending to log?
			r.AppendToLog_Leader(Data) //append to self log as byte array
			r.sendAppendEntriesRPC()
			responseCount = 0 //for RetryTimer
		case AppendEntriesResponse:
			response := req.(AppendEntriesResponse)
			responseCount += 1
			if responseCount >= majority-1 { //excluding self
				waitTime_retry := msecs * time.Duration(waitStepDown)
				RetryTimer.Reset(waitTime_retry)
			}
			if !response.IsHeartBeat {
				retVal := r.serviceAppendEntriesResp(response, HeartbeatTimer, waitTime)
				if retVal == follower {
					return follower
				}
			}
		case AppendEntriesReq: // in case some other leader is also in function, it must fall back or remain leader
			request := req.(AppendEntriesReq)
			if request.Term > r.myCV.CurrentTerm {
				r.myCV.CurrentTerm = request.Term //update self Term and step down
				r.myCV.VotedFor = -1              //since Term has increased so VotedFor must be reset to reflect for this Term
				r.WriteCVToDisk()
				return follower //sender server is the latest leader, become follower
			} else {
				//reject the request sending false
				reply := AppendEntriesResponse{r.myCV.CurrentTerm, false, r.Myconfig.Id, false, r.MyMetaData.LastLogIndex}
				r.send(request.LeaderId, reply)
			}

		case RequestVote:
			request := req.(RequestVote)
			totalCount = responseCount + totalCount + 1 //till responses are coming, network is good to go!
			if totalCount >= majority {
				waitTime_retry := msecs * time.Duration(waitStepDown)
				RetryTimer.Reset(waitTime_retry)
			}
			r.serviceRequestVote(request, leader)

		case int: //Time out-time to send Heartbeats!
			timeout := req.(int)
			if timeout == RetryTimeOut { //that means responses are not being received--means partitioned so become follower
				RetryTimer.Stop()
				return follower
			}
			if timeout == HeartbeatTimeout {
				HeartbeatTimer.Reset(waitTime_msecs)
				responseCount = 0 //since new heartbeat is now being sent
				//it depends on nextIndex which is correctly read in prepAE_Req method,since it was AE other than HB(last entry), it would have already modified the nextIndex map
				r.sendAppendEntriesRPC()
			}
		}
	}
}

func (r *Raft) serviceAppendEntriesResp(response AppendEntriesResponse, HeartbeatTimer *time.Timer, waitTime int) int {
	f_Id := response.FollowerId
	lastIndex := response.LastLogIndex
	if lastIndex == -1 { //follower has no log entries
		r.f_specific[f_Id].nextIndex = 0
	} else {
		ack := &r.MyLog[lastIndex].Acks
		if response.Success { //last log entry sent to follower matched and no new leader has come up
			(*ack) += 1 //increase the ack count since follower responded true
			if response.LastLogIndex < r.MyMetaData.LastLogIndex {
				r.f_specific[f_Id].nextIndex = response.LastLogIndex + 1
			}

		} else { //retry if follower rejected the rpc
			//false is sent it means follower is either more latest or its log is stale!
			if response.Term > r.myCV.CurrentTerm { //this means another server is more up to date than itself
				r.myCV.CurrentTerm = response.Term //update self Term with latest leader's Term
				r.myCV.VotedFor = -1               //since Term has increased so VotedFor must be reset to reflect for this Term
				r.WriteCVToDisk()
				return follower
			} else { //Log is stale and needs to be repaired!
				r.LogRepair(response)
			}
		}
		if *ack == majority-1 { //are all positive acks? if not wait for responses--WHY NOT >= ?? CHECK- Because it will advance commitIndex for acks= 3,4,5
			//which is unecessary
			r.advanceCommitIndex(response.Term)
		}
	}
	return -1
}
func (r *Raft) advanceCommitIndex(responseTerm int) {
	//advance CommitIndex only when entry from current Term has been replicated
	if responseTerm == r.myCV.CurrentTerm { //safety property for commiting Entries from older Terms
		prevCommitIndex := r.MyMetaData.CommitIndex
		newCommitIndex := r.MyMetaData.LastLogIndex
		//When commit index advances by more than 1 entry, it should commit(also give for execution to KVStore) all the prev Entries too
		//i.e. check if they are already committed(replicated on majority by checking acks
		for i := prevCommitIndex + 1; i <= newCommitIndex; i++ { //both CIs will always be same in the implementation, i.e. loop will run once only
			if r.MyLog[i].Acks >= majority-1 {
				r.MyMetaData.CommitIndex += 1 //advance CI
				reply := ClientAppendResponse{}
				Data := r.MyLog[i].Cmd                               //last entry of leader's log
				logItem := LogItem{r.CurrentLogEntryCnt, true, Data} //lsn is count started from 0
				r.CurrentLogEntryCnt += 1
				reply.LogEntry = logItem
				r.CommitCh <- &reply.LogEntry
			}
		}

	}
}

//Sends appendRPC requests to all servers and operates according to the responses received i.e. whether to advance the CommitIndex or not
//sent by leader, r is leader
func (r *Raft) sendAppendEntriesRPC() {
	appEntriesObj := r.prepAppendEntriesReq() //prepare AppendEntries object
	appEntriesObjSlice := make([]interface{}, len(appEntriesObj))

	//Copy to new slice created--This is the method to send a []interface to []TypeX
	for i, d := range appEntriesObj {
		appEntriesObjSlice[i] = d
	}
	r.sendToAll_AppendReq(appEntriesObjSlice) //send AppendEntries to all the followers
}

//Appends to self log
//adds new entry, modifies last and prev indexes, Term
func (r *Raft) AppendToLog_Leader(cmd []byte) {
	Term := r.myCV.CurrentTerm
	logVal := LogVal{Term, cmd, 0} //make object for log's value field with acks set to 0
	r.MyLog = append(r.MyLog, logVal)
	//modify metaData after appending
	LastLogIndex := r.MyMetaData.LastLogIndex + 1
	r.MyMetaData.PrevLogIndex = r.MyMetaData.LastLogIndex
	r.MyMetaData.LastLogIndex = LastLogIndex
	if len(r.MyLog) == 1 {
		r.MyMetaData.PrevLogTerm = r.MyMetaData.PrevLogTerm + 1 //as for empty log PrevLogTerm is -2

	} else if len(r.MyLog) > 1 { //explicit check, else would have sufficed too, just to eliminate len=0 possibility
		r.MyMetaData.PrevLogTerm = r.MyLog[r.MyMetaData.PrevLogIndex].Term
	}
	r.setNextIndex_All() //Added-28 march for LogRepair
	r.WriteLogToDisk()

}

//This is called by a follower since it should be able to overwrite for log repairs
func (r *Raft) AppendToLog_Follower(request AppendEntriesReq) {
	Term := request.LeaderLastLogTerm
	cmd := request.Entries
	index := request.PrevLogIndex + 1
	logVal := LogVal{Term, cmd, 0} //make object for log's value field

	if len(r.MyLog) == index {
		r.MyLog = append(r.MyLog, logVal) //when trying to add a new entry
	} else {
		r.MyLog[index] = logVal //overwriting in case of log repair
	}

	r.MyMetaData.LastLogIndex = index
	r.MyMetaData.PrevLogIndex = index - 1
	if index == 0 {
		r.MyMetaData.PrevLogTerm = r.MyMetaData.PrevLogTerm + 1 //or simple -1
	} else if index >= 1 {
		r.MyMetaData.PrevLogTerm = r.MyLog[index-1].Term
	}
	leaderCI := float64(request.LeaderCommitIndex) //Update commit index
	myLI := float64(r.MyMetaData.LastLogIndex)
	if request.LeaderCommitIndex > r.MyMetaData.CommitIndex {
		r.MyMetaData.CommitIndex = int(math.Min(leaderCI, myLI))
	}
	r.WriteLogToDisk()
}

//Modifies the next index in the map and returns
func (r *Raft) LogRepair(response AppendEntriesResponse) {
	Id := response.FollowerId
	failedIndex := r.f_specific[Id].nextIndex
	var nextIndex int
	if failedIndex != 0 {
		if response.LastLogIndex < r.MyMetaData.LastLogIndex { //==CHECK
			nextIndex = response.LastLogIndex + 1
		} else {
			nextIndex = failedIndex - 1 //decrementing follower's nextIndex
			//nextIndex = response.LastLogIndex + 1 //changed on 12 march--failing for some cases --CHECK, doesn't work with for loop in handleClient
		}
	} else { //if nextIndex is 0 means, follower doesn't have first entry (of leader's log),so decrementing should not be done, so retry the same entry again!
		nextIndex = failedIndex
	}
	r.f_specific[Id].nextIndex = nextIndex
	return
}

//modified for candidate call too
func (r *Raft) serviceAppendEntriesReq(request AppendEntriesReq, HeartBeatTimer *time.Timer, waitTime int, state int) int {
	//replicates entry wise , one by one
	becomeFollower := false //for candidate caller only
	waitTime_msecs := msecs * time.Duration(waitTime)
	appEntriesResponse := AppendEntriesResponse{} //make object for responding to leader
	appEntriesResponse.FollowerId = r.Myconfig.Id
	appEntriesResponse.Success = false     //false by default
	appEntriesResponse.IsHeartBeat = false //by default
	var myLastIndexTerm, myLastIndex int
	myLastIndex = r.MyMetaData.LastLogIndex
	if request.Term >= r.myCV.CurrentTerm { //valid leader
		leaderId := request.LeaderId
		r.UpdateLeaderInfo(leaderId) //update leader info
		if request.Term > r.myCV.CurrentTerm {
			r.myCV.CurrentTerm = request.Term //update self Term
			r.myCV.VotedFor = -1              //update votedfor whenever CT is changed
			r.WriteCVToDisk()
		}
		if state == follower {
			HeartBeatTimer.Reset(waitTime_msecs) //reset the timer if this is HB or AE req from valid leader
		}
		if len(r.MyLog) == 0 { //if log is empty
			myLastIndexTerm = -1
		} else {
			myLastIndexTerm = r.MyLog[myLastIndex].Term
		}
		//This is a HB,here log is empty on both sides so Term must not be checked (as leader has incremented its Term due to elections)
		if request.Entries == nil {
			if len(r.MyLog) == 0 { //just to be sure  ===must be satisfied otherwise leader is invalid and logic bug is there.
				appEntriesResponse.Success = true
				appEntriesResponse.IsHeartBeat = true
				becomeFollower = true
			}
		} else { //log has Data so-- for heartbeat, check the index and Term of last entry
			if request.LeaderLastLogIndex == myLastIndex && request.LeaderLastLogTerm == myLastIndexTerm {
				//this is heartbeat as last entry is already present in self log
				appEntriesResponse.Success = true
				appEntriesResponse.IsHeartBeat = true
				r.MyMetaData.CommitIndex = request.LeaderCommitIndex //update the CI for last entry that leader got majority acks for!
				becomeFollower = true
			} else { //this is not a heartbeat but append request
				if request.PrevLogTerm == myLastIndexTerm && request.PrevLogIndex == myLastIndex { //log is consistent except new entry
					becomeFollower = true
					if state == follower { //when caller is follower then only append to log
						r.AppendToLog_Follower(request) //append to log
						appEntriesResponse.Success = true
						appEntriesResponse.IsHeartBeat = false
					}
				}
			}
		}
	}
	appEntriesResponse.Term = r.myCV.CurrentTerm
	appEntriesResponse.LastLogIndex = r.MyMetaData.LastLogIndex
	r.send(request.LeaderId, appEntriesResponse)
	if state == candidate && becomeFollower { //this is candidate call
		return follower
	} else {
		return -1
	}
}

//Services the received request for Vote, added param state for testing
func (r *Raft) serviceRequestVote(request RequestVote, state int) {
	//fmt.Println("In service RV method of ", r.Myconfig.Id)
	response := RequestVoteResponse{}
	candidateId := request.CandidateId
	response.Id = r.Myconfig.Id
	if r.isDeservingCandidate(request) {
		response.VoteGranted = true
		r.myCV.VotedFor = candidateId
		r.myCV.CurrentTerm = request.Term
	} else {
		if request.Term > r.myCV.CurrentTerm {
			r.myCV.CurrentTerm = request.Term
			r.myCV.VotedFor = -1
		}
		response.VoteGranted = false
	}
	if request.Term > r.myCV.CurrentTerm {
		r.WriteCVToDisk()
	}
	response.Term = r.myCV.CurrentTerm
	r.send(candidateId, response) //send to sender using send(sender,response)
}

func (r *Raft) WriteCVToDisk() {

	fh_CV, err1 := os.OpenFile(r.Path_CV, os.O_RDWR|os.O_APPEND, 0600)
	if err1 != nil {
		checkErr("Error in WriteCVToDisk", err1)
		panic(err1)
	}
	cv_encoder := json.NewEncoder(fh_CV)
	cv_encoder.Encode(r.myCV)
	fh_CV.Close()
}

func (r *Raft) WriteLogToDisk() {
	fh_Log, err1 := os.OpenFile(r.Path_Log, os.O_RDWR|os.O_APPEND, 0600)
	if err1 != nil {
		panic(err1)
	}
	log_encoder := json.NewEncoder(fh_Log)
	lastEntry := len(r.MyLog) - 1
	log_encoder.Encode(r.MyLog[lastEntry])
	fh_Log.Close()
}
func (r *Raft) isDeservingCandidate(request RequestVote) bool {
	return ((request.Term > r.myCV.CurrentTerm && r.logAsGoodAsMine(request)) || (request.Term == r.myCV.CurrentTerm && r.logAsGoodAsMine(request) && (r.myCV.VotedFor == -1 || r.myCV.VotedFor == request.CandidateId)))
}

func (r *Raft) logAsGoodAsMine(request RequestVote) bool {
	var myLastLogTerm, myLastLogIndex int
	myLastLogIndex = r.MyMetaData.LastLogIndex
	if len(r.MyLog) == 0 {
		myLastLogTerm = -1
	} else {
		myLastLogTerm = r.MyLog[myLastLogIndex].Term
	}
	return (request.LastLogTerm > myLastLogTerm || (request.LastLogTerm == myLastLogTerm && request.LastLogIndex >= myLastLogIndex))
}

func (r *Raft) resetVotes() {
	for i := 0; i < noOfServers; i++ {
		r.f_specific[i].Vote = false
	}
}

func (r *Raft) UpdateLeaderInfo(leaderId int) {
	r.LeaderConfig.ClientPort = r.ClusterConfigObj.Servers[leaderId].ClientPort
	r.LeaderConfig.Hostname = r.ClusterConfigObj.Servers[leaderId].Hostname
	r.LeaderConfig.Id = leaderId
	r.LeaderConfig.LogPort = r.ClusterConfigObj.Servers[leaderId].LogPort
}

//preparing object for replicating log value at nextIndex, one for each follower depending on nextIndex read from NextIndexMap
func (r *Raft) prepAppendEntriesReq() (appendEntriesReqArray [noOfServers]AppendEntriesReq) {
	for i := 0; i < noOfServers; i++ {
		if i != r.Myconfig.Id {
			LeaderId := r.LeaderConfig.Id
			var Entries []byte
			var Term, PrevLogIndex, PrevLogTerm, LeaderLastLogTerm int
			nextIndex := r.f_specific[i].nextIndex //read the nextIndex to be sent from map
			if nextIndex >= 0 {                    //this is AE request with last entry sent (this will be considered as HB when log of follower is consistent)
				Entries = r.MyLog[nextIndex].Cmd //entry to be replicated
				LeaderLastLogTerm = r.MyLog[nextIndex].Term
				PrevLogIndex = nextIndex - 1 //should be changed to nextIndex-1
				if nextIndex == 0 {
					PrevLogTerm = -1 //since indexing will be log[-1] so it must be set explicitly
				} else {
					PrevLogTerm = r.MyLog[PrevLogIndex].Term //this is the way to get new PrevLogTerm to be sent
				}
			} else { //so this is prepReq for heartbeat for empty log as nextIndex is -1
				//when log is empty indexing to log shouldn't be done hence copy old values
				Entries = nil
				LeaderLastLogTerm = -1
				PrevLogIndex = r.MyMetaData.PrevLogIndex
				PrevLogTerm = r.MyMetaData.PrevLogTerm
			}
			Term = r.myCV.CurrentTerm
			LeaderCommitIndex := r.MyMetaData.CommitIndex
			LeaderLastLogIndex := r.MyMetaData.LastLogIndex
			appendEntriesObj := AppendEntriesReq{Term, LeaderId, PrevLogIndex, PrevLogTerm, Entries, LeaderCommitIndex, LeaderLastLogIndex, LeaderLastLogTerm}
			appendEntriesReqArray[i] = appendEntriesObj

		}

	}
	return appendEntriesReqArray

}

//prepares the object for sending to RequestVoteRPC, requesting the Vote
func (r *Raft) prepRequestVote() RequestVote {
	LastLogIndex := r.MyMetaData.LastLogIndex
	//if this is the request when log is empty
	var lastLogTerm int
	if len(r.MyLog) == 0 {
		lastLogTerm = -1
	} else {
		lastLogTerm = r.MyLog[LastLogIndex].Term
	}
	//fmt.Println("here2")
	reqVoteObj := RequestVote{r.myCV.CurrentTerm, r.Myconfig.Id, LastLogIndex, lastLogTerm}
	return reqVoteObj
}

func (r *Raft) setNextIndex_All() {
	nextIndex := r.MyMetaData.LastLogIndex //given as LastLogIndex+1 in paper..don't know why,seems wrong.
	for k := range r.ClusterConfigObj.Servers {
		if r.Myconfig.Id != k {
			r.f_specific[k].nextIndex = nextIndex
		}
	}
	return
}

// ErrRedirect as an Error object
func (e ErrRedirect) Error() string {
	return string(e)
}

//checks the number of true Votes in follower_specific member
func (r *Raft) countVotes() (VoteCount int) {
	for i := 0; i < noOfServers; i++ { //change it to  range ClusterObj.Servers
		if r.f_specific[i].Vote {
			VoteCount += 1
		}
	}

	return
}

//Starts the timer with appropriate random number secs, also timeout object is needed for distinguishing between different timeouts
func (r *Raft) StartTimer(timeoutObj int, waitTime int) (timerObj *time.Timer) {
	expInSec := time.Duration(waitTime) * msecs
	timerObj = time.AfterFunc(expInSec, func() {
		r.TimeOut(timeoutObj)
	})
	return
}

//Places timeOutObject on the EventCh of the caller
func (r *Raft) TimeOut(timeoutObj int) {
	r.EventCh <- timeoutObj
}

//For testing
func (r *Raft) myId() string {
	return "I am " + strconv.Itoa(r.Myconfig.Id)
}
