1. src is main folder which uses Exec from test
    -> Steps to be followed:
   i. cd to src folder and execute following commands
        a. go install raft
        b. go install clientCH
        c. go install serverStarter
        d. go test
2. ForKvStore is for debugging strings.Split errors. It requires separate launching of servers in terminals.
   Printlns have been commented.
