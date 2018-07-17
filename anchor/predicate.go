package main

import (
    "fmt"
    "strconv"
    "strings"
    "time"
)

func parseCpu(resource ResourceList) int64 {
    if cpu, errs := resource["cpu"]; errs {
        if strings.HasSuffix(cpu, "m") {
            milliCores := strings.TrimSuffix(cpu, "m")
            cores, err := strconv.ParseInt(milliCores, 10, 64)
            errFatal(err, "Failed to parse CPU")
            return cores
        }
        if c, err := strconv.ParseFloat(cpu, 32); err == nil {
            errFatal(err, "Failed to parse CPU")
            return int64(c * 1000)
        }
    }
    return 0
}

func parseMemory(resource ResourceList) int64 {
    if memory, errs := resource["memory"]; errs {
        if strings.HasSuffix(memory, "Ki") {
            mem := strings.TrimSuffix(memory, "Ki")
            m, err := strconv.ParseInt(mem, 10, 64)
            errFatal(err, "Failed to parse Memory")
            return m
        }
    }
    if memory, errs := resource["memory"]; errs {
        if strings.HasSuffix(memory, "Mi") {
            mem := strings.TrimSuffix(memory, "Mi")
            m, err := strconv.ParseInt(mem, 10, 64)
            errFatal(err, "Failed to parse Memory")
            return m * 1024
        }
    }
    return 0
}

func parsePod(resource ResourceList) int64 {
    if pods, errs := resource["pods"]; errs {
        p, err := strconv.ParseInt(pods, 10, 64)
        errFatal(err, "Failed to parse Pods")
        return p
    }
    return 0
}

// 统计节点上可分配资源总量
func allocatableResource(node *Node, used map[string]*ResourceUsage) ResourceUsage {
    var tr ResourceUsage
    tr.CPU = parseCpu(node.Status.Capacity)
    tr.Memory = parseMemory(node.Status.Capacity)
    tr.Pod = parsePod(node.Status.Capacity)

    var allocatable ResourceUsage
    allocatable.CPU = tr.CPU - used[node.Metadata.Name].CPU
    allocatable.Memory = tr.Memory - used[node.Metadata.Name].Memory
    allocatable.Pod = tr.Pod - used[node.Metadata.Name].Pod
    return allocatable
}

func requestedResource(pod *Pod) ResourceUsage {
    // 统计待调度pod所需资源总量
    var rr ResourceUsage
    for _, c := range pod.Spec.Containers {
        cpus := parseCpu(c.Resources.Requests)
        memorys := parseMemory(c.Resources.Requests)

        rr.CPU += cpus
        rr.Memory += memorys
        rr.Pod += 1
    }
    return rr
}

func usedResource(nodeList *NodeList, podList *PodList) map[string]*ResourceUsage {
    used := make(map[string]*ResourceUsage)
    for _, node := range nodeList.Items {
        used[node.Metadata.Name] = &ResourceUsage{}
    }

    // 统计各个各个节点上pod已用资源总量
    for _, p := range podList.Items {
        if p.Spec.NodeName == "" {
            continue
        }
        ru := used[p.Spec.NodeName]
        for _, c := range p.Spec.Containers {
            cpu := parseCpu(c.Resources.Requests)
            memorys := parseMemory(c.Resources.Requests)

            ru.CPU += cpu
            ru.Memory += memorys
        }
        ru.Pod += 1
    }
    return used
}

func predicate(pod *Pod) ([]*Node, error) {
    // 获取所有节点
    nodeList, err := getNodes()
    errFatal(err, "failed to get nodes")

    // 获取所有pod
    podList, err := getPods()
    errFatal(err, "failed to get pods")

    used := usedResource(nodeList, podList)

    var nodes []*Node
    failures := make([]string, 0)

    var requested ResourceUsage
    var allocatable ResourceUsage

    // 统计待调度pod所需资源总量
    requested = requestedResource(pod)

    for _, node := range nodeList.Items {
        // allocatable 统计各个节点可分配资源总量
        allocatable = allocatableResource(node, used)

        printResourceUsage(allocatable, node, "Resource Allocatable")
        printResourceUsage(*used[node.Metadata.Name], node, "Resource Used")

        if allocatable.CPU < requested.CPU {
            m := fmt.Sprintf("fit failure on node (%s): Insufficient CPU", node.Metadata.Name)
            failures = append(failures, m)
            continue
        }
        if allocatable.Memory < requested.Memory {
            m := fmt.Sprintf("fit failure on node (%s): Insufficient Memory", node.Metadata.Name)
            failures = append(failures, m)
            continue
        }
        if allocatable.Pod < requested.Pod {
            m := fmt.Sprintf("fit failure on node (%s): Insufficient Pod", node.Metadata.Name)
            failures = append(failures, m)
            continue
        }
        nodes = append(nodes, node)
    }

    if len(nodes) == 0 {
        // 触发异常，表明该pod无法调度
        timestamp := time.Now().UTC().Format(time.RFC3339)
        event := Event{
            Count:          1,
            Message:        fmt.Sprintf("pod (%s) failed to fit in any node\n%s", pod.Metadata.Name, strings.Join(failures, "\n")),
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
