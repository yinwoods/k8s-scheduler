// Copyright 2016 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "time"
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

func watchUnscheduledPods() (<-chan Pod, <-chan error) {
    pods := make(chan Pod)
    errc := make(chan error, 1)

    v := url.Values{}
    v.Set("fieldSelector", "spec.nodeName=")

    request := &http.Request{
        Header: make(http.Header),
        Method: http.MethodGet,
        URL: &url.URL{
            Host:     apiHost,
            Path:     watchPodsEndpoint,
            RawQuery: v.Encode(),
            Scheme:   "http",
        },
    }
    request.Header.Set("Accept", "application/json, */*")

    go func() {
        for {
            resp, err := http.DefaultClient.Do(request)
            // 出错重传
            if err != nil {
                errc <- err
                time.Sleep(5 * time.Second)
                continue
            }

            if resp.StatusCode != 200 {
                errc <- errors.New("Invalid status code: " + resp.Status)
                time.Sleep(5 * time.Second)
                continue
            }

            decoder := json.NewDecoder(resp.Body)
            for {
                var event PodWatchEvent
                err = decoder.Decode(&event)
                if err != nil {
                    errc <- err
                    break
                }

                if event.Type == "ADDED" {
                    pods <- event.Object
                }
            }
        }
    }()

    return pods, errc
}

func getUnscheduledPods() ([]*Pod, error) {
    // 获取调度器下未调度的pod
    var podList PodList
    unscheduledPods := make([]*Pod, 0)

    v := url.Values{}
    v.Set("fieldSelector", "spec.nodeName=")

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
        return unscheduledPods, err
    }
    err = json.NewDecoder(resp.Body).Decode(&podList)
    if err != nil {
        return unscheduledPods, err
    }

    for _, pod := range podList.Items {
        if pod.Metadata.Annotations["scheduler.alpha.kubernetes.io/name"] == schedulerName {
            unscheduledPods = append(unscheduledPods, &pod)
        }
    }

    return unscheduledPods, nil
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

type ResourceUsage struct {
    CPU int
    Memory int
    Disk int
    Pods int
}

func parseCpu(resource ResourceList) int {
    if cpu, errs := resource["cpu"]; errs {
        if strings.HasSuffix(cpu, "m") {
            milliCores := strings.TrimSuffix(cpu, "m")
            cores, err := strconv.Atoi(milliCores)
            checkError(err)
            return cores
        }
        if cores, err := strconv.ParseFloat(cpu, 32); err == nil {
            checkError(err)
            return int(cores * 1000)
        }
    }
    return 0
}

func parseMemory(resource ResourceList) int {
    if memory, errs := resource["memory"]; errs {
        if strings.HasSuffix(memory, "Ki") {
            mem := strings.TrimSuffix(memory, "Ki")
            m, err := strconv.Atoi(mem)
            checkError(err)
            return m
        }
    }
    if memory, errs := resource["memory"]; errs {
        if strings.HasSuffix(memory, "Mi") {
            mem := strings.TrimSuffix(memory, "Mi")
            m, err := strconv.Atoi(mem)
            checkError(err)
            return m * 1024
        }
    }
    return 0
}

