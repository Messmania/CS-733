package Server

import (
	"strings"
	"testing"
	//"fmt"
	"time"
)

func TestServer(t *testing.T){
	go Server()
}


//PASSED
//Checking server's affirmative responses
func TestResponse(t *testing.T){
	t.Parallel()
	set1 := "set abc 20 10\r\nabcdefjg\r\n"
	set2 := "set bcd 30 5\r\nefghi\r\n"
	getm1 := "getm abc\r\n"
	cas1 := "cas bcd 30 7887 20\r\nnewValue\r\n"
	get2 := "get bcd\r\n"
	del1 := "delete bcd\r\n"
	
	
	Eset1 := "OK 8081\r\n"
	Eset2 := "OK 7887\r\n"
	Egetm1 := "VALUE 8081 20 10\r\nabcdefjg\r\n"
	Ecas1 := "OK 7887\r\n"	
	Eget2 := "VALUE 20\r\nnewValue\r\n"
	Edel1 := "DELETED\r\n"
	E:=[]string{Eset1,Eset2,Egetm1,Ecas1,Eget2,Edel1}

	cmd:=[]string{set1,set2,getm1,cas1,get2,del1}
	cd:=[]string{"set1","set2","getm1","cas1","get2","del1"}
	chann:=make([]chan string,5)
	
	//for initialing the array otherwise it is becoming nil
	for k:=0;k<5;k++ {
		chann[k]=make(chan string)
	}
	for i:=0;i<5;i++ {
		go Client(chann[i],cmd[i],cd[i])
	}
	
	var R [5]string
	for j:=0;j<5;j++ {
		R[j]=<-chann[j]
		if R[j]!=E[j] {			//strings.EqualFold is failing for same strings..--check later about utf-8 coding
			t.Error("Expected and Received values are:\r\n",E[j],R[j])
		}
	}

}

//PASSED =======but race conditions coming
//To check expiration and remaining exp time in getm
func TestExpiry(t *testing.T){
	t.Parallel()
	chann:=make([]chan string,3)
	
	//for initialing the array otherwise it is becoming nil
	for k:=0;k<3;k++ {
		chann[k]=make(chan string)
	}
	
	//Commands
	set1 := "set abc 2 10\r\nabcdefjg\r\n"
	getm1 := "getm abc\r\n"

	//Expected values
	Eset1 := "OK"
	Egetm1 := "VALUE 1 10\r\nabcdefjg\r\n"
	Egetm2 := "ERRNOTFOUND\r\n"
	E:=[]string{Eset1,Egetm1,Egetm2}

	go Client(chann[0],set1,"set1")
	time.Sleep(time.Second*1)
	go Client(chann[1],getm1,"getm1")
	time.Sleep(time.Second*1)
	go Client(chann[2],getm1,"getm2")

	var R [3]string

	//Excluding hard coded version number	
	Rset1 :=strings.Fields(<-chann[0])
	R[0]= Rset1[0]
	
	RLine:= strings.Split(<-chann[1],"\r\n")
	Rgetm1:=strings.Fields(RLine[0])
	R[1]=Rgetm1[0]+" "+Rgetm1[2]+" "+Rgetm1[3]+"\r\n"+RLine[1]+"\r\n"
	
	R[2]=<-chann[2]
	
	
	for j:=0;j<3;j++ {
		if R[j]!=E[j] {			
			t.Error("Expected and Received values are:\r\n",E[j],R[j])
		}
	}
	
}


//PASSED
func TestErrors(t *testing.T){
	t.Parallel()
	const n int=10
	//=============For ERRNOTFOUND==========
	get1:="get ms\r\n"
	getm1:="getm ms\r\n"
	cas1:="cas ms 2 2204 10\r\nmonikasaha\r\n"
	del1:="delete ms\r\n"
	

	//============For ERR_CMD_ERR===========
	set1:="SET ms 5 11 extra\r\nmonikasahai\r\n"
	cas2:="cas a\r\n"
	get2:="get ms ok\r\n"
	
	//===========For ERR_VERSION============
	set2:="set ms 2 11\r\nmonikasahai\r\n"
	cas3:="cas ms 2 1092 10\r\nnewMonika\r\n"
	
	//==========For ERR_INTERNAL============ Not correct logically
	msg1:="del ms"


	cmd:=[]string{get1,getm1,cas1,del1,get1,set1,cas2,get2,set2,cas3,msg1}
	cd:=[]string{"get1","getm1","cas1","del1","get1","set1","cas2","get2","set2","cas3","msg1"}
	err0:="ERRNOTFOUND\r\n"
	err1:="ERR_CMD_ERR\r\n"
	err2:="ERR_VERSION\r\n"
	Rset1:="OK"	
	err3:="ERR_INTERNAL\r\n"
	E:=[]string{err0,err0,err0,err0,err0,err1,err1,err1,Rset1,err2,err3}

	//Declare and initialize channels
	chann:=make([]chan string,n)
	for k:=0;k<n;k++ {
		chann[k]=make(chan string)
	}
	// Launch clients
	for i:=0;i<n;i++ {
		go Client(chann[i],cmd[i],cd[i])
	}
	var R [n]string
	for j:=0;j<n;j++ {
		R[j]=<-chann[j]
		if j==8 {
			r:=strings.Fields(R[j])
			R[j]=r[0]
		}				
		if R[j]!=E[j] {			
			t.Error("Expected and Received values are:\r\n",E[j],R[j])
		}
	}
	
}

//PASSED
//Checking multiple commands send by single user
func TestMultipleCmds(t *testing.T){
	t.Parallel()
	chann:=make(chan string)
	const n int=4
	cmd1:="set abc 10 10\r\ndata1\r\ngetm abc\r\ndelete abc\r\ngetm abc\r\n"
	E:=[]string{"OK","VALUE 10 10\r\ndata1\r\n","DELETED","ERRNOTFOUND"}
	//E:=[]string{"OK","VALUE 10 10\r\n"}
	go Client(chann,cmd1,"cmd1")
	for i:=0;i<n;i++ {
		R:=<-chann
		Rline:=strings.Split(R,"\r\n")
		r:=strings.Fields(Rline[0])
		if i==0 {		
			R=r[0]
		}else if i==1 {
			R=r[0]+" "+r[2]+" "+r[3]+"\r\n"+Rline[1]+"\r\n"
		}else{
			R=Rline[0]
		}
		if R!=E[i] {			
			t.Errorf("Expected and Received values are:\r\n%v\n%v",E,R)
		}
	}

}


/*
func TestNoReply(t *testing.T){
	chann := make(chan string)
	cmd := "set mnp 10 10 noreply\r\nabcdefjg\r\nget mnp"
	go Client(chann,cmd,"cmd")
	Eset3:="OK"
	fmt.Println("before reading")
	Rset3 := <-chann
	fmt.Println("After reading")
	r:=strings.Fields(Rset3)
	Rset3=r[0]
	if strings.EqualFold(Eset3, Rset3) != true {
		t.Error("Expected and Received values are:\r\n", Eset3, Rset3)
	}
}*/




//Problem: Order of goroutines is not known so rand.Intn output is varying--No need to check version
//to test noreply
//to test concurrency
//to test numbytes
// to test all the ERR replies--DONE
//to test check n expire deletion--DONE
// to test expiry left in getm--DONE
// when shud ERR_INTERNAL come?

