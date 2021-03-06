package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type Api struct {
	url    string
	client http.Client
	debug  bool
}

func newAPI(url string, timeout *time.Duration, debug bool) *Api {
	return &Api{
		url: detectApiEndpoint(url),
		client: http.Client{
			Timeout: *timeout,
		},
		debug: debug,
	}
}

func detectApiEndpoint(url string) string {
	const probe = "/entity.json"

	url = strings.TrimRight(url, "/")
	log.Println("Validating API endpoint")

	resp, err := http.Get(url + probe)
	if err == nil {
		resp.Body.Close() // close body after checking for error

		if resp.StatusCode == 200 {
			log.Println("API endpoint validated")
			return url
		}
	}

	if strings.HasSuffix(url, "/middleware.php") {
		log.Println("API endpoint not responding. Will keep retrying using configured uri")
		return url
	}

	// append middleware.php
	detectedURL := url + "/middleware.php"
	log.Println("API endpoint not responding. Trying " + detectedURL)

	resp, err = http.Get(detectedURL + probe)
	if err == nil {
		resp.Body.Close() // close body after checking for error

		if resp.StatusCode == 200 {
			log.Println("API endpoint detected, using " + detectedURL)
			return detectedURL
		}
	}

	log.Println("API endpoint still not responding. Will keep retrying using configured uri")
	return url
}

func (api *Api) validate() {
	resp, err := http.Get(api.url)
	log.Println(err)
	log.Fatal(resp)
}

func (api *Api) get(endpoint string) (io.Reader, error) {
	url := api.url + endpoint

	start := time.Now()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Add("Accept", "application/json")

	resp, err := api.client.Do(req)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	defer resp.Body.Close() // close body after checking for error

	duration := time.Now().Sub(start)
	log.Printf("GET %s (%dms)", url, duration.Nanoseconds()/1e6)

	// read body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
	}

	if api.debug {
		log.Print(string(body))
	}

	return bytes.NewReader(body), nil
}

func (api *Api) getEntities() []Entity {
	r, err := api.get("/entity.json")
	if err != nil {
		return []Entity{}
	}

	er := EntityResponse{}
	if err := json.NewDecoder(r).Decode(&er); err != nil {
		log.Printf("json decode failed: %v", err)
		return []Entity{}
	}

	return er.Entities
}

func getGroup(d int64) string {
	if d > 3600*24*365 {
		return "year"
	} else if d > 3600*24*30 {
		return "month"
	} else if d > 3600*24*7 {
		return "week"
	} else if d > 3600*24 {
		return "day"
	} else if d > 3600 {
		return "hour"
	} else if d > 60 {
		return "minute"
	}
	return ""
}

func (api *Api) getData(uuid string, from time.Time, to time.Time, group string, options string, tuples int) []Tuple {
	f := from.Unix()
	t := to.Unix()
	url := fmt.Sprintf("/data/%s.json?from=%d&to=%d", uuid, f*1000, t*1000)

	if tuples > 0 {
		url += fmt.Sprintf("&tuples=%d", tuples)

		if group == "" {
			period := (t - f) / int64(tuples)
			group = getGroup(period)
		}
	}

	if group != "" {
		url += "&group=" + group
	}

	if options != "" {
		url += "&options=" + options
	}

	r, err := api.get(url)
	if err != nil {
		return []Tuple{}
	}

	dr := DataResponse{}
	if err := json.NewDecoder(r).Decode(&dr); err != nil {
		log.Printf("json decode failed: %v", err)
		return []Tuple{}
	}

	return dr.Data.Tuples
}

func (api *Api) getPrognosis(uuid string, period string) PrognosisStruct {
	url := fmt.Sprintf("/prognosis/%s.json?period=%s", uuid, period)

	r, err := api.get(url)
	if err != nil {
		return PrognosisStruct{}
	}

	pr := PrognosisResponse{}
	if err := json.NewDecoder(r).Decode(&pr); err != nil {
		log.Printf("json decode failed: %v", err)
		return PrognosisStruct{}
	}

	return pr.Prognosis
}
