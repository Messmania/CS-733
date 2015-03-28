package raft

import (
    //"log"
    "math"
    //"math/rand"
    //"strings"
    //"fmt"
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
    Id int // Id of server. Must be unique
    //Hostname   string // name or ip of host
    //ClientPort int    // port at which server listens to client messages.
    //LogPort    int    // tcp port for inter-replica protocol messages.
}

type ClusterConfig struct {
    Path    string         // Directory for persistent log
    Servers []ServerConfig // All servers in this cluster
}

type SharedLog interface {
    // Each data item is wrapped in a LogEntry with a unique
    // lsn. The only error that will be returned is ErrRedirect,
    // to indicate the server id of the leader. Append initiates
    // a local disk write and a broadcast to the other replicas,
    // and returns without waiting for the result.
    Append(data []byte) (LogEntry, error)
}

// Raft implements the SharedLog interface.
type Raft struct {
    ClusterConfigObj   ClusterConfig
    Myconfig           ServerConfig
    LeaderConfig       ServerConfig
    CurrentLogEntryCnt LSN
    commitCh           chan *LogEntry
    //====for assgn3    ========
    //clientCh  chan ClientAppendResponse
    eventCh   chan interface{}
    voteCount int

    //Persisitent state on all servers
    currentTerm int
    votedFor    int
    //myLog       LogDB
    myLog []LogVal
    //Volatile state on all servers
    myMetaData LogMetadata
}

func (r Raft) isThisServerLeader() bool {
    return r.Myconfig.Id == r.LeaderConfig.Id
}

func (r *Raft) Append(data []byte) (LogEntry, error) {
    obj := ClientAppendReq{data}
    send(r.Myconfig.Id, obj)
    response := <-r.commitCh
    return *response, nil
}

type ErrRedirect int

//==========================Addition for assgn3============
//temp for testing
//const layout = "3:04:5 pm (MST)"

//For converting default time unit of ns to secs
var secs time.Duration = time.Duration(math.Pow10(9))

const majority int = 3
const noOfServers int = 5
const (
    ElectionTimeout      = iota
    HeartbeatTimeout     = iota
    AppendEntriesTimeOut = iota
)

//type logDB map[int]LogVal //map is not ordered!! Change this to array or linked list--DONE(changed to array)
//type LogDB []LogVal //log is array of type LogVal
type LogVal struct {
    term int
    cmd  []byte //should it be logEntry type?, should not be as isCommited and lsn are useless to store in log
    //they are needed only for kvStore processing i.e. logEntry must be passed to commitCh

    //This is added for commiting entries from prev term--So when there are entries from prev term in leaders log but have not been replicated on mjority
    //then there must be a flag which is checked before advancing the commitIndex, if false, commit(replicate on maj) them first!
    //used only by leader
    majorityReceived bool
}
type LogMetadata struct {
    lastLogIndex int //last entry in log,corresponding term is term in LogVal, can also be calculated as len(LogDB)-1
    prevLogIndex int // value is lastLogIndex-1 always so not needed as such,---Change this later
    prevLogTerm  int
    //majorityAcksRcvd bool this is same as lastApplied?
    commitIndex int
    //lastApplied int--Not needed

    //Volatile state for leader
    //nextIndex must be maintained for every follower separately,
    // so there must be a mapping from follower to server's nextIndex for it

    // this dis nextIndex[server_id]=nextIndex
    nextIndexMap map[int]int //used during log repair,this is the entry which must be sent for replication
    matchIndex   int
}
type AppendEntriesReq struct {
    term         int
    leaderId     int
    prevLogIndex int
    prevLogTerm  int
    //entries      []LogEntry  //or LogItem?
    //entries      []LogVal  //should it be LogEntry?, shouldn't be as it doesn't have a term, or a wrapper of term and LogEntry?
    entries []byte //leader appends one by one so why need array of logEntry as given in paper
    //LogEntry interface was given by sir, so that the type returned in Append is LogEntry type i.e. its implementator's type
    leaderCommitIndex  int
    leaderLastLogIndex int
}
type AppendEntriesResponse struct {
    term       int
    success    bool
    followerId int
}
type ClientAppendReq struct {
    data []byte
}
type ClientAppendResponse struct {
    logEntry  LogEntry
    ret_error error
}
type RequestVote struct {
    term         int
    candidateId  int
    lastLogIndex int
    lastLogTerm  int
}
type RequestVoteResponse struct {
    term        int
    voteGranted bool
}

