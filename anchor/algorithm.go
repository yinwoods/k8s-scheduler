
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

func parseCpu(resource ResourceList) int {
    if cpu, errs := resource["cpu"]; errs {
        if strings.HasSuffix(cpu, "m") {
            milliCores := strings.TrimSuffix(cpu, "m")
            cores, err := strconv.Atoi(milliCores)
            errFatal(err, "Failed to parse CPU")
            return cores
        }
        if cores, err := strconv.ParseFloat(cpu, 32); err == nil {
            errFatal(err, "Failed to parse CPU")
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
            errFatal(err, "Failed to parse Memory")
            return m
        }
    }
    if memory, errs := resource["memory"]; errs {
        if strings.HasSuffix(memory, "Mi") {
            mem := strings.TrimSuffix(memory, "Mi")
            m, err := strconv.Atoi(mem)
            errFatal(err, "Failed to parse Memory")
            return m * 1024
        }
    }
    return 0
}

func parsePod(resource ResourceList) int {
    if pods, errs := resource["pods"]; errs {
        p, err := strconv.Atoi(pods)
        errFatal(err, "Failed to parse Pods")
        return p
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
        ru := resourceUsage[p.Spec.NodeName]
        for _, c := range p.Spec.Containers {
            cpu := parseCpu(c.Resources.Requests)
            memorys := parseMemory(c.Resources.Requests)

            ru.CPU += cpu
            ru.Memory += memorys
        }
        ru.Pod += 1
    }

    var nodes []Node
    predicateFailures := make([]string, 0)

    var resourceRequired ResourceUsage
    var resourceFree ResourceUsage
    var resourceAllocatable ResourceUsage

    // 统计待调度pod所需资源总量
    for _, c := range pod.Spec.Containers {
        cpus := parseCpu(c.Resources.Requests)
        memorys := parseMemory(c.Resources.Requests)

        resourceRequired.CPU += cpus
        resourceRequired.Memory += memorys
        resourceRequired.Pod += 1
    }

    for _, node := range nodeList.Items {
        // resourceAllocatable 统计各个节点可分配资源总量
        resourceAllocatable.CPU = parseCpu(node.Status.Allocatable)
        resourceAllocatable.Memory = parseMemory(node.Status.Allocatable)
        resourceAllocatable.Pod = parsePod(node.Status.Allocatable)

        // 统计各个节点可用空闲资源总量
        resourceFree.CPU = (resourceAllocatable.CPU - resourceUsage[node.Metadata.Name].CPU)
        resourceFree.Memory = (resourceAllocatable.Memory - resourceUsage[node.Metadata.Name].Memory)
        resourceFree.Pod = (resourceAllocatable.Pod - resourceUsage[node.Metadata.Name].Pod)

        printResourceUsage(resourceAllocatable, node, "Resource Allocatable")
        printResourceUsage(*resourceUsage[node.Metadata.Name], node, "Resource Used")
        printResourceUsage(resourceFree, node, "Resource Free")

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
func priorities(nodes []Node) (Node, error) {

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
