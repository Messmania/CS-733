/* Trying to make an echo server
 */

package Server

//package main
import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var secs time.Duration = time.Duration(math.Pow10(9))
var m = make(map[string]Data)
var globMutex = &sync.Mutex{}

type Data struct {
	value    string
	version  int64
	numbytes int64
	setTime  int64
	expiry   int64
	dbMutex  *sync.Mutex
}

func Server() {
	//fmt.Println("Entering server\n")
	service := ":9000"
	tcpaddr, err := net.ResolveTCPAddr("tcp", service) // Try eliminating this later, it is not needed
	checkErr(err)
	//fmt.Println("Resolve done\n")
	listener, err := net.ListenTCP("tcp", tcpaddr)
	checkErr(err)
	//fmt.Println("Listen done\n")
	for {
		//fmt.Println("Entering for\n")
		conn, err := listener.Accept()
		if err != nil {
			//fmt.Println("Accept failed\n")
			continue
		}
		//fmt.Println("Calling handleClient\n")
		go handleClient(conn, m)
		//conn.Close()
	}
}

func handleClient(conn net.Conn, m map[string]Data) {
	//fmt.Println("In handleClient\n")
	var buf [512]byte
	n, err := conn.Read(buf[0:])
	if err != nil {
		fmt.Println("In read error\n")
		return
	}
	sr := ""
	/*Convert command read from conn to array of strings then read them separately	*/
	str := string(buf[0:n])
	line := strings.Split(str, "\r\n")
	cmd := strings.Fields(line[0])
	value := line[1] //TO DO--Check if 2nd line has wrong no.of args or ignore
	l := len(cmd)
	key := cmd[1]
	op := strings.ToLower(cmd[0])
	switch op {
	case "set":
		// do argument count check
		if l == 4 {
			exp, _ := strconv.ParseInt(cmd[2], 0, 64)
			numb, _ := strconv.ParseInt(cmd[3], 0, 64)
			ver := int64(rand.Intn(10000))
			//fmt.Println("ver and randno. is", ver,rand.Intn(10000))
			//d,exist :=m[key]
			//if exist==false {
			globMutex.Lock()
			m[key] = Data{value, ver, numb, time.Now().Unix(), exp, &sync.Mutex{}}
			d := m[key]
			if exp > 0 {
				expInSec := secs * time.Duration(exp)
				time.AfterFunc(expInSec, func() {
					checkAndExpire(m, key, exp, d.setTime)
				})
				//fmt.Printf("d.version type and value: %T %T %v\n", ver,d.version,d.version)
				globMutex.Unlock()
				//}
			} /*else {																		//update the values: NO NEED as set will overwrite
			//Use dbChannel for this key
				d.dbMutex.Lock()
				d.value = value
				d.expiry = exp
				d.numbytes = numb
				if exp>0 {
					expInSec :=secs*time.Duration(exp)
					time.AfterFunc(expInSec,func(){
						checkAndExpire(m,key,exp,d.setTime)
					})
				}
			d.dbMutex.Unlock()
			}	*/
			//fmt.Println("before sr, d.version is", d.version)
			sr = "OK " + strconv.FormatInt(d.version, 10) + "\r\n"
		} else if l == 5 { //noreply
			sr = ""
		} else {
			sr = "ERR_CMD_ERR \r\n" //wrong command line format
		}
	case "get":
		if l == 2 {
			//do get processing
			d, exist := m[key]
			if exist != false {
				d.dbMutex.Lock()
				numStr := strconv.FormatInt(d.numbytes, 10)
				sr = "VALUE " + numStr + "\r\n" + d.value + "\r\n"
				d.dbMutex.Unlock()
			} else {
				sr = "ERRNOTFOUND\r\n"
			}
		} else {
			sr = "ERR_CMD_ERR \r\n"
		}
	case "getm":
		if l == 2 {
			//do getm processing
			d, exist := m[key]
			if exist != false {
				d.dbMutex.Lock()
				remExp := d.expiry - (time.Now().Unix() - d.setTime)
				verStr := strconv.FormatInt(d.version, 10)
				sr = "VALUE " + verStr + " " + strconv.FormatInt(remExp, 10) + " " + strconv.FormatInt(d.numbytes, 10) + "\r\n" + d.value + "\r\n"
				d.dbMutex.Unlock()
			} else {
				sr = "ERRNOTFOUND\r\n"
			}
		} else {
			sr = "ERR_CMD_ERR \r\n"
		}
	case "cas":
		if l == 5 {
			d, exist := m[key]
			if exist != false {
				//d.dbChan<-"start"
				d.dbMutex.Lock()
				oldVersion := strconv.FormatInt(d.version, 10)
				newVersion := cmd[3]
				numbytes := cmd[4]
				if newVersion == oldVersion {
					//Replace the value as old and new version are same
					newVFloat, _ := strconv.ParseInt(newVersion, 10, 64) //check their errors as well!!
					numBInt, _ := strconv.ParseInt(numbytes, 10, 64)
					exp, _ := strconv.ParseInt(cmd[2], 0, 64)
					m[key] = Data{value, newVFloat, numBInt, time.Now().Unix(), exp, &sync.Mutex{}}
					d.dbMutex.Unlock()
					sr = "OK " + newVersion + "\r\n"
				} else {
					sr = "ERR_VERSION\r\n"
				}
			} else {
				sr = "ERRNOTFOUND\r\n"
			}
		} else if l == 6 && cmd[5] == "noreply" {
			sr = ""
		} else {
			sr = "ERR_CMD_ERR \r\n"
		}
	case "delete":
		//fmt.Println("Inside delete")
		d, exist := m[key]
		if exist != false {
			d.dbMutex.Lock()
			delete(m, key)
			d.dbMutex.Unlock()
			sr = "DELETED\r\n"
		} else {
			sr = "ERRNOTFOUND\r\n"
		}
	default:
		sr = "ERRINTERNAL\r\n"
	}
	//fmt.Println("at the end of handleClient\n")

	_, err2 := conn.Write([]byte(sr))
	if err2 != nil {
		return
	}
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func checkAndExpire(m map[string]Data, key string, oldExp int64, setTime int64) {
	d, err := m[key]
	absOldExp := setTime + oldExp
	absNewExp := setTime + d.expiry
	if err != false {
		return
	}
	if absOldExp != absNewExp {
		return
	} else {
		delete(m, key)
		return
	}

}

func Client(ch chan string, strEcho string, c string) (reply string) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":9000")
	checkErr(err)
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	checkErr(err)

	conn.Write([]byte(strEcho)) //Writing the message to connection stream which server can read
	var rep [512]byte
	n, err1 := conn.Read(rep[0:])
	checkErr(err1)
	reply = string(rep[0:n])
	//fmt.Printf("Reply of %v is: %v \n",c,reply[0:n])
	conn.Close()
	ch <- reply
	return
}
