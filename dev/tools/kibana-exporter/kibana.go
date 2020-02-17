package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
)

const (
	savedObjectPath = "/api/saved_objects/"

	// Valid object type
	indexPatternType  = "index-pattern"
	dashboardType     = "dashboard"
	searchType        = "search"
	visualizationType = "visualization"
	configType        = "config"
	timelionSheetType = "timelion-sheet"

	// Queries
	// /api/saved_objects/_find?type=dashboard&search=id:AV4REOpp5NkDleZmzKkE-ecs
)

var (
	// TODO: Hardcoded at the moment, should be a param
	Host = "elastic:changeme@localhost:5601"
)

// Config ...
type Config struct{}

// Kibana ...
type Kibana struct{}

func New() (*Kibana, error) {
	// TOODO: Add support for passing space id to be used for loading assets
	return nil, nil
}

func (k *Kibana) get() error {
	_, err := http.Get("http://" + Host + savedObjectPath + dashboardType + "/foo")
	if err != nil {
		return err
	}

	//fmt.Println(resp.Body)
	return nil
}

func (k *Kibana) CreateObject(path, t, id string) error {

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	var ms common.MapStr
	err = json.Unmarshal(data, &ms)
	if err != nil {
		return err
	}

	EncodeData(ms)
	data, err = json.Marshal(ms)
	if err != nil {
		return err
	}

	d, err := makeRequest("POST", savedObjectPath+t+"/"+id, data)

	if err != nil {
		fmt.Println(string(d))
	}

	return err
}

func (k *Kibana) GetObject(t, id string) ([]byte, error) {
	return makeRequest("GET", savedObjectPath+t+"/"+id, nil)
}

func (k *Kibana) DeleteObject(t, id string) error {
	_, err := makeRequest("DELETE", savedObjectPath+t+"/"+id, nil)
	return err
}

func (k *Kibana) AddSpace(path string) error {

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	_, err = makeRequest("POST", "/api/spaces/space", data)
	return err
}

func (k *Kibana) RemoveSpace(id string) error {
	_, err := makeRequest("DELETE", "/api/spaces/space/"+id, nil)
	return err
}

func makeRequest(method, url string, body []byte) ([]byte, error) {

	fmt.Println("http://" + Host + url)
	r := bytes.NewReader(body)
	req, err := http.NewRequest(method, "http://"+Host+url, r)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("kbn-xsrf", "8.0.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Println(string(responseData))
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Error code: %s, error: %s", resp.StatusCode, string(responseData))
	}

	return responseData, nil
}

var (
	responseToDecode = []string{
		"attributes.uiStateJSON",
		"attributes.visState",
		"attributes.optionsJSON",
		"attributes.panelsJSON",
		"attributes.kibanaSavedObjectMeta.searchSourceJSON",
	}
)

// DecodeExported decodes an exported dashboard
func DecodeExported(result common.MapStr) common.MapStr {
	// remove unsupported chars
	objects := result["objects"].([]interface{})
	for _, obj := range objects {
		o := obj.(common.MapStr)
		for _, key := range responseToDecode {
			// All fields are optional, so errors are not caught
			err := decodeValue(o, key)
			if err != nil {
				logp.Debug("dashboards", "Error while decoding dashboard objects: %+v", err)
			}
		}
	}
	result["objects"] = objects
	return result
}

func DecodeInner(o common.MapStr) common.MapStr {

	for _, key := range responseToDecode {
		// All fields are optional, so errors are not caught
		err := decodeValue(o, key)
		if err != nil {
			logp.Debug("dashboards", "Error while decoding dashboard objects: %+v", err)
		}
	}

	return o
}

// DecodeExported decodes an exported dashboard
func EncodeData(object common.MapStr) common.MapStr {
	// remove unsupported chars

	for _, key := range responseToDecode {
		// All fields are optional, so errors are not caught
		err := encodeValue(object, key)
		if err != nil {
			logp.Debug("dashboards", "Error while decoding dashboard objects: %+v", err)
		}
	}

	return object
}

func decodeValue(data common.MapStr, key string) error {
	v, err := data.GetValue(key)
	if err != nil {
		return err
	}
	s := v.(string)
	var d interface{}
	err = json.Unmarshal([]byte(s), &d)
	if err != nil {
		return fmt.Errorf("error decoding %s: %v", key, err)
	}

	data.Put(key, d)
	return nil
}

func encodeValue(data common.MapStr, key string) error {
	v, err := data.GetValue(key)
	if err != nil {
		return err
	}

	d, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("error decoding %s: %v", key, err)
	}

	data.Put(key, string(d))
	return nil
}
