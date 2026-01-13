Here are the curl commands for every endpoint in the server.

1. Docker Management

List all containers (Returns ID, Name, Status, and State for both running and stopped containers)

```bash
curl http://localhost:8080/docker/list
```

Check Status of a specific container (Returns "running", "exited", etc.)

```bash
# Replace 'my-web' with your container ID or Name
curl "http://localhost:8080/docker/status?container_id=my-web"
```

Start a container

```bash
curl -X POST http://localhost:8080/docker/start \
     -H "Content-Type: application/json" \
     -d '{"container_id": "my-web"}'
```

Stop a container

```bash
curl -X POST http://localhost:8080/docker/stop \
     -H "Content-Type: application/json" \
     -d '{"container_id": "my-web"}'
```

---

2. Container Fault Injection (Cgroups)

These faults apply only to the specific container ID you provide.

Throttle CPU (CPU Choke) Limits the container to 10% CPU usage.

```bash
curl -X POST http://localhost:8080/docker/fault \
     -H "Content-Type: application/json" \
     -d '{
           "container_id": "my-web", 
           "fault_type": "cpu_choke"
         }'
```

Limit Memory (RAM) Hard limits the container to 64MB of RAM.

```bash
curl -X POST http://localhost:8080/docker/fault \
     -H "Content-Type: application/json" \
     -d '{
           "container_id": "my-web", 
           "fault_type": "mem_limit"
         }'
```

Restore Normalcy Removes all CPU and Memory limits.

```bash
curl -X POST http://localhost:8080/docker/fault \
     -H "Content-Type: application/json" \
     -d '{
           "container_id": "my-web", 
           "fault_type": "restore"
         }'
```
---

3. Host OS Fault Injection (System-wide)

These faults affect the entire server/VM running the Go code.

CPU Stress Spikes CPU load on all cores for 30 seconds.

```bash
curl -X POST http://localhost:8080/host/inject \
     -H "Content-Type: application/json" \
     -d '{
           "type": "cpu", 
           "duration": 30, 
           "load_percent": 100
         }'
```

Memory Stress Consumes 90% of available system RAM for 60 seconds.

```bash
curl -X POST http://localhost:8080/host/inject \
     -H "Content-Type: application/json" \
     -d '{
           "type": "memory", 
           "duration": 60
         }'
```

Disk I/O Stress Saturates disk write speeds for 20 seconds.

```bash
curl -X POST http://localhost:8080/host/inject \
     -H "Content-Type: application/json" \
     -d '{
           "type": "disk", 
           "duration": 20
         }'
```

Network Latency (Lag) Adds 200ms delay with 20ms jitter to the default network interface for 15 seconds.

```bash
curl -X POST http://localhost:8080/host/inject \
     -H "Content-Type: application/json" \
     -d '{
           "type": "network", 
           "duration": 15,
           "latency": "200ms",
           "jitter": "20ms"
         }'
```

Network Packet Loss Drops 10% of packets randomly for 15 seconds.

```bash
curl -X POST http://localhost:8080/host/inject \
     -H "Content-Type: application/json" \
     -d '{
           "type": "network", 
           "duration": 15,
           "loss": "10%"
         }'
```