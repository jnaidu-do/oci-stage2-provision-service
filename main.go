package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	provisionEndpoint = "http://129.80.55.22:5000/api/v1/provision-baremetal"
	trackEndpoint     = "http://129.80.55.22:5000/api/v1/track-baremetal"
	kafkaBroker       = "10.40.185.73:9092"
	kafkaTopic        = "filteredEvents"
)

type ProvisionRequest struct {
	Region         string `json:"region"`
	NumHypervisors string `json:"num_hypervisors"`
	RegionID       int    `json:"regionId"`
	Token          string `json:"token"`
	CloudProvider  string `json:"cloudProvider"`
	Operation      string `json:"operation"`
}

type Instance struct {
	ID        string `json:"id"`
	PrivateIP string `json:"private_ip"`
}

type ProvisionResponse struct {
	Message   string     `json:"message"`
	Instances []Instance `json:"instances"`
}

type TrackResponse struct {
	LifecycleState string `json:"lifecycle_state"`
	PrivateIP      string `json:"private_ip"`
}

type KafkaMessage struct {
	Payload struct {
		HostIP         string `json:"host_ip"`
		Region         string `json:"region"`
		NumHypervisors string `json:"num_hypervisors"`
		RegionID       int    `json:"regionId"`
		Token          string `json:"token"`
		CloudProvider  string `json:"cloudProvider"`
		Operation      string `json:"operation"`
	} `json:"payload"`
}

// Create a channel for log messages
var logChan = make(chan string, 100)

// Custom logger that writes to both stdout and our channel
type logWriter struct {
	*log.Logger
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	// Write to the channel
	select {
	case logChan <- string(p):
	default:
		// If channel is full, drop the message
	}
	// Write to stdout
	return w.Logger.Writer().Write(p)
}

func provisionHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	origin := r.Header.Get("Origin")
	log.Printf("Received request from origin: %s", origin)

	// Allow both localhost:3000 and the IP address (with and without port)
	allowedOrigins := []string{
		"http://localhost:3000",
		"http://localhost:3000/",
		"http://10.36.24.61",
		"http://10.36.24.61/",
		"http://10.36.24.61:80",
		"http://10.36.24.61:80/",
	}

	for _, allowedOrigin := range allowedOrigins {
		if origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			break
		}
	}

	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Printf("Received new provisioning request")
	if r.Method != http.MethodPost {
		log.Printf("Invalid method: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProvisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("Successfully decoded request: %+v", req)

	// Send immediate response
	log.Printf("Sending immediate 202 Accepted response")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "Baremetal provisioning has started",
	})

	// Start async provisioning
	go func() {
		log.Printf("Starting async provisioning process")
		// Forward request to downstream service
		reqBody, err := json.Marshal(req)
		if err != nil {
			log.Printf("Error marshaling request: %v", err)
			return
		}
		log.Printf("Sending request to downstream service: %s", provisionEndpoint)

		resp, err := http.Post(provisionEndpoint, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			log.Printf("Error calling provision endpoint: %v", err)
			return
		}
		defer resp.Body.Close()

		// Read the raw response body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response body: %v", err)
			return
		}

		// Log the raw response for debugging
		log.Printf("Downstream service response status: %d", resp.StatusCode)
		log.Printf("Downstream service response body: %s", string(bodyBytes))

		// Try to parse the response as JSON
		var provisionResp ProvisionResponse
		if err := json.Unmarshal(bodyBytes, &provisionResp); err != nil {
			log.Printf("Error decoding provision response: %v", err)
			log.Printf("Raw response: %s", string(bodyBytes))
			return
		}
		log.Printf("Successfully decoded provision response: %+v", provisionResp)

		// Validate instances
		if len(provisionResp.Instances) == 0 {
			log.Printf("Error: No instances in provisioning response")
			return
		}

		// Start monitoring for each instance
		for _, instance := range provisionResp.Instances {
			if instance.ID == "" {
				log.Printf("Error: Empty instance ID found in response")
				continue
			}
			log.Printf("Starting background monitoring for instance: %s", instance.ID)
			go monitorProvisioning(instance.ID, req)
		}
	}()
}

