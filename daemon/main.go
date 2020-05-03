package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/golang/protobuf/ptypes"
	api "github.com/osrg/gobgp/api"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

type pathattrLabel int

var (
	ORIGIN           pathattrLabel = 1
	AS_PATH          pathattrLabel = 2
	NEXT_HOP         pathattrLabel = 3
	MULTI_EXIT_DISC  pathattrLabel = 4
	LOCAL_PREF       pathattrLabel = 5
	ATOMIC_AGGREGATE pathattrLabel = 6
	AGGREGATOR       pathattrLabel = 7
	COMMUNITY        pathattrLabel = 8
	ORIGINATOR_ID    pathattrLabel = 9
	CLUSTER_LIST     pathattrLabel = 10
)

type Type string

var (
	typeAdd Type = "add"
	typeDel Type = "del"
)

var lifecycleName = "bgplogger"
var templateName = "bgplogger"
var indexPrefix = "bgplogger-"
var rolloverAlias = "bgplogger"
var bulkSize = 50
var bulkTimeout = time.Second

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.JSONFormatter{})

	var (
		configPath = flag.String("config", "./config.yml", "config file path")
	)
	flag.Parse()

	buf, err := ioutil.ReadFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	config := Config{}
	if err := yaml.Unmarshal([]byte(buf), &config); err != nil {
		log.Fatal(err)
	}

	client, err := newGobgpConn(config.Gobgp)
	if err != nil {
		log.Fatal(err)
	}

	es, err := newElasticsearchClient(config.Elasticsearch)
	if err != nil {
		log.Fatal(err)
	}

	if err := newGobgpMonitor(client, es); err != nil {
		log.Fatal(err)
	}
}

func newGobgpConn(target string) (api.GobgpApiClient, error) {
	log.WithField("target", target).Debug("Connecting to gobgp")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	grpcOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithInsecure(),
	}
	conn, err := grpc.DialContext(ctx, target, grpcOpts...)
	if err != nil {
		return nil, err
	}
	log.Debug("Successfully connected to gobgp")

	client := api.NewGobgpApiClient(conn)
	return client, nil
}

func newElasticsearchClient(config ElasticsearchConfig) (*elasticsearch.Client, error) {
	addresses := []string{}
	for _, host := range config.Hosts {
		if config.Username != "" && config.Password != "" {
			addresses = append(addresses, fmt.Sprintf("%s://%s:%s@%s", config.Protocol, config.Username, config.Password, host))
		} else {
			addresses = append(addresses, fmt.Sprintf("%s://%s", config.Protocol, host))
		}
	}
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: addresses})
	if err != nil {
		return nil, err
	}
	if err := setupElasticsearch(es); err != nil {
		return nil, err
	}
	return es, nil
}

func setupElasticsearch(client *elasticsearch.Client) error {
	ilmGetReq := esapi.ILMGetLifecycleRequest{
		Policy: lifecycleName,
	}
	ilmGetRes, err := ilmGetReq.Do(context.Background(), client)
	if err != nil {
		return err
	}
	if ilmGetRes.StatusCode == 404 {
		log.Debug("creating lifecyclePolicy")
		ilmBody, err := json.Marshal(newLifecyclePolicy())
		if err != nil {
			return err
		}
		ilmReq := esapi.ILMPutLifecycleRequest{
			Policy: lifecycleName,
			Body:   strings.NewReader(string(ilmBody)),
		}
		ilmRes, err := ilmReq.Do(context.Background(), client)
		if err != nil {
			return err
		}
		if !(ilmRes.StatusCode >= 200 && ilmRes.StatusCode < 300) {
			return errors.New(ilmRes.String())
		}
	}

	templateGetReq := esapi.IndicesGetTemplateRequest{
		Name: []string{templateName},
	}
	templateGetRes, err := templateGetReq.Do(context.Background(), client)
	if err != nil {
		return err
	}
	if templateGetRes.StatusCode == 404 {
		log.Debug("creating indexTemplate")
		templateBody, err := json.Marshal(newIndexTemplate(indexPrefix, lifecycleName, rolloverAlias))
		if err != nil {
			return err
		}
		templateReq := esapi.IndicesPutTemplateRequest{
			Name: templateName,
			Body: strings.NewReader(string(templateBody)),
		}
		templateRes, err := templateReq.Do(context.Background(), client)
		if err != nil {
			return err
		}
		if !(templateRes.StatusCode >= 200 && templateRes.StatusCode < 300) {
			return errors.New(templateRes.String())
		}
	}

	indexName := url.QueryEscape("<" + indexPrefix + "{now/d}-000001>")
	indexGetReq := esapi.IndicesExistsRequest{
		Index: []string{indexName},
	}
	indexGetRes, err := indexGetReq.Do(context.Background(), client)
	if err != nil {
		return err
	}
	if indexGetRes.StatusCode == 404 {
		log.Debug("creating rolloverAlias")
		indexBody, err := json.Marshal(newIndex(rolloverAlias))
		if err != nil {
			return err
		}
		indexReq := esapi.IndicesCreateRequest{
			Index: indexName,
			Body:  strings.NewReader(string(indexBody)),
		}
		indexRes, err := indexReq.Do(context.Background(), client)
		if err != nil {
			return err
		}
		if !(indexRes.StatusCode >= 200 && indexRes.StatusCode < 300) {
			return errors.New(indexRes.String())
		}
	}

	return nil
}

