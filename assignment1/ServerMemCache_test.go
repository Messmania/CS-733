package Server

import (
	"strings"
	"testing"
)

func TestClient(t *testing.T) {
	//t.Parallel()
	go Server()
	status := make(chan string)
	//expected output

	set1 := "set abc 20 10\r\nabcdefjg\r\n"
	/*
	set2 := "set bcd 30 5\r\nefghi\r\n"
	getm1 := "getm bcd\r\n"
	cas1 := "cas bcd 30 7881 20\r\nnewValue\r\n"
	del1 := "delete abc\r\n"
	getm2 := "getm abc\r\n"
	*/

	Eset1 := "OK 8081\r\n"
/*	
	Eset2 := "OK 7887\r\n"
	Egetm1 := "VALUE 7887 30 5\r\nefghi\r\n"
	Ecas1 := "ERRNOTFOUND"
	Edel1 := "DELETED"
	Egetm2 := "ERRNOTFOUND"
*/
	go Client(status, set1, "set1")
/*
	go Client(status, set2, "set2")
	go Client(status, getm1, "getm1")
	go Client(status, cas1, "cas1")
	go Client(status, del1, "del1")
	go Client(status, getm2, "getm2")
*/
	Rset1 := <-status

	if strings.EqualFold(Eset1, Rset1) != true {
		t.Errorf("Received and expected values are:\r\n%v%v", Rset1, Eset1)
	}
/*
	Rset2 := <-status
	if strings.EqualFold(Eset2, Rset2) != true {
		t.Errorf("Received and expected values are:\r\n%v%v", Rset2, Eset2)
	}
	Rgetm1 := <-status
	if strings.EqualFold(Egetm1, Rgetm1) != true {
		t.Errorf("Received and expected values are:\r\n%v%v", Rgetm1, Egetm1)
	}
	Rcas1 := <-status
	if strings.EqualFold(Ecas1, Rcas1) != true {
		t.Errorf("Received and expected values are:\r\n%v%v", Rcas1, Ecas1)
	}
	Rdel1 := <-status
	if strings.EqualFold(Edel1, Rdel1) != true {
		t.Errorf("Received and expected values are:\r\n%v%v", Rdel1, Edel1)
	}
	Rgetm2 := <-status
	if strings.EqualFold(Egetm2, Rgetm2) != true {
		t.Errorf("Received and expected values are:\r\n%v%v", Rgetm2, Egetm2)
	}
*/
}
