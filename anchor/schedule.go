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
    "fmt"
    "log"
    "sync"
    "time"
    "bytes"
    "io/ioutil"
    "encoding/json"
    "net/http"
    "net/url"
    "errors"
)

var processorLock = &sync.Mutex{}
const schedulerName = "hightower"

// 再次调度，调度多个未调度的pod
func reconcileUnscheduledPods(interval int, done chan struct{}, wg *sync.WaitGroup) {
    for {
        select {
        case <-time.After(time.Duration(interval) * time.Second):
            err := schedulePods()
            errPrintln(err, "pods schedule failed")
        case <-done:
            wg.Done()
            log.Println("Stopped reconciliation loop.")
            return
        }
    }
}

func monitorUnscheduledPods(done chan struct{}, wg *sync.WaitGroup) {
    pods, errc := watchUnscheduledPods()

    for {
        select {
        case err := <-errc:
            log.Println(err)
        case pod := <-pods:
            processorLock.Lock()
            time.Sleep(2 * time.Second)
            err := schedulePod(&pod)
            errPrintln(err, "pod schedule failed")
            processorLock.Unlock()
        case <-done:
            wg.Done()
            log.Println("Stopped scheduler.")
            return
        }
    }
}

func schedulePod(pod *Pod) error {
    nodes, err := predicate(pod)
    if err != nil {
        return err
    }
    // 无节点能够满足该pod运行所需资源
    if len(nodes) == 0 {
        return fmt.Errorf("Unable to schedule pod (%s) failed to fit in any node", pod.Metadata.Name)
    }

    // 选出price最小的节点
    node, err := priorities(pod, nodes)
    if err != nil {
        return err
    }
    // 调度节点
    err = bind(pod, node)
    if err != nil {
        return err
    }
    return nil
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
        errPrintln(err, "pod schedule failed")
    }
    return nil
}

// 调度pod到节点上
func bind(pod *Pod, node *Node) error {
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