func send(serverId int, msg interface{}) { //type of msg? it can be Append,AppendEntries or RequestVote or the responses
    //send to each servers event channel--How to get every1's channel?
    //maintain a shared/global map which has serverid->raftObj mapping so every server can get others raftObj and its eventCh
    raftObj := server_raft_map[serverId]
    raftObj.eventCh <- msg
}

func (r *Raft) receive() interface{} {
    //waits on eventCh
    request := <-r.eventCh
    return request
}

func (r *Raft) sendToAll_AppendReq(msg []interface{}) {
    for k := range server_raft_map {
        if r.Myconfig.Id != k { //send to all except self
            go send(k, msg[k])
        }
    }

}

//This is different method because it sends same object to all followers,
//In above method arg is an array so if that is used, RV obj will have to be replicated unnecessarily
func (r *Raft) sendToAll(msg interface{}) {
    for k := range server_raft_map {
        if r.Myconfig.Id != k { //send to all except self
            go send(k, msg)
        }
    }

}

//Keeps looping and performing follower functions till it timesout and changes to candidate
func (r *Raft) follower(timeout int) int {
    //start heartbeat timer,timeout func wil place HeartbeatTimeout on channel
    waitTime := timeout //use random number after func is tested--PENDING
    HeartBeatTimer := r.StartTimer(HeartbeatTimeout, waitTime)
    for {
        req := r.receive()
        switch req.(type) {
        case AppendEntriesReq:
            request := req.(AppendEntriesReq) //explicit typecasting
            r.serviceAppendEntriesReq(request, HeartBeatTimer, waitTime)
        case RequestVote:
            waitTime_secs := secs * time.Duration(waitTime)
            request := req.(RequestVote)
            HeartBeatTimer.Reset(waitTime_secs)
            r.serviceRequestVote(request)
        case ClientAppendReq: //follower can't handle clients and redirects to leader, sends upto commitCh as well as clientCh
            request := req.(ClientAppendReq) //explicit typecasting
            response := ClientAppendResponse{}
            logItem := LogItem{r.CurrentLogEntryCnt, false, request.data} //lsn is count started from 0
            r.CurrentLogEntryCnt += 1
            response.logEntry = logItem
            r.commitCh <- &response.logEntry
        case int:
            HeartBeatTimer.Stop() //turn off timer as now election timer will start in candidate() mode
            return candidate
        }
    }
}

//conducts election, returns only when state is changed else keeps looping on outer loop(i.e. restarting elections)
func (r *Raft) candidate() int {
    //--start election timer for election-time out time, so when responses stop coming it must restart the election
    waitTime := 12
    ElectionTimer := r.StartTimer(ElectionTimeout, waitTime)
    //This loop is for election process which keeps on going until a leader is elected
    for {
        r.currentTerm = r.currentTerm + 1 //increment current term
        
        r.votedFor, r.voteCount = r.Myconfig.Id, 1 //vote for self
        reqVoteObj := r.prepRequestVote() //prepare request vote obj
        r.sendToAll(reqVoteObj) //send requests for vote to all servers
        //this loop for reading responses from all servers
        for {
            req := r.receive()
            switch req.(type) {
            case RequestVoteResponse: //got the vote response
                response := req.(RequestVoteResponse) //explicit typecasting so that fields of struct can be used
                if response.voteGranted {
                    r.voteCount = r.voteCount + 1
                }
                if r.voteCount >= majority {
                    ElectionTimer.Stop()
                    r.LeaderConfig.Id = r.Myconfig.Id //update leader details
                    return leader                     //become the leader
                }

            case AppendEntriesReq: //received an AE request instead of votes, i.e. some other leader has been elected
                request := req.(AppendEntriesReq)
                //Can be clubbed with serviceAppendEntriesReq with few additions!--SEE LATER
                waitTime_secs := secs * time.Duration(waitTime)
                appEntriesResponse := AppendEntriesResponse{}
                appEntriesResponse.followerId = r.Myconfig.Id
                appEntriesResponse.success = false //false by default, in case of heartbeat or invalid leader
                if request.term >= r.currentTerm { //valid leader
                    r.LeaderConfig.Id = request.leaderId //update leader info
                    ElectionTimer.Reset(waitTime_secs)   //reset the timer
                    myLastIndexTerm := r.myLog[r.myMetaData.lastLogIndex].term
                    if request.leaderLastLogIndex == r.myMetaData.lastLogIndex && request.term == myLastIndexTerm { //this is heartbeat from a valid leader
                        appEntriesResponse.success = true
                    }
                    send(request.leaderId, appEntriesResponse)
                    return follower
                } else {
                    send(request.leaderId, appEntriesResponse)
                }
            case int:
                waitTime_secs := secs * time.Duration(waitTime)
                ElectionTimer.Reset(waitTime_secs)
                break
            }
        }
    }
}