func isMachineReady(privateIP string) bool {
	log.Printf("Attempting to ping machine at %s...", privateIP)
	// Try to ping the machine
	cmd := exec.Command("ping", "-c", "1", "-W", "5", privateIP)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Ping to %s failed: %v", privateIP, err)
		log.Printf("Ping output: %s", string(output))
		return false
	}
	log.Printf("Ping output: %s", string(output))
	log.Printf("Ping successful, attempting SSH...")

	// Try to SSH to the machine using 3paccess.pem
	log.Printf("Attempting to SSH into machine at %s...", privateIP)
	sshCmd := exec.Command("ssh",
		"-i", "3paccess.pem",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=5",
		fmt.Sprintf("root@%s", privateIP),
		"echo 'SSH connection successful'")

	sshOutput, err := sshCmd.CombinedOutput()
	if err != nil {
		log.Printf("SSH to %s failed: %v", privateIP, err)
		log.Printf("SSH output: %s", string(sshOutput))
		return false
	}
	log.Printf("SSH output: %s", string(sshOutput))

	log.Printf("Machine %s is ready (ping and SSH successful)", privateIP)
	return true
}

func monitorProvisioning(instanceID string, originalReq ProvisionRequest) {
	if instanceID == "" {
		log.Printf("Error: Cannot monitor provisioning with empty instance ID")
		return
	}

	log.Printf("Starting monitoring for instance: %s", instanceID)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		trackURL := fmt.Sprintf("%s?instance_id=%s", trackEndpoint, instanceID)
		log.Printf("Polling instance status at: %s", trackURL)

		resp, err := http.Get(trackURL)
		if err != nil {
			log.Printf("Error checking instance status: %v", err)
			continue
		}

		// Read and log the raw response
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading track response body: %v", err)
			resp.Body.Close()
			continue
		}
		log.Printf("Track response status: %d", resp.StatusCode)
		log.Printf("Track response body: %s", string(bodyBytes))

		var trackResp TrackResponse
		if err := json.Unmarshal(bodyBytes, &trackResp); err != nil {
			log.Printf("Error decoding track response: %v", err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		log.Printf("Current instance state: %s", trackResp.LifecycleState)
		if trackResp.LifecycleState == "RUNNING" {
			log.Printf("Instance is now RUNNING, private IP: %s", trackResp.PrivateIP)

			// Wait for machine to be ready before publishing to Kafka
			log.Printf("Starting health check sequence for machine %s...", trackResp.PrivateIP)
			for i := 0; i < 25; i++ { // Try for 5 minutes (10 attempts * 30 seconds)
				log.Printf("Health check attempt %d/25 for machine %s", i+1, trackResp.PrivateIP)
				if isMachineReady(trackResp.PrivateIP) {
					// Publish to Kafka only when machine is ready
					publishToKafka(trackResp.PrivateIP, originalReq)
					return
				}
				log.Printf("Machine not ready yet, waiting 30 seconds before next attempt...")
				time.Sleep(30 * time.Second)
			}
			log.Printf("Machine did not become ready after 5 minutes, proceeding with Kafka message anyway")
			publishToKafka(trackResp.PrivateIP, originalReq)
			return
		}
		log.Printf("Instance not yet running, will check again in 30 seconds")
	}
}

func publishToKafka(privateIP string, originalReq ProvisionRequest) {
	log.Printf("Preparing to publish to Kafka topic: %s", kafkaTopic)

	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      []string{kafkaBroker},
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		Async:        true,
		RequiredAcks: 1,
	})

	defer writer.Close()

	msg := KafkaMessage{}
	msg.Payload.HostIP = privateIP
	msg.Payload.Region = originalReq.Region
	msg.Payload.NumHypervisors = originalReq.NumHypervisors
	msg.Payload.RegionID = originalReq.RegionID
	msg.Payload.Token = originalReq.Token
	msg.Payload.CloudProvider = originalReq.CloudProvider
	msg.Payload.Operation = originalReq.Operation

	log.Printf("Constructed Kafka message: %+v", msg)

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling Kafka message: %v", err)
		return
	}

	log.Printf("Publishing message to Kafka")
	err = writer.WriteMessages(context.Background(), kafka.Message{
		Value: msgBytes,
	})
	if err != nil {
		log.Printf("Error publishing to Kafka: %v", err)
		return
	}
	log.Printf("Successfully published message to Kafka")
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a channel for this client
	clientChan := make(chan string)

	// Add this client's channel to the list of clients
	go func() {
		for {
			select {
			case msg := <-logChan:
				clientChan <- msg
			case <-r.Context().Done():
				return
			}
		}
	}()

	// Stream logs to the client
	for {
		select {
		case msg := <-clientChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func main() {
	// Set up custom logger
	log.SetOutput(&logWriter{log.New(os.Stdout, "", log.LstdFlags)})

	log.Printf("Starting OCI Stage 2 Provision Service")
	http.HandleFunc("/provision-baremetal-stage2", provisionHandler)
	http.HandleFunc("/logs", logsHandler)

	log.Printf("Server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
