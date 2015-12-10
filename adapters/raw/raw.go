package raw

import (
	"bytes"
	"errors"
	log "github.com/cihub/seelog"
	"net"
	"os"
	"reflect"
	"text/template"

	"github.com/gliderlabs/logspout/router"
	"github.com/gliderlabs/logspout/utils"
	"strings"
	"time"
)

func init() {
	router.AdapterFactories.Register(NewRawAdapter, "raw")
}

var address string
var connection net.Conn
var netType string

func NewRawAdapter(route *router.Route) (router.LogAdapter, error) {
	transport, found := router.AdapterTransports.Lookup(route.AdapterTransport("udp"))
	if !found {
		return nil, errors.New("bad transport: " + route.Adapter)
	}
	address = route.Address
	netType = strings.Replace(route.Adapter, "raw+", "", -1)
	conn, err := transport.Dial(route.Address, route.Options)
	connection = conn
	if err != nil {
		return nil, err
	}
	tmplStr := "{{.Data}}\n"
	if os.Getenv("RAW_FORMAT") != "" {
		tmplStr = os.Getenv("RAW_FORMAT")
	}
	tmpl, err := template.New("raw").Parse(tmplStr)
	if err != nil {
		return nil, err
	}
	go connPing()
	return &RawAdapter{
		route: route,
		conn:  conn,
		tmpl:  tmpl,
	}, nil
}

type RawAdapter struct {
	conn  net.Conn
	route *router.Route
	tmpl  *template.Template
}

func (a *RawAdapter) Stream(logstream chan *router.Message) {
	for message := range logstream {
		buf := new(bytes.Buffer)
		err := a.tmpl.Execute(buf, message)
		if err != nil {
			log.Debug("raw:", err)
			return
		}

		if cn := utils.M1[message.Container.Name]; cn != "" {
			t := time.Unix(time.Now().Unix(), 0)
			timestr := t.Format("2006-01-02T15:04:05")
			logmsg := strings.Replace(string(timestr), "\"", "", -1) + " " +
				utils.UserId + " " +
				utils.ClusterId + " " +
				utils.UUID + " " +
				utils.IP + " " +
				utils.Hostname + " " +
				cn + " " +
				buf.String()
			_, err = connection.Write([]byte(logmsg))
			if err != nil {
				log.Error("raw:", err, reflect.TypeOf(a.conn).String())
				/*if reflect.TypeOf(a.conn).String() != "*net.TCPConn"{
					return
				}*/
			}
		}
	}

}

func connPing() {
	timer := time.NewTicker(2 * time.Second)
	log.Debug("ping tcp ......")
	for {
		select {
		case <-timer.C:
			_, err := connection.Write([]byte(""))
			if err != nil {
				if netType == "tcp" {
					conn, err := utils.ConnTCP(address)
					log.Debug("can connection tcp ", err)
					if err == nil {
						connection = conn
					}
				} else if netType == "tls" {
					conn, err := utils.ConnTLS(address)
					log.Debug("can connection tls ", err)
					if err == nil {
						connection = conn
					}
				}
			}
		}
	}
}