func newLifecyclePolicy() map[string]interface{} {
	return map[string]interface{}{
		"policy": map[string]interface{}{
			"phases": map[string]interface{}{
				"hot": map[string]interface{}{
					"actions": map[string]interface{}{
						"rollover": map[string]interface{}{
							"max_size": "50gb",
							"max_age":  "30d",
						},
					},
				},
				"delete": map[string]interface{}{
					"min_age": "90d",
					"actions": map[string]interface{}{
						"delete": map[string]interface{}{},
					},
				},
			},
		},
	}
}

func newIndexTemplate(prefix string, lifecycleName string, rolloverAlias string) map[string]interface{} {
	return map[string]interface{}{
		"index_patterns": []string{
			prefix + "*",
		},
		"settings": map[string]interface{}{
			"index.lifecycle.name":           lifecycleName,
			"index.lifecycle.rollover_alias": rolloverAlias,
		},
		"mappings": map[string]interface{}{
			"_source": map[string]interface{}{
				"enabled": true,
			},
			"properties": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"type": "date",
				},
				"as_path": map[string]interface{}{
					"type": "long",
				},
				"prefix": map[string]interface{}{
					"type": "text",
				},
				"range_start": map[string]interface{}{
					"type": "ip",
				},
				"range_end": map[string]interface{}{
					"type": "ip",
				},
				"type": map[string]interface{}{
					"type": "text",
				},
			},
		},
	}
}

func newIndex(rolloverAlias string) map[string]interface{} {
	return map[string]interface{}{
		"aliases": map[string]interface{}{
			rolloverAlias: map[string]interface{}{
				"is_write_index": true,
			},
		},
	}
}

func newGobgpMonitor(client api.GobgpApiClient, es *elasticsearch.Client) error {
	monitorTableClient, err := client.MonitorTable(context.Background(), &api.MonitorTableRequest{
		TableType: api.TableType_GLOBAL,
	})
	if err != nil {
		return err
	}
	for {
		res, err := monitorTableClient.Recv()
		if err != nil {
			return err
		}
		if res.Path == nil || res.Path.Nlri == nil || res.Path.Pattrs == nil {
			continue
		}
		prefix := api.IPAddressPrefix{}
		if err := ptypes.UnmarshalAny(res.Path.Nlri, &prefix); err != nil {
			continue
		}
		esDocument := EsDocument{
			Timestamp: time.Now(),
		}
		if res.Path.IsWithdraw {
			esDocument.Type = typeDel
		} else {
			esDocument.Type = typeAdd
		}
		switch res.Path.Family.Afi {
		case api.Family_AFI_IP:
			esDocument.Prefix = net.IPNet{
				IP:   net.ParseIP(prefix.Prefix),
				Mask: net.CIDRMask(int(prefix.PrefixLen), 32),
			}
		case api.Family_AFI_IP6:
			esDocument.Prefix = net.IPNet{
				IP:   net.ParseIP(prefix.Prefix),
				Mask: net.CIDRMask(int(prefix.PrefixLen), 128),
			}
		default:
			continue
		}
		for _, _pattr := range res.Path.Pattrs {
			pattr := api.AsPathAttribute{}
			if err := ptypes.UnmarshalAny(_pattr, &pattr); err != nil {
				continue
			}
			for _, segment := range pattr.Segments {
				switch segment.Type {
				case uint32(AS_PATH):
					esDocument.ASPath = segment.Numbers
				}
			}
		}

		fDocument := FormatEsDocument(esDocument)
		log.WithField("doc", fDocument).Debug("table update")
		if err := insertToElasticsearch(es, fDocument); err != nil {
			log.Error(err)
		}
	}
}

