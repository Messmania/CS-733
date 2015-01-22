package Server

import (
	"strings"
	"testing"
)

func TestClient(t *testing.T) {
	go Server()
	status := make(chan string)
	set1 := "set abc 20 10\r\nabcdefjg\r\n"
	set2 := "set bcd 30 5\r\nefghi\r\n"
	getm1 := "getm abc\r\n"
	cas1 := "cas bcd 30 7887 20\r\nnewValue\r\n"
	del1 := "delete bcd\r\n"
	getm2 := "getm bcd\r\n"
	
	Eset1 := "OK 8081\r\n"
	Eset2 := "OK 7887\r\n"
	Egetm1 := "VALUE 8081 20 10\r\nabcdefjg\r\n"
	Ecas1 := "OK 7887\r\n"
	Edel1 := "DELETED\r\n"
	Egetm2 := "ERRNOTFOUND\r\n"

	
	go Client(status, set1, "set1")
	Rset1 := <-status
	if strings.EqualFold(Eset1, Rset1) != true {
		t.Error("Expected and Received values are:\r\n",Eset1,Rset1)
	}

	go Client(status, set2, "set2") 
	Rset2 := <-status
	if strings.EqualFold(Eset2, Rset2) != true {
		t.Error("Expected and Received values are:\r\n",Eset2, Rset2)
	}

	go Client(status, getm1, "getm1")
	Rgetm1 := <-status
	if strings.EqualFold(Egetm1, Rgetm1) != true {
		t.Error("Expected and Received values are:\r\n", Egetm1, Rgetm1)
	}
	
	go Client(status, cas1, "cas1")
	Rcas1 := <-status
	if strings.EqualFold(Ecas1, Rcas1) != true {
		t.Error("Expected and Received values are:\r\n", Ecas1, Rcas1)
	}
	
	go Client(status, del1, "del1")
	Rdel1 := <-status
	if strings.EqualFold(Edel1, Rdel1) != true {
		t.Error("Expected and Received values are:\r\n", Edel1,Rdel1)
	}
	
	go Client(status, getm2, "getm2")
	Rgetm2 := <-status
	if strings.EqualFold(Egetm2, Rgetm2) != true {
		t.Error("Expected and Received values are:\r\n", Egetm2, Rgetm2)
	}
}
