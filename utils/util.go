package utils

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/Jeffail/gabs"
	log "github.com/cihub/seelog"
	"github.com/fsouza/go-dockerclient"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	countFile = "/tmp/logspout/logspout.json"
)

var (
	UUID        string
	M1          map[string]string
	IP          string
	Hostname    string
	UserId      string
	ClusterId   string
	counter     map[string]int64
	counterlock sync.Mutex
	mesoslock   sync.Mutex
)

func getPort(ports string) string {
	reg := regexp.MustCompile("\\[|\\]")
	strs := strings.Split(reg.ReplaceAllString(ports, ""), "-")
	startPort, serr := strconv.Atoi(strs[0])
	endPort, eerr := strconv.Atoi(strs[1])
	if serr != nil || eerr != nil {
		return "[]"
	}
	var port []string
	for i := startPort; i <= endPort; i++ {
		port = append(port, strconv.Itoa(i))
	}
	return "[" + strings.Join(port, ",") + "]"

}

type Message struct {
	FrameWorks []struct {
		Executors []struct {
			Container string `json:"container"`
			Tasks     []struct {
				SlaveId   string `json:"slave_id"`
				State     string `json:"state"`
				Name      string `json:"name"`
				Id        string `json:"id"`
				Resources struct {
					Ports string `json:"ports"`
				} `json:"resources"`
			} `json:"tasks"`
		} `json:"executors"`
	} `json:"frameworks"`
	HostName string `json:"hostname"`
}

func init() {
	counterlock, mesoslock = sync.Mutex{}, sync.Mutex{}
	M1 = make(map[string]string)
	loadCounter()
	UUID = os.Getenv("HOST_ID")
	if UUID == "" {
		log.Error("cat't found uuid")
		os.Exit(0)
	}
	UserId = os.Getenv("USER_ID")
	if UserId == "" {
		log.Error("cat't found userid")
		os.Exit(0)
	}
	ClusterId = os.Getenv("CLUSTER_ID")
	if ClusterId == "" {
		log.Error("cat't found clusterid")
		os.Exit(0)
	}
	Hostname, _ = os.Hostname()
	IP, _ = GetIp()
	M1 = getCnames()
}

func Run() {
	timer := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-timer.C:
			//getMesosInfo(mesoslock)
			persistenCounter()
		}
	}
}

func getMesosInfo(lock *sync.Mutex) {
	data, err := HttpGet("http://" + IP + ":5051/slave(1)/state.json")
	if err == nil {
		mg := getCnames()
		var m Message
		json.Unmarshal([]byte(data), &m)
		if len(m.FrameWorks) > 0 {
			for _, fw := range m.FrameWorks {
				if len(fw.Executors) > 0 {
					for _, ex := range fw.Executors {
						if len(ex.Tasks) > 0 {
							for _, ts := range ex.Tasks {
								mcn := "/mesos-" + ts.SlaveId + "." + ex.Container
								mg[mcn] = ts.Name + " " +
									ts.Id + " " +
									getPort(ts.Resources.Ports)
							}
						}
					}
				}
			}
		}
		lock.Lock()
		M1 = mg
		lock.Unlock()
	}
	//log.Debug("get mesos json: ", err, M1)
}

func GetIp() (ip string, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}

