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
	msg := KafkaMessage{
		HostIP:         "0.0.0.0",
		Region:         "s2r1",
		NumHypervisors: "1",
		RegionID:       98,
		Token:          "zyx",
		CloudProvider:  "oracle",
		Operation:      "setup_hypervisor",
	}

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
	if receivedKafkaMsg.HostIP != msg.HostIP {
		t.Errorf("HostIP mismatch: got %v, want %v", receivedKafkaMsg.HostIP, msg.HostIP)
	}
	if receivedKafkaMsg.Region != msg.Region {
		t.Errorf("Region mismatch: got %v, want %v", receivedKafkaMsg.Region, msg.Region)
	}
	if receivedKafkaMsg.NumHypervisors != msg.NumHypervisors {
		t.Errorf("NumHypervisors mismatch: got %v, want %v", receivedKafkaMsg.NumHypervisors, msg.NumHypervisors)
	}
	if receivedKafkaMsg.RegionID != msg.RegionID {
		t.Errorf("RegionID mismatch: got %v, want %v", receivedKafkaMsg.RegionID, msg.RegionID)
	}
	if receivedKafkaMsg.Token != msg.Token {
		t.Errorf("Token mismatch: got %v, want %v", receivedKafkaMsg.Token, msg.Token)
	}
	if receivedKafkaMsg.CloudProvider != msg.CloudProvider {
		t.Errorf("CloudProvider mismatch: got %v, want %v", receivedKafkaMsg.CloudProvider, msg.CloudProvider)
	}
	if receivedKafkaMsg.Operation != msg.Operation {
		t.Errorf("Operation mismatch: got %v, want %v", receivedKafkaMsg.Operation, msg.Operation)
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
