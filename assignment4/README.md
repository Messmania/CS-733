1. src is main folder which uses Exec from test
    Steps to be followed:
    1. cd to src folder and execute following commands
        go install raft
        go install clientCH
        go install serverStarter
        go test
2. ForKvStore is for debugging strings.Split errors. It requires separate launching of servers in terminals.
   Printlns have been commented.