//Keeps sending heartbeats until state changes to follower
func (r *Raft) leader() int {
    r.sendAppendEntriesRPC()                                   //send Heartbeats
    waitTime := 2                                              //duration between two heartbeats
    waitTimeAE := 3                                            //max time to wait for AE_Response
    HeartbeatTimer := r.StartTimer(HeartbeatTimeout, waitTime) //starts the timer and places timeout object on the channel
    var AppendEntriesTimer *time.Timer
    ack := 0
    responseCount := 0
    for {
        req := r.receive() //wait for client append req,extract the msg received on self eventCh
        switch req.(type) {
        case ClientAppendReq:
            HeartbeatTimer.Stop() // request for append will be sent now and will server as Heartbeat too
            request := req.(ClientAppendReq)
            data := request.data
            //No check for semantics of cmd before appending to log?
            r.AppendToLog(r.currentTerm, data) //append to self log as byte array
            r.setNextIndex_All() 
            //reset the heartbeat timer, now this sendRPC will maintain the authority of the leader
            ack = 0                                                             //reset the prev ack values
            r.sendAppendEntriesRPC()                                            //random value as max time for receiving AE_responses
            AppendEntriesTimer = r.StartTimer(AppendEntriesTimeOut, waitTimeAE) //Can be written in HeartBeatTimer too
        case AppendEntriesResponse:
            response := req.(AppendEntriesResponse)
            serverCount := len(r.ClusterConfigObj.Servers)
            responseCount += 1
            if response.success { //log of followers are consistent and no new leader has come up
                ack += 1
            } else { //retry if follower rejected the rpc
                if response.term > r.currentTerm { //this means another server is more up to date than itself
                    r.currentTerm = response.term
                    return follower
                } else { //Log is stale and needs to be repaired!
                    AppendEntriesTimer.Reset(time.Duration(waitTimeAE) * secs)
                    id := response.followerId
                    failedIndex := r.myMetaData.nextIndexMap[id]
                    var nextIndex int
                    if nextIndex != 0 {
                        nextIndex = failedIndex - 1 //decrementing follower's nextIndex

                    } else { //if nextIndex is 0 means, follower doesn't have first entry (of leader's log), so retry the same entry again!
                        nextIndex = failedIndex
                    }
                    //Added--3:38-23 march
                    r.myMetaData.nextIndexMap[id] = nextIndex
                    appEntriesRetry := r.prepAppendEntriesReq()

                    send(id, appEntriesRetry)
                }
            }
            if responseCount >= majority { //convert this nested ifs to && if condition
                if ack == majority { //are all positive acks? if not wait for reponses
                    if response.term == r.currentTerm { //safety property for commiting entries from older terms
                        prevCommitIndex := r.myMetaData.commitIndex
                        currentCommitIndex := r.myMetaData.lastLogIndex
                        r.myMetaData.commitIndex = currentCommitIndex //commitIndex advances,Entries committed!

                        //When commit index advances by more than 1 count, it commits all the prev entries too
                        for i := prevCommitIndex + 1; i <= currentCommitIndex; i++ {
                            reply := ClientAppendResponse{}
                            data := r.myLog[i].cmd                               //last entry of leader's log
                            logItem := LogItem{r.CurrentLogEntryCnt, true, data} //lsn is count started from 0
                            r.CurrentLogEntryCnt += 1
                            reply.logEntry = logItem
                            r.commitCh <- &reply.logEntry
                        }

                    }
                }
                //But what if followers crash? Then it keeps waiting and retries after time out
                //if server crashes this process or func ends or doesn't execute this statement
                if responseCount == serverCount-1 && ack == serverCount { //entry is replicated on all servers! else keep waiting for acks!
                    AppendEntriesTimer.Stop()
                }
            }

        case AppendEntriesReq: // in case some other leader is also in function, it must fall back or remain leader
            request := req.(AppendEntriesReq)
            if request.term > r.currentTerm {
                r.currentTerm = request.term //update self term and step down
                return follower              //sender server is the latest leader, become follower
            } else {
                //reject the request sending false
                reply := AppendEntriesResponse{r.currentTerm, false, r.Myconfig.Id}
                send(request.leaderId, reply)
            }

        case int: //Time out-time to send Heartbeats!
            timeout := req.(int)
            r.sendAppendEntriesRPC() //send Heartbeats
            waitTime_secs := secs * time.Duration(waitTime)
            if timeout == HeartbeatTimeout {
                HeartbeatTimer.Reset(waitTime_secs)
            } else if timeout == AppendEntriesTimeOut {
                AppendEntriesTimer.Reset(waitTime_secs) //start the timer again, does this call the TimeOut method again??-YES
            }
        }
    }
}


