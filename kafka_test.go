package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestKafkaPublishing(t *testing.T) {
	// Test message
	msg := KafkaMessage{}
	msg.Payload.HostIP = "0.0.0.0"
	msg.Payload.Region = "s2r1"
	msg.Payload.NumHypervisors = "1"
	msg.Payload.RegionID = 98
	msg.Payload.Token = "zyx"
	msg.Payload.CloudProvider = "oracle"
	msg.Payload.Operation = "setup_hypervisor"

	// Create Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      []string{kafkaBroker},
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		Async:        true,
		RequiredAcks: 1,
	})
	defer writer.Close()

	// Create Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaBroker},
		Topic:   kafkaTopic,
		GroupID: "test-group",
	})
	defer reader.Close()

	// Marshal message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Publish message
	err = writer.WriteMessages(ctx, kafka.Message{
		Value: msgBytes,
	})
	if err != nil {
		t.Fatalf("Failed to publish message: %v", err)
	}

	// Verify message was received
	receivedMsg, err := reader.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	// Unmarshal received message
	var receivedKafkaMsg KafkaMessage
	if err := json.Unmarshal(receivedMsg.Value, &receivedKafkaMsg); err != nil {
		t.Fatalf("Failed to unmarshal received message: %v", err)
	}

	// Verify message content
	if receivedKafkaMsg.Payload.HostIP != msg.Payload.HostIP {
		t.Errorf("HostIP mismatch: got %v, want %v", receivedKafkaMsg.Payload.HostIP, msg.Payload.HostIP)
	}
	if receivedKafkaMsg.Payload.Region != msg.Payload.Region {
		t.Errorf("Region mismatch: got %v, want %v", receivedKafkaMsg.Payload.Region, msg.Payload.Region)
	}
	if receivedKafkaMsg.Payload.NumHypervisors != msg.Payload.NumHypervisors {
		t.Errorf("NumHypervisors mismatch: got %v, want %v", receivedKafkaMsg.Payload.NumHypervisors, msg.Payload.NumHypervisors)
	}
	if receivedKafkaMsg.Payload.RegionID != msg.Payload.RegionID {
		t.Errorf("RegionID mismatch: got %v, want %v", receivedKafkaMsg.Payload.RegionID, msg.Payload.RegionID)
	}
	if receivedKafkaMsg.Payload.Token != msg.Payload.Token {
		t.Errorf("Token mismatch: got %v, want %v", receivedKafkaMsg.Payload.Token, msg.Payload.Token)
	}
	if receivedKafkaMsg.Payload.CloudProvider != msg.Payload.CloudProvider {
		t.Errorf("CloudProvider mismatch: got %v, want %v", receivedKafkaMsg.Payload.CloudProvider, msg.Payload.CloudProvider)
	}
	if receivedKafkaMsg.Payload.Operation != msg.Payload.Operation {
		t.Errorf("Operation mismatch: got %v, want %v", receivedKafkaMsg.Payload.Operation, msg.Payload.Operation)
	}

	t.Logf("Successfully verified message in Kafka: %+v", receivedKafkaMsg)
}

// TestKafkaConnection tests if we can connect to Kafka
func TestKafkaConnection(t *testing.T) {
	// Create Kafka writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      []string{kafkaBroker},
		Topic:        kafkaTopic,
		Balancer:     &kafka.LeastBytes{},
		Async:        true,
		RequiredAcks: 1,
	})
	defer writer.Close()

	// Create Kafka reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaBroker},
		Topic:   kafkaTopic,
		GroupID: "test-group",
	})
	defer reader.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to write a test message
	testMsg := []byte("test connection")
	err := writer.WriteMessages(ctx, kafka.Message{
		Value: testMsg,
	})
	if err != nil {
		t.Fatalf("Failed to connect to Kafka: %v", err)
	}

	// Verify message was received
	receivedMsg, err := reader.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("Failed to read test message: %v", err)
	}

	if string(receivedMsg.Value) != string(testMsg) {
		t.Errorf("Message content mismatch: got %v, want %v", string(receivedMsg.Value), string(testMsg))
	}

	t.Log("Successfully verified Kafka connection and message delivery")
}
