package gn_logger

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
)

// --- Manual Mock ---

type mockFirehoseClient struct {
	putCalls     int
	lastInput    *firehose.PutRecordBatchInput
	failResponse bool
	failEntries  bool
}

func (m *mockFirehoseClient) PutRecordBatch(ctx context.Context, input *firehose.PutRecordBatchInput, _ ...func(*firehose.Options)) (*firehose.PutRecordBatchOutput, error) {
	m.putCalls++
	m.lastInput = input

	if m.failResponse {
		return nil, errors.New("simulated PutRecordBatch failure")
	}

	resp := &firehose.PutRecordBatchOutput{
		FailedPutCount:   awsInt32(0),
		RequestResponses: make([]types.PutRecordBatchResponseEntry, len(input.Records)),
	}

	if m.failEntries {
		resp.FailedPutCount = awsInt32(int32(len(input.Records)))
		for i := range input.Records {
			resp.RequestResponses[i] = types.PutRecordBatchResponseEntry{ErrorCode: awsStr("InternalError")}
		}
	}

	return resp, nil
}

// --- Helpers ---

func awsStr(s string) *string { return &s }
func awsInt(i int) *int       { return &i }
func awsInt32(i int32) *int32 { return &i }

// --- Tests ---

func TestWrite_ValidSize(t *testing.T) {
	stream, _ := NewFirehoseLogStream(FirehoseLogStreamOptions{
		StreamName:   "test",
		MaxBatchSize: awsInt(10),
		WatcherDelay: awsInt(999999), // effectively disables ticker
	})
	defer stream.Close()

	data := []byte("valid log")
	n, err := stream.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
}

func TestWrite_TooLarge(t *testing.T) {
	stream, _ := NewFirehoseLogStream(FirehoseLogStreamOptions{
		StreamName:   "test",
		MaxBatchSize: awsInt(10),
	})
	defer stream.Close()

	tooBig := make([]byte, MAX_LOG_BYTE_LENGTH+1)
	n, err := stream.Write(tooBig)

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if n != len(tooBig) {
		t.Errorf("expected written byte count %d, got %d", len(tooBig), n)
	}
}

func TestListenRecordStream_TriggersSend(t *testing.T) {
	mockClient := &mockFirehoseClient{}
	stream, _ := NewFirehoseLogStream(FirehoseLogStreamOptions{
		StreamName:   "test",
		MaxBatchSize: awsInt(2),
		WatcherDelay: awsInt(999999),
	})
	stream.firehoseClient = mockClient

	stream.recordStream <- types.Record{Data: []byte("log1")}
	stream.recordStream <- types.Record{Data: []byte("log2")}

	// Give goroutine a moment to process
	time.Sleep(50 * time.Millisecond)

	if mockClient.putCalls == 0 {
		t.Error("expected send triggered by reaching MaxBatchSize")
	}
}

func TestSend_RespectsByteLimit(t *testing.T) {
	mockClient := &mockFirehoseClient{}
	stream, _ := NewFirehoseLogStream(FirehoseLogStreamOptions{
		StreamName:   "test",
		MaxBatchSize: awsInt(10),
		WatcherDelay: awsInt(999999),
	})
	stream.firehoseClient = mockClient

	big := bytes.Repeat([]byte("a"), MAX_RECORDS_BYTE_LENGTH/2+100)
	stream.recordsBuff = []types.Record{{Data: big}, {Data: big}}

	sent := stream.send()

	if sent != 1 {
		t.Errorf("expected 1 record to be sent due to byte size trimming, got %d", sent)
	}
	if len(stream.recordsBuff) != 1 {
		t.Errorf("expected 1 record left in buffer, got %d", len(stream.recordsBuff))
	}
}

func TestSend_Error_RequeuesAll(t *testing.T) {
	mockClient := &mockFirehoseClient{failResponse: true}
	stream, _ := NewFirehoseLogStream(FirehoseLogStreamOptions{
		StreamName:   "fail-test",
		MaxBatchSize: awsInt(2),
		WatcherDelay: awsInt(999999),
	})
	stream.firehoseClient = mockClient

	stream.recordsBuff = []types.Record{{Data: []byte("a")}, {Data: []byte("b")}}

	sent := stream.send()

	if sent != 0 {
		t.Errorf("expected 0 sent due to error, got %d", sent)
	}
	if len(stream.recordsBuff) != 2 {
		t.Errorf("expected 2 requeued, got %d", len(stream.recordsBuff))
	}
}

func TestSend_FailedEntries_RequeuesOnlyFailed(t *testing.T) {
	mockClient := &mockFirehoseClient{failEntries: true}
	stream, _ := NewFirehoseLogStream(FirehoseLogStreamOptions{
		StreamName:   "fail-entry-test",
		MaxBatchSize: awsInt(2),
		WatcherDelay: awsInt(999999),
	})
	stream.firehoseClient = mockClient

	stream.recordsBuff = []types.Record{{Data: []byte("a")}, {Data: []byte("b")}}

	sent := stream.send()

	if sent != 0 {
		t.Errorf("expected 0 sent due to failed entries, got %d", sent)
	}
	if len(stream.recordsBuff) != 2 {
		t.Errorf("expected 2 failed records requeued, got %d", len(stream.recordsBuff))
	}
}

func TestClose_SendsRemainingRecords(t *testing.T) {
	mockClient := &mockFirehoseClient{}
	stream, _ := NewFirehoseLogStream(FirehoseLogStreamOptions{
		StreamName:   "close-test",
		MaxBatchSize: awsInt(5),
		WatcherDelay: awsInt(999999),
	})
	stream.firehoseClient = mockClient

	stream.recordsBuff = []types.Record{{Data: []byte("one")}, {Data: []byte("two")}}

	err := stream.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if mockClient.putCalls == 0 {
		t.Error("expected final send during Close")
	}
	if len(stream.recordsBuff) != 0 {
		t.Errorf("expected buffer to be empty after Close, got %d", len(stream.recordsBuff))
	}
}

func TestTicker_TriggersSend(t *testing.T) {
	mockClient := &mockFirehoseClient{}
	stream, _ := NewFirehoseLogStream(FirehoseLogStreamOptions{
		StreamName:   "ticker-test",
		MaxBatchSize: awsInt(10), // high threshold so ticker, not batch size, triggers send
		WatcherDelay: awsInt(10), // fire every 10ms
	})
	stream.firehoseClient = mockClient

	// Write a single record (won't hit MaxBatchSize)
	_, err := stream.Write([]byte("log from ticker"))
	if err != nil {
		t.Fatalf("unexpected error on Write: %v", err)
	}

	// Give ticker goroutine some time to trigger
	time.Sleep(50 * time.Millisecond)

	// Cleanup
	stream.Close()

	if mockClient.putCalls == 0 {
		t.Error("expected ticker to trigger send, but no call was made")
	}
}