func (r *Raft) sendAppendEntriesRPC() {
    appEntriesObj := r.prepAppendEntriesReq() //prepare AppendEntries object

    appEntriesObjSlice := make([]interface{}, len(appEntriesObj))
    //Copy to new slice created
    for i, d := range appEntriesObj {
        appEntriesObjSlice[i] = d
    }
    r.sendToAll_AppendReq(appEntriesObjSlice) //send AppendEntries to all the followers
}

//Appends to self log
//adds new entry, modifies last and prev indexes, term
//how to modify the term fields? n who does--candidate state advances terms
func (r *Raft) AppendToLog(term int, cmd []byte) {
    //r.myMetaData.prevLogIndex = r.myMetaData.lastLogIndex

    logVal := LogVal{term, cmd, false} //make object for log's value field
    //fmt.Println("Before putting in log,", logVal)

    //How to overwrite in slices???---PENDING
    r.myLog = append(r.myLog, logVal)
    //r.myLog[index] = logVal //append to log, when array is dynamic, i.e. slice, indexing can't be used like normal array
    //fmt.Println("I am:", r.Myconfig.Id, "Added cmd to my log")

    //modify metadata after appending
    r.myMetaData.lastLogIndex = r.myMetaData.lastLogIndex + 1
    r.myMetaData.prevLogIndex = r.myMetaData.lastLogIndex
    if len(r.myLog) == 1 {
        r.myMetaData.prevLogTerm = r.myMetaData.prevLogTerm + 1
    } else if len(r.myLog) > 1 {
        r.myMetaData.prevLogTerm = r.myLog[r.myMetaData.prevLogIndex].term
    }
    r.currentTerm = term

}

//Follower receives AEReq and appends to log checking the request objects fields
//also updates leader info after reseting timer or appending to log
func (r *Raft) serviceAppendEntriesReq(request AppendEntriesReq, HeartBeatTimer *time.Timer, waitTime int) {
    //replicates entry wise , one by one
    waitTime_secs := secs * time.Duration(waitTime)

    //===========for testing--reducing timeout of f4 to see if he becomes leader in next term
    //  if r.Myconfig.Id == 4 {
    //      waitTime_secs = 1
    //  }
    //===Testing ends==========
    
    appEntriesResponse := AppendEntriesResponse{} //make object for responding to leader
    appEntriesResponse.followerId = r.Myconfig.Id
    appEntriesResponse.success = false //false by default
    var myLastIndexTerm, myLastIndex int
    myLastIndex = r.myMetaData.lastLogIndex
    if request.term >= r.currentTerm { //valid leader
        r.LeaderConfig.Id = request.leaderId //update leader info
        r.currentTerm = request.term         //update self term
        HeartBeatTimer.Reset(waitTime_secs)  //reset the timer if this is HB or AE req from valid leader
        if len(r.myLog) == 0 {               //if log is empty
            myLastIndexTerm = -1
        } else {
            myLastIndexTerm = r.myLog[r.myMetaData.lastLogIndex].term
        }

        if request.entries == nil && myLastIndex == request.leaderLastLogIndex { //means log is empty on both sides so term must not be checked (as leader has incremented its term)
            appEntriesResponse.success = true
        } else { //log has data so-- for hearbeat, check the index and term of last entry
            if request.leaderLastLogIndex == r.myMetaData.lastLogIndex && request.term == myLastIndexTerm { //this is heartbeat as last entry is already present in self log
                appEntriesResponse.success = true
            } else {
                //this is not a heartbeat but append request
                if request.prevLogTerm == r.currentTerm && request.prevLogIndex == r.myMetaData.lastLogIndex { //log is consistent till now
                    r.AppendToLog(request.term, request.entries) //append to log, also modifies  r.currentTerm
                    appEntriesResponse.success = true
                }
            }
        }
    }
    appEntriesResponse.term = r.currentTerm
    send(request.leaderId, appEntriesResponse)
}

