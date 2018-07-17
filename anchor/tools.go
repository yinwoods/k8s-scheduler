package main

import (
    "log"
    "encoding/json"
    "net/http"
    "bytes"
    "errors"
    "net/url"
    "io/ioutil"
)

var (
    apiHost           = "127.0.0.1:8080"
    bindingsEndpoint  = "/api/v1/namespaces/default/pods/%s/binding/"
    eventsEndpoint    = "/api/v1/namespaces/default/events"
    nodesEndpoint     = "/api/v1/nodes"
    podsEndpoint      = "/api/v1/pods"
    watchPodsEndpoint = "/api/v1/watch/pods"
)

func postEvent(event Event) error {
    var b []byte
    body := bytes.NewBuffer(b)
    err := json.NewEncoder(body).Encode(event)
    if err != nil {
        return err
    }

    request := &http.Request{
        Body:          ioutil.NopCloser(body),
        ContentLength: int64(body.Len()),
        Header:        make(http.Header),
        Method:        http.MethodPost,
        URL: &url.URL{
            Host:   apiHost,
            Path:   eventsEndpoint,
            Scheme: "http",
        },
    }
    request.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(request)
    if err != nil {
        return err
    }
    if resp.StatusCode != 201 {
        return errors.New("Event: Unexpected HTTP status code" + resp.Status)
    }
    return nil
}

func getNodes() (*NodeList, error) {
    var nodeList NodeList

    request := &http.Request{
        Header: make(http.Header),
        Method: http.MethodGet,
        URL: &url.URL{
            Host:   apiHost,
            Path:   nodesEndpoint,
            Scheme: "http",
        },
    }
    request.Header.Set("Accept", "application/json, */*")

    resp, err := http.DefaultClient.Do(request)
    if err != nil {
        return nil, err
    }

    err = json.NewDecoder(resp.Body).Decode(&nodeList)
    if err != nil {
        return nil, err
    }

    return &nodeList, nil
}

func getPods() (*PodList, error) {
    var podList PodList

    v := url.Values{}
    v.Add("fieldSelector", "status.phase=Running")
    v.Add("fieldSelector", "status.phase=Pending")

    request := &http.Request{
        Header: make(http.Header),
        Method: http.MethodGet,
        URL: &url.URL{
            Host:     apiHost,
            Path:     podsEndpoint,
            RawQuery: v.Encode(),
            Scheme:   "http",
        },
    }
    request.Header.Set("Accept", "application/json, */*")

    resp, err := http.DefaultClient.Do(request)
    if err != nil {
        return nil, err
    }
    err = json.NewDecoder(resp.Body).Decode(&podList)
    if err != nil {
        return nil, err
    }
    return &podList, nil
}

func errFatal(err error, msg string) {
    if err != nil {
        log.Println(msg)
        log.Fatal(err)
    }
}

func errPrintln(err error, msg string) {
    if err != nil {
        log.Println(msg, err)
    }
}

func printResourceUsage(ru ResourceUsage, node *Node, msg string) {
    log.Print("node - " + node.Metadata.Name + "\t" + msg + ":\t")
    log.Printf("CPU: [%d] Memory: [%d] Pod: [%d]\n", ru.CPU, ru.Memory, ru.Pod)
}

func printNodeScores(nodeScore map[*Node]float64) {
    for node, score := range nodeScore {
        log.Printf("node [%s] got score: [%f]\n", node.Metadata.Name, score)
    }
}
