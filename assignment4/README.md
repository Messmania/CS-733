# Steps to install
	1. Set GOPATH to parent of the required src folder
	2. This project uses Exec from test method
		    1. Steps to be followed: cd to src folder and execute following commands
	            1. go install raft
	            2. go install clientCH
	            3. go install serverStarter
	            4. go test

# Flow :
	1. System has 5 servers, when they start, they look for their persistent state on Disk which is stored in folder DiskLog
	2. If folder doesn't exist then it is created for each server on their start.
	3. If folders exist, which means server crashed and is now resuming , then state is read from the DiskLog and database is updated
	4. All servers start by creating/reading their state from disk.
	5. Follower timeouts are set as [10 2 7 9 13] for S0,1,2,3,4 resp. so that testing can be done.

# Leader election: 
	1. All servers start off as followers and then the one with least timeout becomes candidate and starts the election.
	2. A leader is elected during the election. 
	3. When the leader is elected, then it sends Heartbeats to all periodically (which is set to 50 msec for this) to maintain its authority.
	4. If any follower doesn't receive Heartbeat, it times out and starts the fresh election.
	5. With no log entries, any server is a valid leader.
	6. If leader crashes, then new leader is elected by comparing the current term and log length.

# Client requests:
	1. Client sends append requests to leader to which leader responds only after it is replicated to majority of the followers.
	2. Client requests to followers get a redirect message giving the PORT NUMBER of current leader.

# Log Repair:
	1. Log repair is done in Heartbeats.
	2. Leader sends its last entry along with prev entry to all as Heartbeat.
	3. When follower have prev entry in their log, it means their log is consistent till now and new entry (i.e. leader's last entry) is appended to log.
	4. When a follower does not have prev entry , it means its log is stale and needs repair. So follower sends back false along with its last index.
	5. When leader receives false and compares the last index received, it decrements the nextIndex by 1 for this follower.
	6. Now in next Heartbeat, prev entry and entry before that will be sent to this follower and this will keep repeating till log is repaired.
	
# Commit Index:
	1. Commit index is advanced only when majority acks for current entry is received.
	
# Encoding:
	1. Json encoding is used to write persistent state to files.
	2. Gob encoding is used for exchange of requests and responses between servers.
	3. Byte encoding is used for exchange of messages between servers and clients.
	
# Communication:
	1. Each server communicates with each other as well as clients using conn objects.
	2. Each servers makes two connections, one for receiving other for sending.
	3. Each server has a map for storing conn for destination servers.Therefore when sending the message to servers, conn is read from map, if nil, new connection is made with destination server and conn is stored in the map.
	4. Whenever a server closes the connection , corresponding conn is set as nil in other end of the pipe, so encoding doesn't give error.	
	
# Test Cases:
	1. Test_SCA : Tests a single entry append to current leader which tests the commit of entry from current term too.
	2. Test_MCA : Tests concurrency to leader by firing 5 commmands in parallel.
	3. Test_CA_Followers: Tests the redirection message from followers.
	4. Test_ServerCrash: Crashes the current leader to detect leader changes in following tests.
	5. Test_LeaderChanges : Since S1 is crashed, now next eligible leader is 2. So only 1 append to 2 for checking this is current leader.
	6. Test_NewLeaderResponses : MCA for new leader also since S1 is crashed so its log will become stale.
	7. TestErrors : this tests the kvStore functionality for wrong commands.
	8. TestMRSC : Multiple requests Single Client, this tests multiple append request by single client.
	9. TestMRMC : Multiple Requests Multiple Clients, this tests multiple appends by each of the many clients.
	10. TestCheckAndExpire: This tests data expire function of the kvStore.
	11. Test_KillAll : Kills all servers after all tests pass successfully.

	
