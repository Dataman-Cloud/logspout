package raw

import (
	"bytes"
	"errors"
	"github.com/Jeffail/gabs"
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
	msg := gabs.New()
	for message := range logstream {
		buf := new(bytes.Buffer)
		err := a.tmpl.Execute(buf, message)
		if err != nil {
			log.Debug("raw:", err)
			return
		}

		if cn := utils.M1[message.Container.Name]; cn != "" {
			//logmsg := utils.SendMessage(cn, buf.String(), message.Container)
			//_, err = connection.Write([]byte(logmsg))
			utils.SendMessage(cn, buf.String(), message.Container.ID, msg)
			_, err = connection.Write(append([]byte(msg.String()), '\n'))
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
