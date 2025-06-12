package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
	NumHypervisors int    `json:"num_hypervisors"`
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
	HostIP         string `json:"host_ip"`
	Region         string `json:"region"`
	NumHypervisors int    `json:"num_hypervisors"`
	RegionID       int    `json:"regionId"`
	Token          string `json:"token"`
	CloudProvider  string `json:"cloudProvider"`
	Operation      string `json:"operation"`
}

func provisionHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	origin := r.Header.Get("Origin")
	if origin == "http://localhost:3000" || origin == "http://10.36.24.61:80" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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

		// Validate instance ID
		if len(provisionResp.Instances) == 0 {
			log.Printf("Error: No instances in provisioning response")
			return
		}

		instanceID := provisionResp.Instances[0].ID
		if instanceID == "" {
			log.Printf("Error: No instance_id received from provisioning service")
			return
		}

		log.Printf("Retrieved instance ID: %s", instanceID)

		// Start background monitoring
		log.Printf("Starting background monitoring for instance: %s", instanceID)
		monitorProvisioning(instanceID, req)
	}()
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
			// Publish to Kafka
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

	msg := KafkaMessage{
		HostIP:         privateIP,
		Region:         originalReq.Region,
		NumHypervisors: originalReq.NumHypervisors,
		RegionID:       originalReq.RegionID,
		Token:          originalReq.Token,
		CloudProvider:  originalReq.CloudProvider,
		Operation:      originalReq.Operation,
	}

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

func main() {
	log.Printf("Starting OCI Stage 2 Provision Service")
	http.HandleFunc("/provision-baremetal-stage2", provisionHandler)

	log.Printf("Server listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
