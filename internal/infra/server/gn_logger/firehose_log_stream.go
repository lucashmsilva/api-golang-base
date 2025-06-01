package gn_logger

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/firehose"
	"github.com/aws/aws-sdk-go-v2/service/firehose/types"
)

const (
	// Firehose limits
	MAX_LOG_BYTE_LENGTH     = 1000 * 1024     // 1000 KB
	MAX_RECORDS_BYTE_LENGTH = 4 * 1024 * 1024 // 4 MB

	// Customizable via options
	MAX_RECORD_BATCH_SIZE    = 500
	DEFAULT_WATCHER_MS_DELAY = 1000
)

type FirehoseLogStreamOptions struct {
	StreamName   string
	MaxBatchSize *int
	WatcherDelay *int
}

type FirehoseLogStream struct {
	options        FirehoseLogStreamOptions
	recordsBuff    []types.Record
	firehoseClient FirehoseClient
	ticker         *time.Ticker
	recordStream   chan types.Record
	mu             sync.Mutex
}

// Interface to allow mocking of the AWS Firehose API
type FirehoseClient interface {
	PutRecordBatch(ctx context.Context, input *firehose.PutRecordBatchInput, optFns ...func(*firehose.Options)) (*firehose.PutRecordBatchOutput, error)
}

func NewFirehoseLogStream(opts FirehoseLogStreamOptions) (*FirehoseLogStream, error) {
	var watcherDelay int
	var cfg aws.Config

	if opts.WatcherDelay == nil {
		watcherDelay = DEFAULT_WATCHER_MS_DELAY
	} else {
		watcherDelay = *opts.WatcherDelay
	}

	if opts.MaxBatchSize == nil {
		defaultMaxBatchSize := MAX_RECORD_BATCH_SIZE
		opts.MaxBatchSize = &defaultMaxBatchSize
	}

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, err
	}

	firehoseStream := &FirehoseLogStream{
		options:        opts,
		recordsBuff:    []types.Record{},
		firehoseClient: firehose.NewFromConfig(cfg),
		ticker:         time.NewTicker(time.Millisecond * time.Duration(watcherDelay)),
		recordStream:   make(chan types.Record, 1),
	}

	go firehoseStream.listenRecordStream()
	go firehoseStream.startWatcher()

	return firehoseStream, nil
}

func (f *FirehoseLogStream) Write(logBytes []byte) (n int, err error) {
	if len(logBytes) > MAX_LOG_BYTE_LENGTH {
		fmt.Printf("log length exceeds %v B.\n", MAX_LOG_BYTE_LENGTH)
		return len(logBytes), nil
	}

	f.recordStream <- types.Record{Data: slices.Clone(logBytes)}

	return len(logBytes), nil
}

func (f *FirehoseLogStream) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ticker.Stop()
	close(f.recordStream)

	for len(f.recordsBuff) > 0 {
		f.send()
	}

	return nil
}

// Process writes as with a buffered channel of log records that will be sent.
// This way (most of the time) avoids needing to sync at the io.Write level,
// due to the record buffer being appended at every write.
func (f *FirehoseLogStream) listenRecordStream() {
	for r := range f.recordStream {
		f.mu.Lock()

		f.recordsBuff = append(f.recordsBuff, r)
		if len(f.recordsBuff) >= *f.options.MaxBatchSize {
			go f.send()
		}

		f.mu.Unlock()
	}
}

func (f *FirehoseLogStream) startWatcher() {
	for range f.ticker.C {
		go f.send()
	}
}

func (f *FirehoseLogStream) send() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.recordsBuff) == 0 {
		return 0
	}

	var records []types.Record
	var recordsByteLength int

	if len(f.recordsBuff) >= *f.options.MaxBatchSize {
		records = f.recordsBuff[:*f.options.MaxBatchSize]
	} else {
		records = f.recordsBuff[:]
	}

	f.recordsBuff = f.recordsBuff[len(records):]

	for _, v := range records {
		recordsByteLength += len(v.Data)
	}

	// If the list of records to be sent has a byte length bigger than the limit,
	// pop the last record and add it to the front of the buffer. Repeat until the list fits.
	for recordsByteLength > MAX_RECORDS_BYTE_LENGTH {
		if len(records) == 0 {
			return 0
		}

		lastRecord := records[len(records)-1]
		records = records[:len(records)-1]
		records = slices.Insert(records, 0, lastRecord)

		recordsByteLength -= len(lastRecord.Data)
	}

	input := &firehose.PutRecordBatchInput{
		DeliveryStreamName: &f.options.StreamName,
		Records:            records,
	}

	response, err := f.firehoseClient.PutRecordBatch(context.TODO(), input) // putRecordBatchMock(context.TODO(), input)
	if err != nil {
		// In case of errors from AWS, add the entire record list back to the buffer
		f.recordsBuff = append(f.recordsBuff, records...)
		fmt.Printf("Error sending logs to firehose: %v]\n", err)
		return 0
	}

	if *response.FailedPutCount == int32(0) {
		return len(records)
	}

	// If any record failed to be sent, add them back to the buffer
	failedRecords := make([]types.Record, 0, *response.FailedPutCount)
	for i, r := range response.RequestResponses {
		if r.ErrorCode != nil {
			failedRecords = append(failedRecords, records[i])
		}
	}

	f.recordsBuff = append(f.recordsBuff, failedRecords...)

	return len(records) - int(*response.FailedPutCount)
}