func predicate(pod *Pod) ([]Node, error) {
    // 获取所有节点
    nodeList, err := getNodes()
    if err != nil {
        return nil, err
    }

    // 获取所有pod
    podList, err := getPods()
    if err != nil {
        return nil, err
    }

    resourceUsage := make(map[string]*ResourceUsage)
    for _, node := range nodeList.Items {
        resourceUsage[node.Metadata.Name] = &ResourceUsage{}
    }

    // 统计各个各个节点上pod已用资源总量
    for _, p := range podList.Items {
        if p.Spec.NodeName == "" {
            continue
        }
        for _, c := range p.Spec.Containers {
            cores := parseCpu(c.Resources.Requests)
            memorys := parseMemory(c.Resources.Requests)

            ru := resourceUsage[p.Spec.NodeName]
            ru.CPU += cores
            ru.Memory += memorys
        }
    }

    var nodes []Node
    predicateFailures := make([]string, 0)

    var resourceRequired ResourceUsage
    var resourceFree ResourceUsage
    var resourceAllocatable ResourceUsage

    // 统计待调度pod所需资源总量
    for _, c := range pod.Spec.Containers {
        cores := parseCpu(c.Resources.Requests)
        memorys := parseMemory(c.Resources.Requests)

        resourceRequired.CPU += cores
        resourceRequired.Memory += memorys
    }

    for _, node := range nodeList.Items {
        // resourceAllocatable 统计各个节点可分配资源总量
        resourceAllocatable.CPU = parseCpu(node.Status.Allocatable)
        resourceAllocatable.Memory = parseMemory(node.Status.Allocatable)

        // 统计各个节点可用空闲资源总量
        resourceFree.CPU = (resourceAllocatable.CPU - resourceUsage[node.Metadata.Name].CPU)
        resourceFree.Memory = (resourceAllocatable.Memory - resourceUsage[node.Metadata.Name].Memory)
        fmt.Println(resourceAllocatable)
        fmt.Println(resourceUsage[node.Metadata.Name])
        fmt.Println(resourceFree)

        if resourceFree.CPU < resourceRequired.CPU {
            m := fmt.Sprintf("fit failure on node (%s): Insufficient CPU", node.Metadata.Name)
            predicateFailures = append(predicateFailures, m)
            continue
        }
        nodes = append(nodes, node)
    }

    if len(nodes) == 0 {
        // 触发异常，表明该pod无法调度
        timestamp := time.Now().UTC().Format(time.RFC3339)
        event := Event{
            Count:          1,
            Message:        fmt.Sprintf("pod (%s) failed to fit in any node\n%s", pod.Metadata.Name, strings.Join(predicateFailures, "\n")),
            Metadata:       Metadata{GenerateName: pod.Metadata.Name + "-"},
            Reason:         "FailedScheduling",
            LastTimestamp:  timestamp,
            FirstTimestamp: timestamp,
            Type:           "Warning",
            Source:         EventSource{Component: "hightower-scheduler"},
            InvolvedObject: ObjectReference{
                Kind:      "Pod",
                Name:      pod.Metadata.Name,
                Namespace: "default",
                Uid:       pod.Metadata.Uid,
            },
        }

        postEvent(event)
    }

    return nodes, nil
}

func bind(pod *Pod, node Node) error {
    binding := Binding{
        ApiVersion: "v1",
        Kind:       "Binding",
        Metadata:   Metadata{Name: pod.Metadata.Name},
        Target: Target{
            ApiVersion: "v1",
            Kind:       "Node",
            Name:       node.Metadata.Name,
        },
    }

    var b []byte
    body := bytes.NewBuffer(b)
    err := json.NewEncoder(body).Encode(binding)
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
            Path:   fmt.Sprintf(bindingsEndpoint, pod.Metadata.Name),
            Scheme: "http",
        },
    }
    request.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(request)
    if err != nil {
        return err
    }
    if resp.StatusCode != 201 {
        return errors.New("Binding: Unexpected HTTP status code" + resp.Status)
    }

    // Emit a Kubernetes event that the Pod was scheduled successfully.
    message := fmt.Sprintf("Successfully assigned %s to %s", pod.Metadata.Name, node.Metadata.Name)
    timestamp := time.Now().UTC().Format(time.RFC3339)
    event := Event{
        Count:          1,
        Message:        message,
        Metadata:       Metadata{GenerateName: pod.Metadata.Name + "-"},
        Reason:         "Scheduled",
        LastTimestamp:  timestamp,
        FirstTimestamp: timestamp,
        Type:           "Normal",
        Source:         EventSource{Component: "hightower-scheduler"},
        InvolvedObject: ObjectReference{
            Kind:      "Pod",
            Name:      pod.Metadata.Name,
            Namespace: "default",
            Uid:       pod.Metadata.Uid,
        },
    }
    log.Println(message)
    return postEvent(event)
}

// TODO
/*
Add Algorithm
now it's choosed by ip
*/
func bestNode(nodes []Node) (Node, error) {
    type NodePrice struct {
        Node  Node
        Price float64
    }

    var bestNodePrice *NodePrice
    for _, n := range nodes {
        ips, ok := n.Metadata.Annotations["flannel.alpha.coreos.com/public-ip"]
        if !ok {
            continue
        }

        splitIps := strings.Split(ips, ".")
        price := splitIps[len(splitIps) - 1]

        f, err := strconv.ParseFloat(price, 32)
        if err != nil {
            return Node{}, err
        }
        if bestNodePrice == nil {
            bestNodePrice = &NodePrice{n, f}
            continue
        }
        if f < bestNodePrice.Price {
            bestNodePrice.Node = n
            bestNodePrice.Price = f
        }
    }

    if bestNodePrice == nil {
        bestNodePrice = &NodePrice{nodes[0], 0}
    }
    return bestNodePrice.Node, nil
}

// 调度多个pod
func schedulePods() error {
    processorLock.Lock()
    defer processorLock.Unlock()
    pods, err := getUnscheduledPods()
    if err != nil {
        return err
    }
    for _, pod := range pods {
        err := schedulePod(pod)
        if err != nil {
            log.Println(err)
        }
    }
    return nil
}