func ConnTCP(address string) (net.Conn, error) {
	raddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTCP("tcp", nil, raddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func ConnTLS(address string) (net.Conn, error) {
	cert, err := tls.LoadX509KeyPair("/root/ssl/client.pem", "/root/ssl/client.key")
	if err != nil {
		return nil, err
	}
	config := tls.Config{Certificates: []tls.Certificate{cert}, InsecureSkipVerify: true}
	conn, err := tls.Dial("tcp", address, &config)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func getCnames() map[string]string {
	cnames := os.Getenv("CNAMES")
	if cnames != "" {
		cmap := make(map[string]string)
		ca := strings.Split(cnames, ",")
		for _, cname := range ca {
			rname := strings.Replace(cname, "/", "", 1)
			cmap[cname] = rname + " " + rname + " " + "[]"
		}
		return cmap
	}
	return make(map[string]string)
}

/*func SendMessage(cn, msg string, d *docker.Container) string {
	counter[ShortID(d.ID)]++
	t := time.Now()
	timestr := t.Format(time.RFC3339Nano)
	logmsg := timestr + " " +
		UserId + " " +
		fmt.Sprint(counter[ShortID(d.ID)]) + " " +
		ClusterId + " " +
		UUID + " " +
		IP + " " +
		Hostname + " " +
		cn + " " +
		msg
	return logmsg
}*/

func SendMessage(cn, msg, cid string, container *gabs.Container) {
	counter[cid]++
	t := time.Now()
	timestr := t.Format(time.RFC3339Nano)
	container.Set(timestr, "timestamp")
	container.Set(UserId, "userid")
	container.Set(counter[cid], "counter")
	container.Set(ClusterId, "clusterid")
	container.Set(UUID, "uuid")
	container.Set(IP, "ip")
	container.Set(Hostname, "hostname")
	ars := strings.Split(cn, " ")
	if len(ars) == 3 {
		container.Set(ars[0], "typename")
		container.Set(ars[1], "taskid")
		container.Set(ars[2], "ports")
		container.Set(msg, "msg")
	} else {
		container.Set("", "msg")
	}
}

func loadCounter() {
	counter = make(map[string]int64)
	buf, err := ioutil.ReadFile(countFile)
	if err == nil {
		json, err := gabs.ParseJSON(buf)
		if err == nil {
			m, err := json.ChildrenMap()
			if err == nil {
				for k, v := range m {
					if reflect.TypeOf(v.Data()).String() == "float64" {
						counter[ShortID(k)] = int64(v.Data().(float64))
					}
				}
				log.Debug("load container counter: ", json)
			} else {
				log.Error("counter childrenmap err: ", err)
			}
		} else {
			log.Error("counter str to json err: ", err)
		}
	} else {
		log.Debug("not found counter file: ", err)
	}
}

func persistenCounter() {
	if len(counter) > 0 {
		json := gabs.New()
		counterlock.Lock()
		for k, v := range counter {
			json.Set(v, k)
		}
		counterlock.Unlock()
		err := ioutil.WriteFile(countFile, json.Bytes(), 0x644)
		if err != nil {
			log.Error("persisten counter failed: ", err)
		}
	}
}

func DeleteCounter(client *docker.Client) {
	containers, err := client.ListContainers(docker.ListContainersOptions{})
	if err == nil {
		copycounter := counter
		for _, container := range containers {
			_, ok := copycounter[ShortID(container.ID)]
			if ok {
				delete(copycounter, container.ID)
			}
		}

		counterlock.Lock()
		for k, _ := range copycounter {
			delete(counter, k)
		}
		counterlock.Unlock()
	}
}

func DeleteM1(cname string) {
	if _, ok := getCnames()[cname]; !ok {
		mesoslock.Lock()
		delete(M1, cname)
		mesoslock.Unlock()
	}
}

func ReceiveContainer(container *docker.Container) {
	env := container.Config.Env
	appName := ""
	taskId := ""
	ports := ""
	if len(env) > 0 {
		for _, e := range env {
			if strings.Contains(e, "MARATHON_APP_ID=/") {
				appName = strings.Split(e, "=/")[1]
			} else if strings.Contains(e, "MESOS_TASK_ID=") {
				taskId = strings.Split(e, "=")[1]
			} else if strings.Contains(e, "PORTS=") {
				ports = strings.Split(e, "=")[1]
			}
		}
		if appName != "" && taskId != "" && ports != "" {
			mesoslock.Lock()
			M1[container.Name] = appName + " " + taskId + " " + "[" + ports + "]"
			mesoslock.Unlock()
		}
	}
}

func ShortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