//r is follower who received the request
//Services the received request for vote
func (r *Raft) serviceRequestVote(request RequestVote) {
    //fmt.Println("In service RV method of ", r.Myconfig.Id)
    response := RequestVoteResponse{} //prep response object,for responding back to requester
    candidateId := request.candidateId
    //if haven't voted in this term then only vote!
    //Check if r.currentTerm is lastLogTerm or not in any scenario.. comparison must be with lastLogTerm of self log
    //  if r.currentTerm > request.term { //if candidate is newer vote for it and set VotedFor for this term (if some1 else asks for vote in this term,
    //      response.voteGranted = false
    //  } else {
    if r.votedFor == -1 && (request.lastLogTerm > r.currentTerm || (request.lastLogTerm == r.currentTerm && request.lastLogIndex >= r.myMetaData.lastLogIndex)) {
        response.voteGranted = true
        r.votedFor = candidateId
    } else {

        response.voteGranted = false
    }
    //  }

    //fmt.Println("Follower", r.Myconfig.Id, "voting", response.voteGranted) //"because votefor is", r.votedFor, "my and request terms are:", r.currentTerm, request.term)
    //fmt.Println("Follower", r.Myconfig.Id, "Current term is", r.currentTerm, "Self lastLogIndex is", r.myMetaData.lastLogIndex)
    //fmt.Println("VotedFor,request.lastLogTerm", r.votedFor, request.lastLogTerm)

    response.term = r.currentTerm //to return self's term too
    //fmt.Printf("In serviceRV of %v, obj prep is %v \n", r.Myconfig.Id, response)
    send(candidateId, response) //send to sender using send(sender,response)
}

//preparing object for replicating log value at nextIndex, one for each follower depending on nextIndex read from nextIndexMap
func (r *Raft) prepAppendEntriesReq() (appendEntriesReqArray [noOfServers]AppendEntriesReq) {
       for i := 0; i < noOfServers; i++ {
        //fmt.Println("loop i is:", i, r.Myconfig.Id)
        if i != r.Myconfig.Id {
            nextIndex := r.myMetaData.nextIndexMap[i]
            leaderId := r.LeaderConfig.Id
            var entries []byte
            var term, prevLogIndex, prevLogTerm int
            if len(r.myLog) != 0 {
                term = r.myLog[nextIndex].term
                entries = r.myLog[nextIndex].cmd //entry to be replicated
                prevLogIndex = nextIndex - 1     //should be changed to nextIndex-1
                if len(r.myLog) == 1 {
                    prevLogTerm = r.myMetaData.prevLogTerm + 1 //should be changed to term corresponding to nextIndex-1 entry
                } else {
                    prevLogTerm = r.myLog[prevLogIndex].term
                }
            } else {
                //when log is empty indexing to log shouldn't be done hence copy old values
                term = r.currentTerm
                entries = nil
                prevLogIndex = r.myMetaData.prevLogIndex
                prevLogTerm = r.myMetaData.prevLogTerm
            }

            leaderCommitIndex := r.myMetaData.commitIndex
            leaderLastLogIndex := r.myMetaData.lastLogIndex
            appendEntriesObj := AppendEntriesReq{term, leaderId, prevLogIndex, prevLogTerm, entries, leaderCommitIndex, leaderLastLogIndex}
            appendEntriesReqArray[i] = appendEntriesObj
        }

    }
    return appendEntriesReqArray

}

//prepares the object for sending to RequestVoteRPC, requesting the vote
func (r *Raft) prepRequestVote() RequestVote {
    lastLogIndex := r.myMetaData.lastLogIndex
    var lastLogTerm int
    if len(r.myLog) == 0 {
        //fmt.Println("In if of prepRV()")
        //lastLogTerm = -1 //Just for now--Modify later
        lastLogTerm = r.currentTerm
    } else {
        //fmt.Println("In else of prepRV()")
        lastLogTerm = r.myLog[lastLogIndex].term
    }
    //fmt.Println("here2")
    reqVoteObj := RequestVote{r.currentTerm, r.Myconfig.Id, lastLogIndex, lastLogTerm}
    return reqVoteObj
}

//Starts the timer with appropriate random number secs
func (r *Raft) StartTimer(timeoutObj int, waitTime int) (timerObj *time.Timer) {
    //waitTime := rand.Intn(10)
    //for testing
    //waitTime := 5
    expInSec := secs * time.Duration(waitTime) //gives in seconds
    //fmt.Printf("Expiry time of %v is:%v \n", timeoutObj, expInSec)
    timerObj = time.AfterFunc(expInSec, func() {
        r.TimeOut(timeoutObj)
    })
    return
}

//Places timeOutObject on the eventCh of the caller
func (r *Raft) TimeOut(timeoutObj int) {
    r.eventCh <- timeoutObj
}

func (r *Raft) setNextIndex_All() {
    nextIndex := r.myMetaData.lastLogIndex //given as lastLogIndex+1 in paper..don't know why,seems wrong.
    for k := range server_raft_map {
        if r.Myconfig.Id != k {
            r.myMetaData.nextIndexMap[k] = nextIndex
        }
    }
    return
}

// ErrRedirect as an Error object
func (e ErrRedirect) Error() string {
    return string(e)
}
