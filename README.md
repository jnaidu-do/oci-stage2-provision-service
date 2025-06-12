# OCI Stage 2 Provision Service

This is a Go microservice that orchestrates the bare metal provisioning process. It exposes an API endpoint to trigger provisioning, monitors the process in the background, and publishes notifications to Kafka when complete.

## Prerequisites

- Go 1.21 or later
- Access to the downstream provisioning service (http://129.80.55.22:5000)
- Access to Kafka broker (10.40.185.73:9092)
- Docker (optional, for containerized deployment)

## Installation

### Local Development

1. Clone the repository
2. Install dependencies:
```bash
go mod download
```

### Docker Deployment

1. Build the Docker image:
```bash
docker build -t oci-stage2-provision-service .
```

2. Run the container:
```bash
docker run -p 8080:8080 oci-stage2-provision-service
```

## Running the Service

### Local Development
Start the service:
```bash
go run main.go
```

### Docker
The service will be available on port 8080 when running in Docker.

## API Usage

### Trigger Provisioning

```bash
curl -X POST http://localhost:8080/provision-baremetal-stage2 \
  -H "Content-Type: application/json" \
  -d '{
    "region": "s2r1",
    "num_hypervisors": 1,
    "regionId": 98,
    "token": "zyx",
    "cloudProvider": "oracle",
    "operation": "setup_hypervisor"
  }'
```

The service will:
1. Forward the request to the downstream provisioning service
2. Immediately respond with a 202 Accepted status
3. Monitor the provisioning process in the background
4. Publish a notification to Kafka when provisioning is complete

## Response Format

### Immediate Response (202 Accepted)
```json
{
  "status": "Baremetal provisioning has started"
}
```

### Kafka Message (when provisioning completes)
```json
{
    "host_ip": "10.27.0.48",
    "region": "s2r1",
    "num_hypervisors": 1,
    "regionId": 98,
    "token": "zyx",
    "cloudProvider": "oracle",
    "operation": "setup_hypervisor"
}
``` 