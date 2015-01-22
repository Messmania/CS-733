package Server

import (
	"strings"
	"testing"
	//"fmt"
)

//func TestServer(t *testing.T) {
//	a:=Server()

func TestClient(t *testing.T) {
	///invoke client by sending commands--- Do we need to send command to connect to server too? Ideally we should!
	//t.Parallel()
	go Server()
	status := make(chan string)
	//expected output

	Eset1 := "OK 8081\r\n"
	set1 := "set abc 20 10\r\nabcdefjg\r\n"
	go Client(status, set1, "set1")
	Rset1 := <-status
	if strings.EqualFold(Eset1, Rset1) != true {
		t.Errorf("Received and expected values are:\r\n%v%v", Rset1, Eset1)
	}
	Eset2 := "OK 7887\r\n"
	go Client(status, set1, "set1")
	Rset2 := <-status
	if strings.EqualFold(Eset2, Rset2) != true {
		t.Errorf("Received and expected values are:\r\n%v%v", Rset2, Eset2)
	}
	/*
		Egetm1:="VALUE 7887 29 5\r\nefghi\r\n"
		Ecas1:="ERRNOTFOUND"
		Edel1:="DELETED"
		Egetm2:="ERRNOTFOUND"


		set2:="set bcd 30 5\r\nefghi\r\n"
		getm1:="getm abc\r\n"
		cas1:="cas bcd 30 7887 20\r\nnewValue\r\n"
		del1:="delete abc\r\n"
		getm2:="getm bcd\r\n"


		go Client(status,set2,"set2")
		go Client(status,getm1,"getm1")
		go Client(status,cas1,"cas1")
		go Client(status,del1,"del1")
		go Client(status,getm2,"getm2")
		for count:=0;count<7;{
		count++
		<-status
		} */

}
