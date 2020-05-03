package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var index = "bgplogger-*"
var pageSize = 50

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

	es, err := newElasticsearchClient(config.Elasticsearch)
	if err != nil {
		log.Fatal(err)
	}

	if err := newRouter(config.Listen, es); err != nil {
		log.Fatal(err)
	}
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
	return es, nil
}

func newRouter(listen string, es *elasticsearch.Client) error {
	r := gin.Default()
	r.GET("/searchByIP", func(c *gin.Context) {
		ip := c.Request.URL.Query().Get("ip")
		page, err := strconv.Atoi(c.Request.URL.Query().Get("page"))
		if err != nil {
			page = 0
		}
		reqBody, err := json.Marshal(map[string]interface{}{
			"from": page * pageSize,
			"size": pageSize,
			"sort": []interface{}{
				map[string]interface{}{
					"@timestamp": map[string]interface{}{
						"order": "desc",
					},
				},
			},
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"filter": []interface{}{
						map[string]interface{}{
							"range": map[string]interface{}{
								"range_start": map[string]interface{}{
									"lte": ip,
								},
							},
						},
						map[string]interface{}{
							"range": map[string]interface{}{
								"range_end": map[string]interface{}{
									"gte": ip,
								},
							},
						},
					},
				},
			},
		})
		if err != nil {
			c.JSON(500, gin.H{})
		}
		req := esapi.SearchRequest{
			Index: []string{index},
			Body:  strings.NewReader(string(reqBody)),
		}
		res, err := req.Do(context.Background(), es)
		if err != nil {
			log.Error(err)
			c.JSON(500, gin.H{})
		}
		if !(res.StatusCode >= 200 && res.StatusCode < 300) {
			log.Error(res.String())
			c.JSON(500, gin.H{})
		}
		body, err := ioutil.ReadAll(res.Body)
		searchResult := SearchResult{}
		if err := json.Unmarshal(body, &searchResult); err != nil {
			log.Error(err)
			c.JSON(500, gin.H{})
		}

		resp := []EsDocument{}
		for _, hit := range searchResult.Hits.Hits {
			resp = append(resp, hit.Source)
		}
		c.JSON(200, gin.H{
			"history": resp,
		})
	})

	r.GET("/all", func(c *gin.Context) {
		page, err := strconv.Atoi(c.Request.URL.Query().Get("page"))
		if err != nil {
			page = 0
		}
		reqBody, err := json.Marshal(map[string]interface{}{
			"from": page * pageSize,
			"size": pageSize,
			"sort": []interface{}{
				map[string]interface{}{
					"@timestamp": map[string]interface{}{
						"order": "desc",
					},
				},
			},
		})
		if err != nil {
			c.JSON(500, gin.H{})
		}
		req := esapi.SearchRequest{
			Index: []string{index},
			Body:  strings.NewReader(string(reqBody)),
		}
		res, err := req.Do(context.Background(), es)
		if err != nil {
			log.Error(err)
			c.JSON(500, gin.H{})
			return
		}
		if !(res.StatusCode >= 200 && res.StatusCode < 300) {
			log.Error(res.String())
			c.JSON(500, gin.H{})
			return
		}
		body, err := ioutil.ReadAll(res.Body)
		searchResult := SearchResult{}
		if err := json.Unmarshal(body, &searchResult); err != nil {
			log.Error(err)
			c.JSON(500, gin.H{})
			return
		}

		resp := []EsDocument{}
		for _, hit := range searchResult.Hits.Hits {
			resp = append(resp, hit.Source)
		}
		c.JSON(200, gin.H{
			"history": resp,
		})
	})
	return r.Run(listen)
}

type SearchResult struct {
	Hits struct {
		Hits []struct {
			Source EsDocument `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type EsDocument struct {
	Timestamp  time.Time `json:"@timestamp"`
	Prefix     string    `json:"prefix"`
	RangeStart string    `json:"range_start"`
	RangeEnd   string    `json:"range_end"`
	ASPath     []uint32  `json:"as_path"`
	Type       string    `json:"type"`
}

type Config struct {
	Listen        string              `yaml:"listen"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
}

type ElasticsearchConfig struct {
	Hosts    []string `yaml:"hosts"`
	Protocol string   `yaml:"protocol"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}