var queueLock = &sync.Mutex{}
var queue = []string{}
var timerCancel = make(chan struct{})

func insertToElasticsearch(client *elasticsearch.Client, document EsDocument) error {
	action, err := json.Marshal(newCreateAction(indexPrefix))
	if err != nil {
		return err
	}
	body, err := json.Marshal(document)
	if err != nil {
		return err
	}
	queueLock.Lock()
	queue = append(queue, string(action), string(body))
	if len(queue) > bulkSize*2 {
		timerCancel <- struct{}{}
		buf := make([]string, len(queue))
		copy(buf, queue)
		queue = []string{}
		queueLock.Unlock()
		if err := bulkInsertToElasticsearch(client, buf); err != nil {
			return err
		}
	} else if len(queue) == 2 {
		queueLock.Unlock()
		timer := time.NewTimer(bulkTimeout)
		select {
		case <-timerCancel:
			return nil
		case <-timer.C:
		}
		queueLock.Lock()
		buf := make([]string, len(queue))
		copy(buf, queue)
		queue = []string{}
		queueLock.Unlock()
		if err := bulkInsertToElasticsearch(client, buf); err != nil {
			return err
		}
	} else {
		queueLock.Unlock()
	}
	return nil
}

func bulkInsertToElasticsearch(client *elasticsearch.Client, buf []string) error {
	log.Debug("bulk write")
	req := esapi.BulkRequest{
		Index: rolloverAlias,
		Body:  strings.NewReader(strings.Join(buf, "\n") + "\n"),
	}
	res, err := req.Do(context.Background(), client)
	if err != nil {
		return err
	}
	if !(res.StatusCode >= 200 && res.StatusCode < 300) {
		return errors.New(res.String())
	}
	return nil
}

func newCreateAction(prefix string) map[string]interface{} {
	return map[string]interface{}{
		"index": map[string]interface{}{},
	}
}

type EsDocument struct {
	Timestamp  time.Time `json:"@timestamp"`
	Prefix     net.IPNet `json:"-"`
	PrefixStr  string    `json:"prefix"`
	RangeStart string    `json:"range_start"`
	RangeEnd   string    `json:"range_end"`
	ASPath     []uint32  `json:"as_path"`
	Type       Type      `json:"type"`
}

func FormatEsDocument(ed EsDocument) EsDocument {
	return EsDocument{
		Timestamp:  ed.Timestamp,
		PrefixStr:  ed.Prefix.String(),
		RangeStart: getRangeStart(ed.Prefix).String(),
		RangeEnd:   getRangeEnd(ed.Prefix).String(),
		ASPath:     ed.ASPath,
		Type:       ed.Type,
	}
}

func getRangeStart(ipnet net.IPNet) net.IP {
	return ipnet.IP.Mask(ipnet.Mask)
}

func getRangeEnd(ipnet net.IPNet) net.IP {
	ip := ipnet.IP.To16()
	p := make(net.IP, net.IPv6len)
	for i, ipbyte := range ip {
		if len(ipnet.Mask) == net.IPv4len {
			if i <= 12 {
				p[i] = ipbyte
			} else {
				p[i] = ipbyte | ^ipnet.Mask[i-12]
			}
		} else {
			p[i] = ipbyte | ^ipnet.Mask[i]
		}
	}
	return p
}

type Config struct {
	Gobgp         string              `yaml:"gobgp"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
}

type ElasticsearchConfig struct {
	Hosts    []string `yaml:"hosts"`
	Protocol string   `yaml:"protocol"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}
