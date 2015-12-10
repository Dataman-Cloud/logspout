package kafkaraw

import (
	"bytes"
	"errors"
	log "github.com/cihub/seelog"
	"os"
	"text/template"

	kafka "github.com/Shopify/sarama"
	"github.com/gliderlabs/logspout/router"
	"github.com/gliderlabs/logspout/utils"
)

func init() {
	router.AdapterFactories.Register(NewKafkaRawAdapter, "kafkaraw")
}

var topic string

func NewKafkaRawAdapter(route *router.Route) (router.LogAdapter, error) {
	topic = os.Getenv("TOPIC")
	compressType := os.Getenv("COMPRESS_TYPE")
	if topic == "" {
		err := errors.New("not found kafka topic")
		return nil, err
	}
	transport, found := router.AdapterTransports.Lookup(route.AdapterTransport("udp"))
	if !found {
		return nil, errors.New("bad transport: " + route.Adapter)
	}
	_ = transport
	config := kafka.NewConfig()
	if compressType == "gzip" {
		config.Producer.Compression = kafka.CompressionGZIP
	} else if compressType == "snappy" {
		config.Producer.Compression = kafka.CompressionSnappy
	} else {
		config.Producer.Compression = kafka.CompressionNone
	}
	producer, err := kafka.NewSyncProducer([]string{route.Address}, config)
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
	return &RawAdapter{
		route:    route,
		producer: producer,
		tmpl:     tmpl,
	}, nil
}

type RawAdapter struct {
	producer kafka.SyncProducer
	route    *router.Route
	tmpl     *template.Template
}

func (a *RawAdapter) Stream(logstream chan *router.Message) {
	for message := range logstream {
		buf := new(bytes.Buffer)
		err := a.tmpl.Execute(buf, message)
		if err != nil {
			log.Error("raw:", err)
			return
		}
		if cn := utils.M1[message.Container.Name]; cn != "" {
			logmsg := utils.SendMessage(cn, buf.String(), message.Container)
			msg := &kafka.ProducerMessage{Topic: topic, Value: kafka.StringEncoder(logmsg)}
			partition, offset, err := a.producer.SendMessage(msg)
			_, _, _ = partition, offset, err
		}
	}

}
